package models

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// Message is one turn of a conversation.
type Message struct {
	Role    string `json:"role"` // user or assistant
	Content string `json:"content"`
}

// Request is a provider-neutral model call.
type Request struct {
	Model     string
	System    string
	Messages  []Message
	MaxTokens int
	Effort    string  // reasoning effort; empty for the provider default
	Format    *Format // ask for JSON matching a schema where the wire format supports it; nil for free text
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
		b := map[string]any{"model": r.Model, "max_tokens": r.MaxTokens, "messages": r.Messages}
		if r.System != "" {
			b["system"] = r.System
		}
		body = b
	case StyleResponses:
		path = "/responses"
		b := map[string]any{"model": r.Model, "input": r.Messages, "max_output_tokens": r.MaxTokens}
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
		b := map[string]any{"model": r.Model, "messages": msgs, "max_tokens": r.MaxTokens}
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
	InputTokens      int      `json:"input_tokens"`
	OutputTokens     int      `json:"output_tokens"`
	PromptTokens     int      `json:"prompt_tokens"`
	CompletionTokens int      `json:"completion_tokens"`
	Cost             *float64 `json:"cost"` // some gateways report the charge
}

func decode(style string, raw []byte) (Response, error) {
	var v struct {
		Model   string `json:"model"`
		Usage   usage  `json:"usage"`
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
		} `json:"choices"` // chat
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return Response{}, fmt.Errorf("decode %s reply: %w", style, err)
	}
	r := Response{Model: v.Model, TokensIn: v.Usage.InputTokens + v.Usage.PromptTokens, TokensOut: v.Usage.OutputTokens + v.Usage.CompletionTokens}
	if v.Usage.Cost != nil {
		r.CostUSD = *v.Usage.Cost
	}
	var sb strings.Builder
	switch style {
	case StyleMessages:
		for _, c := range v.Content {
			if c.Type == "text" {
				sb.WriteString(c.Text)
			}
		}
	case StyleResponses:
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
