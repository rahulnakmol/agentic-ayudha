package models

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/notify"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

func TestLaneResolution(t *testing.T) {
	cfg := config.Models{Lanes: map[string]map[string]config.ModelChoice{
		"build":  {"claude": {Model: verbs.ModelClaudeSonnet}},
		"review": {"cursor": {Model: verbs.ModelGPTSol}, "claude": {Model: "my-model", Family: "xai"}},
	}}
	var logged []string
	r := Resolver{Models: cfg, Log: func(s string) { logged = append(logged, s) }}
	tests := []struct {
		lane           verbs.LaneID
		host           verbs.HostID
		direct, heavy  bool
		model          string
		family         verbs.Family
		source, effort string
	}{
		// Registry defaults, one row per host.
		{verbs.LaneBuild, verbs.HostAmp, false, false, "", verbs.FamilyHostManaged, SourceDefault, ""},
		{verbs.LaneBuild, verbs.HostCursor, false, false, verbs.ModelClaudeOpus, verbs.FamilyAnthropic, SourceDefault, ""},
		{verbs.LaneReview, verbs.HostAmp, false, false, verbs.ModelGPTSol, verbs.FamilyOpenAI, SourceDefault, ""},
		{verbs.LaneUIReview, verbs.HostCursor, false, false, verbs.ModelGeminiPro, verbs.FamilyGoogle, SourceDefault, ""},
		{verbs.LaneChore, verbs.HostClaude, false, false, verbs.ModelClaudeHaiku, verbs.FamilyAnthropic, SourceDefault, ""},
		// Config overrides.
		{verbs.LaneBuild, verbs.HostClaude, false, false, verbs.ModelClaudeSonnet, verbs.FamilyAnthropic, SourceConfig, ""},
		{verbs.LaneReview, verbs.HostClaude, false, false, "my-model", verbs.FamilyXAI, SourceConfig, ""},
		// Cursor cannot run GPT: the configured GPT lane resolves to the Grok default.
		{verbs.LaneReview, verbs.HostCursor, false, false, verbs.ModelGrok, verbs.FamilyXAI, SourceFallback, ""},
		// The binary's own calls cannot use a host-managed mode.
		{verbs.LaneChore, verbs.HostAmp, true, false, verbs.ModelClaudeHaiku, verbs.FamilyAnthropic, SourceDirect, ""},
		{verbs.LaneUIReview, verbs.HostAmp, true, false, verbs.ModelGeminiPro, verbs.FamilyGoogle, SourceDirect, ""},
		// Heavy reviews escalate to GPT-6 Astra at high effort.
		{verbs.LaneReview, verbs.HostAmp, true, true, verbs.ModelGPTAstra, verbs.FamilyOpenAI, SourceDefault, "high"},
	}
	for _, tt := range tests {
		var got Resolved
		var err error
		if tt.direct {
			got, err = r.ResolveDirect(tt.lane, tt.host, tt.heavy)
		} else {
			got, err = r.Resolve(tt.lane, tt.host)
		}
		if err != nil || got.Model != tt.model || got.Family != tt.family || got.Source != tt.source || got.Effort != tt.effort {
			t.Errorf("%s on %s (direct %v, heavy %v) = %+v, %v; want %s %s %s effort %q",
				tt.lane, tt.host, tt.direct, tt.heavy, got, err, tt.model, tt.family, tt.source, tt.effort)
		}
	}
	joined := strings.Join(logged, "\n")
	if !strings.Contains(joined, "gpt-6.1-sol (openai) is not available on Cursor; using the lane's Cursor default grok-4.7") {
		t.Errorf("substitution not logged; log:\n%s", joined)
	}
}

func TestReviewerFamilyMustDiffer(t *testing.T) {
	if err := CheckFamilies("#12", verbs.FamilyAnthropic, verbs.FamilyAnthropic); err == nil {
		t.Fatal("builder = reviewer family accepted")
	}
	if err := CheckFamilies("#12", verbs.FamilyHostManaged, verbs.FamilyHostManaged); err == nil {
		t.Fatal("host-managed reviewer accepted")
	}
	if err := CheckFamilies("#12", verbs.FamilyAnthropic, verbs.FamilyOpenAI); err != nil {
		t.Fatal(err)
	}
	// One gateway serves every family; the family still comes from the model.
	gw := config.Models{Gateway: config.Gateway{BaseURL: "https://gw.example/v1", Secret: "ai-gateway"}}
	a, _ := RouteFor(gw, verbs.ModelClaudeOpus, verbs.FamilyOf(verbs.ModelClaudeOpus))
	b, _ := RouteFor(gw, verbs.ModelClaudeFable, verbs.FamilyOf(verbs.ModelClaudeFable))
	if a.BaseURL != b.BaseURL || CheckFamilies("#12", verbs.FamilyOf(verbs.ModelClaudeOpus), verbs.FamilyOf(verbs.ModelClaudeFable)) == nil {
		t.Fatal("Fable (anthropic) accepted as reviewer of an Opus build because both go through one gateway")
	}
}

func TestRouting(t *testing.T) {
	gw := config.Models{Gateway: config.Gateway{BaseURL: "https://gw.example/v1", Secret: "ai-gateway"}}
	styled := gw
	styled.Gateway.Styles = []string{"xai=responses", "grok-4.7=chat", "gpt-6-astra=chat"}
	global := gw
	global.Gateway.APIStyle = StyleChat
	tests := []struct {
		name   string
		m      config.Models
		model  string
		via    string
		style  string
		secret string
	}{
		{"anthropic via gateway: messages", gw, verbs.ModelClaudeOpus, "gateway", StyleMessages, "ai-gateway"},
		{"openai via gateway: responses", gw, verbs.ModelGPTSol, "gateway", StyleResponses, "ai-gateway"},
		{"xai via gateway: chat", gw, verbs.ModelGrok, "gateway", StyleChat, "ai-gateway"},
		{"google via gateway: chat", gw, verbs.ModelGeminiPro, "gateway", StyleChat, "ai-gateway"},
		{"model entry beats family entry", styled, verbs.ModelGrok, "gateway", StyleChat, "ai-gateway"},
		{"family entry applies to other models", styled, "grok-5", "gateway", StyleResponses, "ai-gateway"},
		{"per-model override of a family default", styled, verbs.ModelGPTAstra, "gateway", StyleChat, "ai-gateway"},
		{"api_style applies to every model without an entry", global, verbs.ModelClaudeOpus, "gateway", StyleChat, "ai-gateway"},
		{"no gateway: direct Anthropic", config.Models{}, verbs.ModelClaudeOpus, "direct", StyleMessages, "anthropic"},
		{"no gateway: direct xAI", config.Models{}, verbs.ModelGrok, "direct", StyleChat, "xai"},
	}
	for _, tt := range tests {
		r, err := RouteFor(tt.m, tt.model, verbs.FamilyOf(tt.model))
		if err != nil || r.Via != tt.via || r.Style != tt.style || r.Secret != tt.secret {
			t.Errorf("%s: %+v, %v", tt.name, r, err)
		}
	}
	if _, err := RouteFor(gw, "", verbs.FamilyHostManaged); err == nil {
		t.Error("host-managed mode routed")
	}
}

// TestWireFormats checks each wire format against a server that answers the
// way the gateway does, including auth headers and token usage.
func TestWireFormats(t *testing.T) {
	var got struct {
		path, auth, version string
		body                map[string]any
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.path, got.auth, got.version = r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("anthropic-version")
		b, _ := io.ReadAll(r.Body)
		got.body = nil
		_ = json.Unmarshal(b, &got.body)
		switch r.URL.Path {
		case "/v1/messages":
			_, _ = w.Write([]byte(`{"model":"claude-haiku-5-5","content":[{"type":"thinking","text":"x"},{"type":"text","text":"hello"}],"usage":{"input_tokens":11,"output_tokens":3}}`))
		case "/v1/responses":
			_, _ = w.Write([]byte(`{"model":"gpt-6.1-sol","output":[{"type":"reasoning","content":[]},{"type":"message","content":[{"type":"output_text","text":"hello"}]}],"usage":{"input_tokens":12,"output_tokens":4}}`))
		case "/v1/chat/completions":
			_, _ = w.Write([]byte(`{"model":"grok-4.7","choices":[{"message":{"role":"assistant","content":"hello"}}],"usage":{"prompt_tokens":13,"completion_tokens":5,"cost":0.0042}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	req := Request{System: "be brief", Messages: []Message{{Role: "user", Content: "hi"}}, MaxTokens: 50, Effort: "high"}
	tests := []struct {
		style, path string
		in, out     int
		cost        float64
		check       func(map[string]any) bool
	}{
		{StyleMessages, "/v1/messages", 11, 3, 0, func(b map[string]any) bool { return b["system"] == "be brief" && b["max_tokens"] == 50.0 }},
		{StyleResponses, "/v1/responses", 12, 4, 0, func(b map[string]any) bool {
			return b["instructions"] == "be brief" && b["reasoning"].(map[string]any)["effort"] == "high"
		}},
		{StyleChat, "/v1/chat/completions", 13, 5, 0.0042, func(b map[string]any) bool {
			msgs := b["messages"].([]any)
			return msgs[0].(map[string]any)["role"] == "system" && b["reasoning_effort"] == "high"
		}},
	}
	// Structured output: Responses text.format and Chat response_format; Messages has none.
	sreq := req
	sreq.Format = &Format{Name: "verdict", Schema: json.RawMessage(`{"type":"object"}`), Strict: true}
	for style, check := range map[string]func(map[string]any) bool{
		StyleResponses: func(b map[string]any) bool {
			f, _ := b["text"].(map[string]any)["format"].(map[string]any)
			return f["type"] == "json_schema" && f["name"] == "verdict" && f["strict"] == true && f["schema"] != nil
		},
		StyleChat: func(b map[string]any) bool {
			f, _ := b["response_format"].(map[string]any)
			return f["type"] == "json_schema" && f["json_schema"].(map[string]any)["name"] == "verdict"
		},
		StyleMessages: func(b map[string]any) bool { return b["text"] == nil && b["response_format"] == nil },
	} {
		c := HTTPClient{BaseURL: srv.URL + "/v1", Style: style, Key: "k", HTTP: srv.Client()}
		if _, err := c.Complete(context.Background(), sreq); err != nil || !check(got.body) {
			t.Errorf("%s structured output: %v, sent %v", style, err, got.body)
		}
	}
	for _, tt := range tests {
		c := HTTPClient{BaseURL: srv.URL + "/v1", Style: tt.style, Key: "k-123", HTTP: srv.Client()}
		r, err := c.Complete(context.Background(), req)
		if err != nil || r.Text != "hello" || r.TokensIn != tt.in || r.TokensOut != tt.out || r.CostUSD != tt.cost {
			t.Errorf("%s: %+v, %v", tt.style, r, err)
		}
		if got.path != tt.path || got.auth != "Bearer k-123" || !tt.check(got.body) {
			t.Errorf("%s: sent %s auth %q body %v", tt.style, got.path, got.auth, got.body)
		}
		if (tt.style == StyleMessages) != (got.version != "") {
			t.Errorf("%s: anthropic-version header %q", tt.style, got.version)
		}
	}
	// Images (review lane A, WP6.6): each format's own image block, before the text;
	// turns without images stay plain strings.
	ireq := req
	ireq.Messages = []Message{{Role: "user", Content: "look", Images: []Image{{MediaType: "image/png", Data: []byte("PNG")}}}, {Role: "assistant", Content: "ok"}}
	for style, want := range map[string]string{
		StyleMessages:  `[{"content":[{"source":{"data":"UE5H","media_type":"image/png","type":"base64"},"type":"image"},{"text":"look","type":"text"}],"role":"user"},{"content":"ok","role":"assistant"}]`,
		StyleResponses: `[{"content":[{"image_url":"data:image/png;base64,UE5H","type":"input_image"},{"text":"look","type":"input_text"}],"role":"user"},{"content":"ok","role":"assistant"}]`,
		StyleChat:      `[{"content":"be brief","role":"system"},{"content":[{"image_url":{"url":"data:image/png;base64,UE5H"},"type":"image_url"},{"text":"look","type":"text"}],"role":"user"},{"content":"ok","role":"assistant"}]`,
	} {
		c := HTTPClient{BaseURL: srv.URL + "/v1", Style: style, Key: "k", HTTP: srv.Client()}
		if _, err := c.Complete(context.Background(), ireq); err != nil {
			t.Fatal(err)
		}
		key := "messages"
		if style == StyleResponses {
			key = "input"
		}
		if b, _ := json.Marshal(got.body[key]); string(b) != want {
			t.Errorf("%s images: %s\nwant %s", style, b, want)
		}
	}
}

func TestBudgetBreaker(t *testing.T) {
	london, _ := config.NewClock("Europe/London", "Mon-Fri 09:00-18:00")
	now := func() time.Time { return time.Date(2026, 10, 12, 10, 0, 0, 0, london.Loc) } // Monday, in hours
	setup := func(t *testing.T, budget config.Budget, cost float64) (*Gateway, *Fake, *notify.Fake, *Breaker) {
		dir := t.TempDir()
		sink := &notify.Fake{}
		log, _ := actions.Open(filepath.Join(dir, "actions.jsonl"), actions.Options{Now: now})
		br := &Breaker{Budget: budget, Clock: london, Now: now, PauseFile: filepath.Join(dir, "state", "paused"),
			Notify: &notify.Sender{Sink: sink, Clock: london, Now: now, State: filepath.Join(dir, "state", "notify.json"), Actions: log}}
		fake := &Fake{Replies: []Response{{Text: "ok", TokensIn: 100, TokensOut: 10, CostUSD: cost}}}
		gw := &Gateway{Resolver: Resolver{}, Actions: log, Breaker: br,
			Key: func(string) (string, error) { return "k", nil }, Client: func(Route, string) Client { return fake }}
		return gw, fake, sink, br
	}
	call := Call{Story: "#12", Lane: verbs.LaneReview, Host: verbs.HostClaude, Req: Request{Messages: []Message{{Role: "user", Content: "review"}}}}

	t.Run("just below the limit does not trip", func(t *testing.T) {
		gw, fake, sink, br := setup(t, config.Budget{PerStoryUSD: 1.00}, 0.33)
		for i := 0; i < 3; i++ {
			if _, _, err := gw.Do(context.Background(), call); err != nil {
				t.Fatalf("call %d: %v", i+1, err)
			}
		}
		if len(fake.Requests) != 3 || len(sink.Pages) != 0 || br.Paused() {
			t.Fatalf("0.99 of 1.00 spent: %d calls, %d pages, paused %v", len(fake.Requests), len(sink.Pages), br.Paused())
		}
	})
	t.Run("reaching the limit trips once and pages once", func(t *testing.T) {
		gw, fake, sink, br := setup(t, config.Budget{PerStoryUSD: 1.00}, 0.25)
		var errs []error
		for i := 0; i < 6; i++ {
			_, _, err := gw.Do(context.Background(), call)
			errs = append(errs, err)
		}
		for i, err := range errs {
			if wantTrip := i >= 3; errors.Is(err, ErrBudget) != wantTrip {
				t.Fatalf("call %d: err %v, want tripped = %v", i+1, err, wantTrip)
			}
		}
		if len(fake.Requests) != 4 {
			t.Fatalf("%d provider calls, want 4 (the 4th reaches 1.00; later calls are refused before the provider)", len(fake.Requests))
		}
		if len(sink.Pages) != 1 || !strings.Contains(sink.Pages[0].Body, "per_story_usd = 1.00") || !br.Paused() {
			t.Fatalf("pages = %+v, paused %v; want exactly one page", sink.Pages, br.Paused())
		}
	})
	t.Run("per-day limit across stories", func(t *testing.T) {
		gw, _, sink, _ := setup(t, config.Budget{PerDayUSD: 0.50}, 0.25)
		for i, story := range []string{"#1", "#2", "#3"} {
			c := call
			c.Story = story
			_, _, err := gw.Do(context.Background(), c)
			if errors.Is(err, ErrBudget) != (i >= 1) {
				t.Fatalf("call %d: %v", i+1, err)
			}
		}
		if len(sink.Pages) != 1 {
			t.Fatalf("%d pages", len(sink.Pages))
		}
	})
	t.Run("cost is accounted in the actions log", func(t *testing.T) {
		gw, _, _, _ := setup(t, config.Budget{}, 0)
		gw.Models.Prices = []config.Price{{Model: verbs.ModelGPTSol, InputPerMTok: 2, OutputPerMTok: 8}}
		_, m, err := gw.Do(context.Background(), call)
		usd, source := Cost(gw.Models.Prices, m.Model, Response{TokensIn: 100, TokensOut: 10})
		if err != nil || source != "price-table" || usd != (100*2.0+10*8.0)/1e6 {
			t.Fatalf("%v %v %s", err, usd, source)
		}
		if usd, source := Cost(nil, "unknown", Response{TokensIn: 1e6}); source != "fallback-rate" || usd != FallbackInputPerMTok {
			t.Fatalf("fallback: %v %s", usd, source)
		}
	})
	t.Run("a log that cannot be read is an error, a missing one seeds nothing", func(t *testing.T) {
		dir := t.TempDir()
		bad := filepath.Join(dir, "actions.jsonl")
		_ = os.WriteFile(bad, []byte("{\"seq\":1}\nnot json\n"), 0o644)
		br := &Breaker{Budget: config.Budget{PerDayUSD: 1}, Clock: london, Now: now, PauseFile: filepath.Join(dir, "paused")}
		if err := br.SeedLog(filepath.Join(dir, "missing.jsonl")); err != nil || br.Check("") != nil {
			t.Fatalf("missing log: %v", err)
		}
		if err := br.SeedLog(bad); err == nil || !strings.Contains(err.Error(), "actions.jsonl") {
			t.Fatalf("malformed log: %v", err)
		}
		if err := br.SeedLog(filepath.Join(bad, "actions.jsonl")); err == nil {
			t.Fatal("a log under a file: want an error")
		}
	})
}

func TestNotifyGatingAndDedupe(t *testing.T) {
	london, _ := config.NewClock("Europe/London", "Mon-Fri 09:00-18:00")
	at := time.Date(2026, 10, 10, 12, 0, 0, 0, london.Loc) // Saturday
	sink := &notify.Fake{}
	s := &notify.Sender{Sink: sink, Clock: london, Now: func() time.Time { return at }, State: filepath.Join(t.TempDir(), "n.json")}
	ctx := context.Background()
	if st, _ := s.Page(ctx, notify.Page{Key: "digest"}); st != notify.Deferred || len(sink.Pages) != 0 {
		t.Fatalf("weekend page: %s, %d sent", st, len(sink.Pages))
	}
	if st, _ := s.Page(ctx, notify.Page{Key: "sev1", Urgent: true}); st != notify.Sent {
		t.Fatalf("urgent page on a weekend: %s", st)
	}
	at = time.Date(2026, 10, 12, 9, 30, 0, 0, london.Loc) // Monday morning
	if n, _ := s.Flush(ctx); n != 1 {
		t.Fatalf("flushed %d", n)
	}
	if st, _ := s.Page(ctx, notify.Page{Key: "digest"}); st != notify.Deduped || len(sink.Pages) != 2 {
		t.Fatalf("second digest the same day: %s, %d sent", st, len(sink.Pages))
	}
	at = at.Add(24 * time.Hour)
	if st, _ := s.Page(ctx, notify.Page{Key: "digest"}); st != notify.Sent {
		t.Fatalf("next day: %s", st)
	}
}

func TestStructured(t *testing.T) {
	schema := []byte(`{"type":"object","required":["disposition","findings"],"additionalProperties":false,
		"properties":{"disposition":{"enum":["merge","fix","rebuild"]},"findings":{"type":"array","items":{"type":"object","required":["severity"],"properties":{"severity":{"type":"integer","minimum":0,"maximum":3}}}}}}`)
	type verdict struct {
		Disposition string `json:"disposition"`
		Findings    []struct {
			Severity int `json:"severity"`
		} `json:"findings"`
	}
	fake := &Fake{Replies: []Response{
		{Text: `{"disposition":"ship","findings":[{"severity":7}],"extra":1}`},
		{Text: "Here you go:\n```json\n{\"disposition\":\"fix\",\"findings\":[{\"severity\":1}]}\n```"},
	}}
	var v verdict
	if err := Structured(context.Background(), fake.Complete, Request{}, schema, &v); err != nil || v.Disposition != "fix" || v.Findings[0].Severity != 1 {
		t.Fatalf("%+v, %v", v, err)
	}
	retry := fake.Requests[1].Messages[len(fake.Requests[1].Messages)-1].Content
	for _, want := range []string{"$.disposition: not one of the allowed values", "$.findings[0].severity: 7 is above the maximum 3", "$.extra: property not allowed"} {
		if !strings.Contains(retry, want) {
			t.Errorf("retry prompt lacks %q:\n%s", want, retry)
		}
	}
	bad := &Fake{Replies: []Response{{Text: `{"disposition":"merge"}`}}}
	var re *ReplyError
	if err := Structured(context.Background(), bad.Complete, Request{}, schema, &v); !errors.As(err, &re) || re.Parse || !strings.Contains(err.Error(), "findings") {
		t.Fatalf("invalid twice: %v", err)
	}
	if bad.Requests[0].Format == nil || string(bad.Requests[0].Format.Schema) != string(schema) {
		t.Fatal("Structured did not ask for structured output")
	}
	// JSON inside prose and fences is found; an empty reply is retried with a non-empty assistant turn.
	prose := &Fake{Replies: []Response{{Text: ""}, {Text: "Sure. The verdict is {\"disposition\":\"merge\",\"findings\":[]} as requested."}}}
	if err := Structured(context.Background(), prose.Complete, Request{}, schema, &v); err != nil || v.Disposition != "merge" {
		t.Fatalf("bare JSON in prose: %+v %v", v, err)
	}
	if prose.Requests[1].Messages[0].Content != "(empty reply)" {
		t.Errorf("empty reply echoed as %q", prose.Requests[1].Messages[0].Content)
	}
	junk := &Fake{Replies: []Response{{Text: "I cannot comply."}}}
	if err := Structured(context.Background(), junk.Complete, Request{}, schema, &v); !errors.As(err, &re) || !re.Parse || re.Reply != "I cannot comply." {
		t.Fatalf("unparseable twice: %v", err)
	}
}

// A gateway that rejects structured output gets the same call again without it.
func TestFormatFallback(t *testing.T) {
	var calls []bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b map[string]any
		_ = json.NewDecoder(r.Body).Decode(&b)
		calls = append(calls, b["text"] != nil)
		if b["text"] != nil {
			http.Error(w, `{"error":"unsupported parameter: text.format"}`, http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"output":[{"type":"message","content":[{"type":"output_text","text":"{}"}]}]}`))
	}))
	defer srv.Close()
	c := HTTPClient{BaseURL: srv.URL, Style: StyleResponses, Key: "k", HTTP: srv.Client()}
	r, err := c.Complete(context.Background(), Request{Format: &Format{Name: "x", Schema: json.RawMessage(`{}`)}})
	if err != nil || r.Text != "{}" || len(calls) != 2 || !calls[0] || calls[1] {
		t.Fatalf("fallback: %+v %v, calls with format %v", r, err, calls)
	}
}

// Stop reasons per wire format; a reply that stopped at the token cap is
// Truncated, and Structured reports it without retrying under the same cap.
func TestStopReasons(t *testing.T) {
	for _, tc := range []struct {
		style, body, stop string
		truncated         bool
	}{
		{StyleMessages, `{"content":[{"type":"text","text":"{\"a\":"}],"stop_reason":"max_tokens","usage":{"output_tokens":2000}}`, "max_tokens", true},
		{StyleMessages, `{"content":[{"type":"text","text":"{}"}],"stop_reason":"end_turn"}`, "end_turn", false},
		{StyleResponses, `{"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"output":[{"type":"message","content":[{"type":"output_text","text":"{"}]}]}`, "incomplete:max_output_tokens", true},
		{StyleResponses, `{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"{}"}]}]}`, "completed", false},
		{StyleChat, `{"choices":[{"message":{"content":"{\"a\""},"finish_reason":"length"}]}`, "length", true},
		{StyleChat, `{"choices":[{"message":{"content":"{}"},"finish_reason":"stop"}]}`, "stop", false},
	} {
		r, err := decode(tc.style, []byte(tc.body))
		if err != nil || r.StopReason != tc.stop || r.Truncated != tc.truncated {
			t.Errorf("%s %s: %+v %v", tc.style, tc.stop, r, err)
		}
	}
	fake := &Fake{Replies: []Response{{Text: `{"disposition": "mer`, StopReason: "max_tokens", Truncated: true}}}
	var v map[string]any
	var re *ReplyError
	err := Structured(context.Background(), fake.Complete, Request{MaxTokens: 2000}, []byte(`{"type":"object"}`), &v)
	if !errors.As(err, &re) || !re.Truncated || len(fake.Requests) != 1 || !strings.Contains(err.Error(), "2000-token cap (max_tokens)") {
		t.Fatalf("truncated reply: %v after %d call(s)", err, len(fake.Requests))
	}
}

// Anthropic's cached prompt input counts as input tokens, so cost and the
// [budget] breaker see it. The usage block has the shape the gateway returns
// for Messages calls (run 38185993917 recorded input_tokens 2 on every skill
// call); the cache counts here are illustrative, since the harness did not keep raw replies.
func TestCacheTokensCount(t *testing.T) {
	body := `{"model":"claude-opus-5-5","content":[{"type":"text","text":"{}"}],"stop_reason":"end_turn",
		"usage":{"input_tokens":2,"cache_creation_input_tokens":1548,"cache_read_input_tokens":310,"output_tokens":1696}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
	defer srv.Close()
	r, err := decode(StyleMessages, []byte(body))
	if err != nil || r.TokensIn != 1860 || r.TokensOut != 1696 {
		t.Fatalf("decode: %+v %v", r, err)
	}
	want := (1860*FallbackInputPerMTok + 1696*FallbackOutputPerMTok) / 1e6 // 0.1551; 0.1272 without the cache
	if usd, _ := Cost(nil, r.Model, r); usd < want-1e-9 || usd > want+1e-9 {
		t.Fatalf("cost %.6f, want %.6f", usd, want)
	}
	// A per-story limit between the two costs trips only when cached input counts.
	dir := t.TempDir()
	log, _ := actions.Open(filepath.Join(dir, "actions.jsonl"), actions.Options{})
	clock, _ := config.NewClock("Europe/London", "Mon-Fri 09:00-18:00")
	br := &Breaker{Budget: config.Budget{PerStoryUSD: 0.14}, Clock: clock, Now: time.Now, PauseFile: filepath.Join(dir, "paused")}
	gw := &Gateway{Resolver: Resolver{}, Actions: log, Breaker: br, Key: func(string) (string, error) { return "k", nil },
		Client: func(Route, string) Client {
			return HTTPClient{BaseURL: srv.URL, Style: StyleMessages, Key: "k", HTTP: srv.Client()}
		}}
	_, _, err = gw.Do(context.Background(), Call{Story: "#9", Lane: verbs.LaneSpec, Host: verbs.HostClaude, Req: Request{Messages: []Message{{Role: "user", Content: "draft"}}}})
	if !errors.Is(err, ErrBudget) || !br.Paused() {
		t.Fatalf("breaker did not trip on 0.155 USD against 0.14: %v", err)
	}
}
