package models

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// Message is one turn of a conversation.
type Message struct {
	Role    string  `json:"role"` // user or assistant
	Content string  `json:"content"`
	Images  []Image `json:"-"` // user turns only; sent before the text in each wire format
	// CacheBreak ends a cacheable prefix after this message: an Anthropic
	// Messages cache_control breakpoint. Other formats cache prefixes on
	// their own (OpenAI with Request.CacheKey), so they ignore it.
	CacheBreak bool `json:"-"`
}

// Image is an image attached to a user message.
type Image struct {
	MediaType string // image/png or image/jpeg
	Data      []byte
}

// wire renders messages for a wire format: unchanged while no message has
// images, else each message with images as a content array (images first,
// then the text) in the format's own block types.
func wire(style string, msgs []Message) any {
	breaks := style == StyleMessages && slices.ContainsFunc(msgs, func(m Message) bool { return m.CacheBreak })
	if !breaks && !slices.ContainsFunc(msgs, func(m Message) bool { return len(m.Images) > 0 }) {
		return msgs
	}
	out := make([]map[string]any, len(msgs))
	for i, m := range msgs {
		if style == StyleMessages && m.CacheBreak && len(m.Images) == 0 {
			out[i] = map[string]any{"role": m.Role, "content": []map[string]any{{"type": "text", "text": m.Content, "cache_control": map[string]string{"type": "ephemeral"}}}}
			continue
		}
		if len(m.Images) == 0 {
			out[i] = map[string]any{"role": m.Role, "content": m.Content}
			continue
		}
		var parts []map[string]any
		for _, img := range m.Images {
			b64 := base64.StdEncoding.EncodeToString(img.Data)
			dataURL := "data:" + img.MediaType + ";base64," + b64
			switch style {
			case StyleMessages:
				parts = append(parts, map[string]any{"type": "image", "source": map[string]any{"type": "base64", "media_type": img.MediaType, "data": b64}})
			case StyleResponses:
				parts = append(parts, map[string]any{"type": "input_image", "image_url": dataURL})
			default:
				parts = append(parts, map[string]any{"type": "image_url", "image_url": map[string]any{"url": dataURL}})
			}
		}
		text := map[string]any{"type": "text", "text": m.Content}
		if style == StyleResponses {
			text["type"] = "input_text"
		}
		out[i] = map[string]any{"role": m.Role, "content": append(parts, text)}
	}
	return out
}

// Request is a provider-neutral model call.
type Request struct {
	Model     string
	System    string
	Messages  []Message
	MaxTokens int
	Effort    string  // reasoning effort; empty for the provider default
	Format    *Format // ask for JSON matching a schema where the wire format supports it; nil for free text
	// CacheKey groups calls that share a prompt prefix for OpenAI Responses
	// (prompt_cache_key), so they reach the same cache; others ignore it.
	CacheKey string
	// Stream asks the endpoint to stream the reply (all three wire formats),
	// so a long generation keeps bytes flowing past a proxy's idle timeout (a
	// gateway behind Cloudflare answers 524 after 100 seconds of silence,
	// Bug #608). A reply that arrives as plain JSON still decodes.
	Stream bool
	// ServiceTier asks Responses and Chat endpoints to serve the call at that
	// tier (service_tier; TierPriority is xAI's and OpenAI's fast serving).
	// Messages has no priority tier, so Anthropic never gets it.
	ServiceTier string
}

// TierPriority is the service_tier that asks for fast serving, billed at the
// model's priority rate (ListPrice.Priority).
const TierPriority = "priority"

// Format asks the endpoint for structured output: OpenAI Responses
// text.format and Chat Completions response_format (json_schema). Anthropic
// Messages has no equivalent here, so the schema reaches it only through the
// prompt (Structured puts it there for every style).
type Format struct {
	Name   string
	Schema json.RawMessage
	Strict bool // the endpoint must follow the schema exactly; the schema must use only keywords strict mode allows
}

// Response is a provider-neutral reply.
type Response struct {
	Text      string
	Model     string
	TokensIn  int
	TokensOut int // reasoning and thinking tokens included: they bill as output
	// CachedIn and CacheWriteIn are the parts of TokensIn read from and written
	// to the provider's prompt cache, which bill at their own rates (Cost).
	// CacheWrite1hIn is the part of CacheWriteIn written to Anthropic's 1-hour
	// cache, billed at 2x input instead of the 5-minute 1.25x.
	CachedIn       int
	CacheWriteIn   int
	CacheWrite1hIn int
	CostUSD        float64 // reported by the endpoint, when it reports one; else 0
	// StopReason is why the model stopped, as the endpoint reports it: Messages
	// stop_reason, Responses status (with incomplete_details.reason), Chat
	// finish_reason. Truncated is set when it stopped at the token cap.
	StopReason string
	Truncated  bool
	// ServiceTier is the tier the endpoint served the reply at (Responses and
	// Chat service_tier, e.g. "priority"), when it says.
	ServiceTier string
}

// CallTimeout bounds one call, a stream included, when the client has no
// HTTP client of its own. A streamed high-effort review keeps delivering
// events for many minutes (a gateway measured 10 minutes of grok-4.7
// reasoning, Bug #608), so it is well above a proxy's cut of an unstreamed
// reply (about 125 seconds) and under the merge gate's 30-minute inspect job,
// leaving that job time to install aayudha and fall back to the reviewer app.
// A 12-minute bound cut a live grok-4.7 stream on a 1.2k-line diff (#663).
// Nothing retries a call that timed out.
var CallTimeout = 25 * time.Minute

// Client is one way of calling a model.
type Client interface {
	Complete(ctx context.Context, r Request) (Response, error)
}

// Wire formats.
const (
	StyleMessages  = "messages"  // Anthropic Messages: POST /messages
	StyleResponses = "responses" // OpenAI Responses: POST /responses
	StyleChat      = "chat"      // OpenAI Chat Completions: POST /chat/completions
)

// FamilyStyle is the default wire format per family.
func FamilyStyle(f verbs.Family) string {
	switch f {
	case verbs.FamilyAnthropic:
		return StyleMessages
	case verbs.FamilyOpenAI:
		return StyleResponses
	}
	return StyleChat
}

// HTTPClient speaks one wire format to one endpoint.
type HTTPClient struct {
	BaseURL string // including the version path, e.g. https://gateway.example/v1
	Style   string
	Key     string
	KeyAuth string // "bearer" (default) or "x-api-key"
	HTTP    *http.Client
}

// Complete implements Client.
func (c HTTPClient) Complete(ctx context.Context, r Request) (Response, error) {
	if r.MaxTokens == 0 {
		r.MaxTokens = 4096
	}
	var path string
	var body any
	switch c.Style {
	case StyleMessages:
		path = "/messages"
		b := map[string]any{"model": r.Model, "max_tokens": r.MaxTokens, "messages": wire(c.Style, r.Messages)}
		if r.System != "" {
			b["system"] = r.System
		}
		if r.Stream {
			b["stream"] = true
		}
		if r.Effort != "" {
			b["output_config"] = map[string]string{"effort": r.Effort}
		}
		body = b
	case StyleResponses:
		path = "/responses"
		b := map[string]any{"model": r.Model, "input": wire(c.Style, r.Messages), "max_output_tokens": r.MaxTokens}
		if r.System != "" {
			b["instructions"] = r.System
		}
		if r.Effort != "" {
			b["reasoning"] = map[string]string{"effort": r.Effort}
		}
		if f := r.Format; f != nil {
			b["text"] = map[string]any{"format": map[string]any{"type": "json_schema", "name": f.Name, "schema": f.Schema, "strict": f.Strict}}
		}
		if r.CacheKey != "" {
			b["prompt_cache_key"] = r.CacheKey
		}
		if r.ServiceTier != "" {
			b["service_tier"] = r.ServiceTier
		}
		if r.Stream {
			b["stream"] = true
		}
		body = b
	case StyleChat:
		path = "/chat/completions"
		msgs := r.Messages
		if r.System != "" {
			msgs = append([]Message{{Role: "system", Content: r.System}}, msgs...)
		}
		b := map[string]any{"model": r.Model, "messages": wire(c.Style, msgs), "max_tokens": r.MaxTokens}
		if r.Effort != "" {
			b["reasoning_effort"] = r.Effort
		}
		if r.ServiceTier != "" {
			b["service_tier"] = r.ServiceTier
		}
		if f := r.Format; f != nil {
			b["response_format"] = map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": f.Name, "schema": f.Schema, "strict": f.Strict}}
		}
		if r.Stream {
			b["stream"] = true
			b["stream_options"] = map[string]bool{"include_usage": true}
		}
		body = b
	default:
		return Response{}, fmt.Errorf("unknown wire format %q", c.Style)
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return Response{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSuffix(c.BaseURL, "/")+path, bytes.NewReader(payload))
	if err != nil {
		return Response{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.KeyAuth == "x-api-key" {
		req.Header.Set("x-api-key", c.Key)
	} else {
		req.Header.Set("Authorization", "Bearer "+c.Key)
	}
	if c.Style == StyleMessages {
		req.Header.Set("anthropic-version", "2023-06-01")
	}
	// A caller's shorter timeout (the CLI's general 5-minute client) would cut
	// a long review mid-stream, so a model call always gets CallTimeout.
	hc := &http.Client{Timeout: CallTimeout}
	if c.HTTP != nil {
		cp := *c.HTTP
		cp.Timeout = max(cp.Timeout, CallTimeout)
		hc = &cp
	}
	resp, err := hc.Do(req)
	if err != nil {
		return Response{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 == 2 && strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		body := io.LimitReader(resp.Body, 32<<20)
		switch c.Style {
		case StyleMessages:
			return decodeMessagesStream(body)
		case StyleResponses:
			return decodeResponsesStream(body)
		case StyleChat:
			return decodeChatStream(body)
		}
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return Response{}, err
	}
	if resp.StatusCode == http.StatusBadRequest && r.Format != nil && c.Style != StyleMessages {
		// The endpoint (or a gateway in front of it) does not take structured
		// output for this model: ask again with the schema in the prompt only.
		r.Format = nil
		return c.Complete(ctx, r)
	}
	if resp.StatusCode/100 != 2 {
		return Response{}, &StatusError{Code: resp.StatusCode, Body: truncate(string(raw), 300), RetryAfter: retryAfter(resp.Header.Get("Retry-After"), time.Now())}
	}
	return decode(c.Style, raw)
}

// decodeMessagesStream reads an Anthropic Messages event stream: the text
// deltas in order, the stop reason, and the usage from message_start and
// message_delta. An error event is an error, and so is a stream cut off
// before message_stop; both come back with the usage so far.
func decodeMessagesStream(body io.Reader) (Response, error) {
	var r Response
	var sb strings.Builder
	var stopped bool
	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 64<<10), 8<<20)
	for sc.Scan() {
		data, ok := strings.CutPrefix(sc.Text(), "data:")
		if !ok {
			continue
		}
		var ev struct {
			Type    string `json:"type"`
			Message struct {
				Model string `json:"model"`
				Usage usage  `json:"usage"`
			} `json:"message"`
			Delta struct {
				Type       string `json:"type"`
				Text       string `json:"text"`
				StopReason string `json:"stop_reason"`
			} `json:"delta"`
			Usage usage `json:"usage"`
			Error struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(data)), &ev); err != nil {
			if !sc.Scan() {
				// The stream was cut inside this event: report the read
				// error (a dropped connection), not the partial line.
				return r, fmt.Errorf("read messages stream: %w", cmp.Or(sc.Err(), io.ErrUnexpectedEOF))
			}
			return r, fmt.Errorf("decode messages stream: %w", err)
		}
		switch ev.Type {
		case "message_start":
			u := ev.Message.Usage
			r.Model = ev.Message.Model
			r.TokensIn = u.InputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens
			r.CachedIn, r.CacheWriteIn, r.CacheWrite1hIn = u.CacheReadInputTokens, u.CacheCreationInputTokens, u.CacheCreation.Ephemeral1hInputTokens
			r.TokensOut = u.OutputTokens
		case "content_block_delta":
			if ev.Delta.Type == "text_delta" {
				sb.WriteString(ev.Delta.Text)
			}
		case "message_delta":
			if ev.Delta.StopReason != "" {
				r.StopReason, r.Truncated = ev.Delta.StopReason, ev.Delta.StopReason == "max_tokens"
			}
			if ev.Usage.OutputTokens > 0 {
				r.TokensOut = ev.Usage.OutputTokens
			}
			if ev.Usage.Cost != nil {
				r.CostUSD = *ev.Usage.Cost
			}
		case "message_stop":
			stopped = true
		case "error":
			// The usage so far comes back with the error: the attempt may be
			// billed, so a caller charges and records it.
			return r, &StreamError{Type: ev.Error.Type, Message: truncate(ev.Error.Message, 300)}
		}
	}
	if err := sc.Err(); err != nil {
		return r, fmt.Errorf("read messages stream: %w", err)
	}
	if !stopped {
		// The stream ended between events, before message_stop: what arrived
		// is a prefix of the reply, not the reply.
		return r, fmt.Errorf("read messages stream: no message_stop: %w", io.ErrUnexpectedEOF)
	}
	r.Text = sb.String()
	if r.Text == "" {
		return r, noText(r, fmt.Errorf("messages stream has no text (stop reason %q)", r.StopReason))
	}
	return r, nil
}

// sseData calls f with each event's data line of a server-sent event stream
// until f says stop. A data line that is not JSON with no line after it is
// the stream cut inside an event: the read error (or an unexpected EOF) comes
// back, so the caller retries a dropped connection.
func sseData(body io.Reader, what string, f func(data string) (stop bool, err error)) error {
	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 64<<10), 8<<20)
	for sc.Scan() {
		data, ok := strings.CutPrefix(sc.Text(), "data:")
		if !ok {
			continue
		}
		data = strings.TrimSpace(data)
		if data != "[DONE]" && !json.Valid([]byte(data)) {
			if !sc.Scan() {
				return fmt.Errorf("read %s stream: %w", what, cmp.Or(sc.Err(), io.ErrUnexpectedEOF))
			}
			return fmt.Errorf("decode %s stream: not JSON: %s", what, truncate(data, 100))
		}
		stop, err := f(data)
		if err != nil || stop {
			return err
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("read %s stream: %w", what, err)
	}
	return nil
}

// decodeResponsesStream reads an OpenAI Responses event stream. The final
// response.completed (or response.incomplete) event carries the whole
// response, which decodes exactly as a non-streamed reply does. A
// response.failed event's response carries the usage, which comes back with
// the error: the attempt may be billed.
func decodeResponsesStream(body io.Reader) (Response, error) {
	var final json.RawMessage
	var r Response
	err := sseData(body, "responses", func(data string) (bool, error) {
		var ev struct {
			Type     string          `json:"type"`
			Response json.RawMessage `json:"response"`
			Message  string          `json:"message"`
			Code     string          `json:"code"`
		}
		_ = json.Unmarshal([]byte(data), &ev)
		switch ev.Type {
		case "response.completed", "response.incomplete":
			final = ev.Response
			return true, nil
		case "response.failed", "error":
			var v struct {
				Model string `json:"model"`
				Usage usage  `json:"usage"`
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			_ = json.Unmarshal(ev.Response, &v)
			r.Model = v.Model
			r.setUsage(v.Usage)
			return false, &StreamError{Type: cmp.Or(ev.Code, v.Error.Code, ev.Type), Message: truncate(cmp.Or(ev.Message, v.Error.Message), 300)}
		}
		return false, nil
	})
	if err != nil {
		return r, err
	}
	if final == nil {
		return r, fmt.Errorf("read responses stream: no response.completed: %w", io.ErrUnexpectedEOF)
	}
	return decode(StyleResponses, final)
}

// decodeChatStream reads a Chat Completions stream: the content deltas of
// the first choice, its finish reason, and the usage chunk that
// stream_options.include_usage adds, rebuilt into the non-streamed reply so
// it decodes the same way. The stream is complete at [DONE] or the usage
// chunk; an error chunk, or a stream cut off before either, comes back with
// the usage seen so far.
func decodeChatStream(body io.Reader) (Response, error) {
	var r Response
	var finish string
	var u json.RawMessage
	var sb strings.Builder
	complete := false
	err := sseData(body, "chat", func(data string) (bool, error) {
		if data == "[DONE]" {
			complete = true
			return true, nil
		}
		var ch struct {
			Model       string          `json:"model"`
			ServiceTier string          `json:"service_tier"`
			Usage       json.RawMessage `json:"usage"`
			Choices     []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Error *struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal([]byte(data), &ch)
		r.Model, r.ServiceTier = cmp.Or(ch.Model, r.Model), cmp.Or(ch.ServiceTier, r.ServiceTier)
		if len(ch.Usage) > 0 && string(ch.Usage) != "null" {
			var uu usage
			_ = json.Unmarshal(ch.Usage, &uu)
			r.setUsage(uu)
			u, complete = ch.Usage, true
		}
		if ch.Error != nil {
			return false, &StreamError{Type: cmp.Or(ch.Error.Type, "error"), Message: truncate(ch.Error.Message, 300)}
		}
		if len(ch.Choices) > 0 {
			sb.WriteString(ch.Choices[0].Delta.Content)
			finish = cmp.Or(ch.Choices[0].FinishReason, finish)
		}
		return false, nil
	})
	if err != nil {
		return r, err
	}
	if !complete {
		return r, fmt.Errorf("read chat stream: no [DONE] or usage: %w", io.ErrUnexpectedEOF)
	}
	type msg struct {
		Content string `json:"content"`
	}
	type choice struct {
		Message      msg    `json:"message"`
		FinishReason string `json:"finish_reason"`
	}
	raw, _ := json.Marshal(struct {
		Model       string          `json:"model"`
		ServiceTier string          `json:"service_tier,omitempty"`
		Usage       json.RawMessage `json:"usage,omitempty"`
		Choices     []choice        `json:"choices"`
	}{r.Model, r.ServiceTier, u, []choice{{msg{sb.String()}, finish}}})
	return decode(StyleChat, raw)
}

// noText is the error for a reply without text. One that stopped at its token
// cap spent the cap before writing (on thinking): it is a truncated reply, so a
// caller that reruns those at a larger cap reruns this one too.
func noText(r Response, err error) error {
	if r.Truncated {
		return &ReplyError{Truncated: true, Err: err}
	}
	return err
}

// StreamError is an error event inside a streamed reply, after the
// endpoint answered 200: its Type is the provider's (api_error, overloaded_error,
// invalid_request_error, ...).
type StreamError struct {
	Type    string
	Message string
}

func (e *StreamError) Error() string {
	return fmt.Sprintf("stream error: %s: %s", e.Type, e.Message)
}

// StatusError is a non-2xx reply.
type StatusError struct {
	Code int
	Body string
	// RetryAfter is the reply's Retry-After header (seconds or an HTTP date),
	// 0 when it has none.
	RetryAfter time.Duration
}

// retryAfter parses a Retry-After header value at now.
func retryAfter(v string, now time.Time) time.Duration {
	if v == "" {
		return 0
	}
	if s, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
		return max(time.Duration(s)*time.Second, 0)
	}
	if t, err := http.ParseTime(v); err == nil {
		return max(t.Sub(now), 0)
	}
	return 0
}

func (e *StatusError) Error() string {
	switch e.Code {
	case http.StatusGatewayTimeout, 524:
		// A proxy in front of the endpoint gave up waiting for the reply
		// (Cloudflare's 524 after 100 seconds without a byte).
		return fmt.Sprintf("model endpoint returned %d, a timeout waiting for the reply (rerun; if it recurs, check that the route streams): %s", e.Code, e.Body)
	}
	return fmt.Sprintf("model endpoint returned %d: %s", e.Code, e.Body)
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

type usage struct {
	InputTokens int `json:"input_tokens"`
	// Anthropic Messages reports cached prompt input apart from input_tokens;
	// both count as input for cost and the [budget] breaker, each at its own
	// rate. cache_creation splits the writes by TTL (its parts sum to
	// cache_creation_input_tokens); the 1-hour part bills at CacheWrite1h.
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheCreation            struct {
		Ephemeral1hInputTokens int `json:"ephemeral_1h_input_tokens"`
	} `json:"cache_creation"`
	OutputTokens     int      `json:"output_tokens"`
	PromptTokens     int      `json:"prompt_tokens"`
	CompletionTokens int      `json:"completion_tokens"`
	TotalTokens      int      `json:"total_tokens"`
	Cost             *float64 `json:"cost"` // some gateways report the charge
	// OpenAI Responses: cached_tokens and cache_write_tokens are parts of
	// input_tokens; Chat Completions (xAI, Gemini): cached_tokens of prompt_tokens.
	InputTokensDetails struct {
		CachedTokens     int `json:"cached_tokens"`
		CacheWriteTokens int `json:"cache_write_tokens"`
	} `json:"input_tokens_details"`
	PromptTokensDetails struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
}

func decode(style string, raw []byte) (Response, error) {
	var v struct {
		Model             string `json:"model"`
		Usage             usage  `json:"usage"`
		StopReason        string `json:"stop_reason"` // messages
		Status            string `json:"status"`      // responses
		IncompleteDetails struct {
			Reason string `json:"reason"`
		} `json:"incomplete_details"` // responses
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"` // messages
		Output []struct {
			Type    string `json:"type"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"` // responses
		OutputText  string `json:"output_text"`
		ServiceTier string `json:"service_tier"` // responses, chat
		Choices     []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"` // chat
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return Response{}, fmt.Errorf("decode %s reply: %w", style, err)
	}
	r := Response{Model: v.Model, ServiceTier: v.ServiceTier}
	r.setUsage(v.Usage)
	var sb strings.Builder
	switch style {
	case StyleMessages:
		r.StopReason, r.Truncated = v.StopReason, v.StopReason == "max_tokens"
		for _, c := range v.Content {
			if c.Type == "text" {
				sb.WriteString(c.Text)
			}
		}
	case StyleResponses:
		r.StopReason = v.Status
		if v.IncompleteDetails.Reason != "" {
			r.StopReason += ":" + v.IncompleteDetails.Reason
		}
		r.Truncated = v.Status == "incomplete" && v.IncompleteDetails.Reason == "max_output_tokens"
		for _, o := range v.Output {
			for _, c := range o.Content {
				if c.Type == "output_text" {
					sb.WriteString(c.Text)
				}
			}
		}
		if sb.Len() == 0 {
			sb.WriteString(v.OutputText)
		}
	case StyleChat:
		if len(v.Choices) > 0 {
			sb.WriteString(v.Choices[0].Message.Content)
			r.StopReason, r.Truncated = v.Choices[0].FinishReason, v.Choices[0].FinishReason == "length"
		}
	}
	r.Text = sb.String()
	if r.Text == "" {
		return r, noText(r, fmt.Errorf("%s reply has no text: %s", style, truncate(string(raw), 300)))
	}
	return r, nil
}

// setUsage sets the tokens and cost of a usage object of any wire format.
func (r *Response) setUsage(u usage) {
	r.TokensIn = u.InputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens + u.PromptTokens
	r.TokensOut = u.OutputTokens + u.CompletionTokens
	r.CachedIn = u.CacheReadInputTokens + u.InputTokensDetails.CachedTokens + u.PromptTokensDetails.CachedTokens
	r.CacheWriteIn, r.CacheWrite1hIn = u.CacheCreationInputTokens+u.InputTokensDetails.CacheWriteTokens, u.CacheCreation.Ephemeral1hInputTokens
	// Some chat endpoints (xAI) leave reasoning tokens out of completion_tokens
	// but in total_tokens; they bill as output, so the difference counts.
	if extra := u.TotalTokens - r.TokensIn - r.TokensOut; u.TotalTokens > 0 && extra > 0 {
		r.TokensOut += extra
	}
	if u.Cost != nil {
		r.CostUSD = *u.Cost
	}
}

// Direct provider endpoints, the fallback when [models.gateway] is unset. Each
// reads its key from the secret named after the family (e.g. AAYU_SECRET_ANTHROPIC).
var directProviders = map[verbs.Family]struct {
	BaseURL, Style, KeyAuth string
}{
	verbs.FamilyAnthropic: {"https://api.anthropic.com/v1", StyleMessages, "x-api-key"},
	verbs.FamilyOpenAI:    {"https://api.openai.com/v1", StyleResponses, "bearer"},
	verbs.FamilyGoogle:    {"https://generativelanguage.googleapis.com/v1beta/openai", StyleChat, "bearer"},
	verbs.FamilyXAI:       {"https://api.x.ai/v1", StyleChat, "bearer"},
}

// DirectSecrets are the secret names of the direct providers' keys, sorted.
// Each is a pay-as-you-go key: a call made with it is metered spend.
func DirectSecrets() []string {
	var out []string
	for f := range directProviders {
		out = append(out, string(f))
	}
	slices.Sort(out)
	return out
}

// Route says how a model is reached.
type Route struct {
	Via     string `json:"via"` // "gateway" or "direct"
	BaseURL string `json:"base_url"`
	Style   string `json:"style"`
	Secret  string `json:"secret"`
	KeyAuth string `json:"-"`
}

// RouteFor picks the endpoint and wire format for a resolved model: the
// gateway when configured, else the family's direct provider.
func RouteFor(m config.Models, model string, family verbs.Family) (Route, error) {
	if family == verbs.FamilyHostManaged || model == "" {
		return Route{}, errors.New("a host-managed mode cannot be called directly")
	}
	if g := m.Gateway; g.BaseURL != "" {
		style := g.APIStyle
		if style == "" {
			style = FamilyStyle(family)
		}
		for _, match := range []string{string(family), model} { // a model entry beats a family entry
			for _, st := range g.Styles {
				if k, v, _ := strings.Cut(st, "="); k == match {
					style = v
				}
			}
		}
		return Route{Via: "gateway", BaseURL: g.BaseURL, Style: style, Secret: g.Secret, KeyAuth: "bearer"}, nil
	}
	p, ok := directProviders[family]
	if !ok {
		return Route{}, fmt.Errorf("no direct provider for family %q; configure [models.gateway]", family)
	}
	return Route{Via: "direct", BaseURL: p.BaseURL, Style: p.Style, Secret: string(family), KeyAuth: p.KeyAuth}, nil
}
