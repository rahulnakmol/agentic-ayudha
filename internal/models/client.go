package models

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
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
	if !slices.ContainsFunc(msgs, func(m Message) bool { return len(m.Images) > 0 }) {
		return msgs
	}
	out := make([]map[string]any, len(msgs))
	for i, m := range msgs {
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
	// Stream asks an Anthropic Messages endpoint to stream the reply, so a long
	// generation keeps bytes flowing past a proxy's idle timeout (a gateway
	// behind Cloudflare answers 524 after 100 seconds of silence). Other wire
	// formats ignore it, and a reply that arrives as plain JSON still decodes.
	Stream bool
}

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
	TokensOut int
	CostUSD   float64 // reported by the endpoint, when it reports one; else 0
	// StopReason is why the model stopped, as the endpoint reports it: Messages
	// stop_reason, Responses status (with incomplete_details.reason), Chat
	// finish_reason. Truncated is set when it stopped at the token cap.
	StopReason string
	Truncated  bool
}

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
		if f := r.Format; f != nil {
			b["response_format"] = map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": f.Name, "schema": f.Schema, "strict": f.Strict}}
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
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 10 * time.Minute}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return Response{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 == 2 && c.Style == StyleMessages && strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		return decodeMessagesStream(io.LimitReader(resp.Body, 32<<20))
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
		return Response{}, &StatusError{Code: resp.StatusCode, Body: truncate(string(raw), 300)}
	}
	return decode(c.Style, raw)
}

// decodeMessagesStream reads an Anthropic Messages event stream: the text
// deltas in order, the stop reason, and the usage from message_start and
// message_delta. An error event is an error.
func decodeMessagesStream(body io.Reader) (Response, error) {
	var r Response
	var sb strings.Builder
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
			return Response{}, fmt.Errorf("decode messages stream: %w", err)
		}
		switch ev.Type {
		case "message_start":
			u := ev.Message.Usage
			r.Model = ev.Message.Model
			r.TokensIn = u.InputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens
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
		case "error":
			return Response{}, fmt.Errorf("messages stream error: %s: %s", ev.Error.Type, truncate(ev.Error.Message, 300))
		}
	}
	if err := sc.Err(); err != nil {
		return Response{}, fmt.Errorf("read messages stream: %w", err)
	}
	r.Text = sb.String()
	if r.Text == "" {
		return r, fmt.Errorf("messages stream has no text (stop reason %q)", r.StopReason)
	}
	return r, nil
}

// StatusError is a non-2xx reply.
type StatusError struct {
	Code int
	Body string
}

func (e *StatusError) Error() string {
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
	// both count as input (conservatively at the full input rate) for cost and
	// the [budget] breaker.
	CacheReadInputTokens     int      `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int      `json:"cache_creation_input_tokens"`
	OutputTokens             int      `json:"output_tokens"`
	PromptTokens             int      `json:"prompt_tokens"`
	CompletionTokens         int      `json:"completion_tokens"`
	Cost                     *float64 `json:"cost"` // some gateways report the charge
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
		OutputText string `json:"output_text"`
		Choices    []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"` // chat
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return Response{}, fmt.Errorf("decode %s reply: %w", style, err)
	}
	u := v.Usage
	r := Response{Model: v.Model, TokensIn: u.InputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens + u.PromptTokens,
		TokensOut: u.OutputTokens + u.CompletionTokens}
	if v.Usage.Cost != nil {
		r.CostUSD = *v.Usage.Cost
	}
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
		return r, fmt.Errorf("%s reply has no text: %s", style, truncate(string(raw), 300))
	}
	return r, nil
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
