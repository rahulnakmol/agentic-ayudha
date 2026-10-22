package models

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"testing/iotest"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/notify"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

func TestLaneResolution(t *testing.T) {
	cfg := config.Models{Lanes: map[string]map[string]config.ModelChoice{
		"build":  {"claude": {Model: verbs.ModelClaudeSonnet}},
		"review": {"cursor": {Model: verbs.ModelGPTSol}, "claude": {Model: "my-model", Family: "xai"}, "copilot": {Model: "claude-haiku-5.5"}},
		"chore":  {"copilot": {Model: "claude-haiku-5.5"}},
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
		// Cursor's x plan, the default, includes only Cursor's own models
		// (Decision #469).
		{verbs.LaneBuild, verbs.HostCursor, false, false, verbs.ModelGrok, verbs.FamilyXAI, SourceDefault, ""},
		{verbs.LaneChore, verbs.HostCursor, false, false, verbs.ModelComposer, "", SourceDefault, ""},
		{verbs.LaneReview, verbs.HostAmp, false, false, verbs.ModelGPTSol, verbs.FamilyOpenAI, SourceDefault, ""},
		{verbs.LaneUIReview, verbs.HostCursor, false, false, verbs.ModelGrokPrior, verbs.FamilyXAI, SourceDefault, ""},
		{verbs.LaneChore, verbs.HostClaude, false, false, verbs.ModelClaudeHaiku, verbs.FamilyAnthropic, SourceDefault, ""},
		// Config overrides.
		{verbs.LaneBuild, verbs.HostClaude, false, false, verbs.ModelClaudeSonnet, verbs.FamilyAnthropic, SourceConfig, ""},
		{verbs.LaneReview, verbs.HostClaude, false, false, "my-model", verbs.FamilyXAI, SourceConfig, ""},
		// Cursor cannot run GPT: the configured GPT lane resolves to the Grok default.
		{verbs.LaneReview, verbs.HostCursor, false, false, verbs.ModelGrokPrior, verbs.FamilyXAI, SourceFallback, ""},
		// The binary's own calls cannot use a host-managed mode.
		{verbs.LaneChore, verbs.HostAmp, true, false, verbs.ModelClaudeHaiku, verbs.FamilyAnthropic, SourceDirect, ""},
		{verbs.LaneUIReview, verbs.HostAmp, true, false, verbs.ModelGemini, verbs.FamilyGoogle, SourceDirect, "high"},
		// Copilot's rows, defaults and a person's, name models in Copilot's
		// catalog, which the gateway does not route: direct calls use claude's row.
		{verbs.LaneReview, verbs.HostCopilot, false, false, "claude-haiku-5.5", verbs.FamilyAnthropic, SourceConfig, ""},
		{verbs.LaneReview, verbs.HostCopilot, true, false, "my-model", verbs.FamilyXAI, SourceDirect, ""},
		{verbs.LaneChore, verbs.HostCopilot, true, false, verbs.ModelClaudeHaiku, verbs.FamilyAnthropic, SourceDirect, ""},
		// UI review on Copilot defaults to Gemini 4 Argon, which the binary
		// calls through Copilot CLI (CopilotRouted), never the gateway.
		{verbs.LaneUIReview, verbs.HostCopilot, false, false, verbs.CopilotGeminiArgon, verbs.FamilyGoogle, SourceDefault, "high"},
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
	if !strings.Contains(joined, "gpt-6.1-sol (openai) is not available on Cursor; using the lane's Cursor default grok-4.6") {
		t.Errorf("substitution not logged; log:\n%s", joined)
	}
	// A legacy Cursor plan keeps the defaults it includes: Claude, Grok 4.7,
	// Gemini, and Haiku.
	legacy := Resolver{Hosts: config.Hosts{Cursor: config.CursorHost{Plan: verbs.PlanLegacy}}}
	for lane, want := range map[verbs.LaneID]string{verbs.LaneBuild: verbs.ModelClaudeOpus, verbs.LaneReview: verbs.ModelGrok, verbs.LaneUIReview: verbs.ModelGemini, verbs.LaneChore: verbs.ModelClaudeHaiku} {
		if got, err := legacy.Resolve(lane, verbs.HostCursor); err != nil || got.Model != want {
			t.Errorf("%s on a legacy Cursor plan = %+v, %v; want %s", lane, got, err, want)
		}
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
		{"google via gateway: chat", gw, verbs.ModelGemini, "gateway", StyleChat, "ai-gateway"},
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
	// Fast serving (inspect's paid review asks for it): service_tier on
	// Responses and Chat; Messages has no priority tier, so never on it.
	req := Request{System: "be brief", Messages: []Message{{Role: "user", Content: "hi"}}, MaxTokens: 50, Effort: "high", ServiceTier: TierPriority}
	tests := []struct {
		style, path string
		in, out     int
		cost        float64
		check       func(map[string]any) bool
	}{
		{StyleMessages, "/v1/messages", 11, 3, 0, func(b map[string]any) bool {
			oc, _ := b["output_config"].(map[string]any)
			return b["system"] == "be brief" && b["max_tokens"] == 50.0 && oc["effort"] == "high" && b["service_tier"] == nil
		}},
		{StyleResponses, "/v1/responses", 12, 4, 0, func(b map[string]any) bool {
			return b["instructions"] == "be brief" && b["reasoning"].(map[string]any)["effort"] == "high" && b["service_tier"] == "priority"
		}},
		{StyleChat, "/v1/chat/completions", 13, 5, 0.0042, func(b map[string]any) bool {
			msgs := b["messages"].([]any)
			return msgs[0].(map[string]any)["role"] == "system" && b["reasoning_effort"] == "high" && b["service_tier"] == "priority"
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
	// Prompt caching: a CacheBreak message ends with an Anthropic cache_control
	// breakpoint and the others stay plain; OpenAI Responses gets the cache key
	// instead; Chat sends neither.
	creq := req
	creq.CacheKey = "aayu-eval-survey/x"
	creq.Messages = []Message{{Role: "user", Content: "prefix", CacheBreak: true}, {Role: "assistant", Content: "draft", CacheBreak: true}, {Role: "user", Content: "check"}}
	for style, want := range map[string]string{
		StyleMessages:  `[{"content":[{"cache_control":{"type":"ephemeral"},"text":"prefix","type":"text"}],"role":"user"},{"content":[{"cache_control":{"type":"ephemeral"},"text":"draft","type":"text"}],"role":"assistant"},{"content":"check","role":"user"}]`,
		StyleResponses: `[{"content":"prefix","role":"user"},{"content":"draft","role":"assistant"},{"content":"check","role":"user"}]`,
		StyleChat:      `[{"content":"be brief","role":"system"},{"content":"prefix","role":"user"},{"content":"draft","role":"assistant"},{"content":"check","role":"user"}]`,
	} {
		c := HTTPClient{BaseURL: srv.URL + "/v1", Style: style, Key: "k", HTTP: srv.Client()}
		if _, err := c.Complete(context.Background(), creq); err != nil {
			t.Fatal(err)
		}
		key := "messages"
		if style == StyleResponses {
			key = "input"
		}
		if b, _ := json.Marshal(got.body[key]); string(b) != want || (got.body["prompt_cache_key"] == creq.CacheKey) != (style == StyleResponses) {
			t.Errorf("%s cache: %s key %v\nwant %s", style, b, got.body["prompt_cache_key"], want)
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
	t.Run("going over the limit trips once and pages once", func(t *testing.T) {
		gw, fake, sink, br := setup(t, config.Budget{PerStoryUSD: 1.00}, 0.25)
		var errs []error
		for i := 0; i < 7; i++ {
			_, _, err := gw.Do(context.Background(), call)
			errs = append(errs, err)
		}
		// Exactly 1.00 is still within the limit, as it is for build's cost_usd.
		for i, err := range errs {
			if wantTrip := i >= 4; errors.Is(err, ErrBudget) != wantTrip {
				t.Fatalf("call %d: err %v, want tripped = %v", i+1, err, wantTrip)
			}
		}
		if len(fake.Requests) != 5 {
			t.Fatalf("%d provider calls, want 5 (the 5th goes over 1.00; later calls are refused before the provider)", len(fake.Requests))
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
		// Windows reports a path under a file as not found (ERROR_PATH_NOT_FOUND), which is a missing log there.
		if err := br.SeedLog(filepath.Join(bad, "actions.jsonl")); err == nil && runtime.GOOS != "windows" {
			t.Fatal("a log under a file: want an error")
		}
		if err := br.SeedLog(dir); err == nil { // opens, then fails on the first read
			t.Fatal("a directory as the log: want an error")
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
	// ValidReply makes Structured's check without a retry: nil without a
	// format or for a valid fenced reply, else why the reply fails.
	f := &Format{Schema: schema}
	if ValidReply("anything", nil) != nil || ValidReply("```json\n{\"disposition\":\"fix\",\"findings\":[]}\n```", f) != nil {
		t.Error("ValidReply rejected a valid reply")
	}
	if err := ValidReply(`{"disposition":"merge"}`, f); err == nil || !strings.Contains(err.Error(), "findings") {
		t.Errorf("ValidReply on a reply without findings: %v", err)
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

// A 503 or a 429 carries its Retry-After, in seconds or as an HTTP date, so a
// caller can wait as long as the endpoint asked; a 502 without one has none.
func TestStatusErrorRetryAfter(t *testing.T) {
	for _, tc := range []struct {
		code     int
		header   string
		min, max time.Duration
	}{
		{503, "7", 7 * time.Second, 7 * time.Second},
		{429, time.Now().Add(90 * time.Second).UTC().Format(http.TimeFormat), 85 * time.Second, 90 * time.Second},
		{502, "", 0, 0},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if tc.header != "" {
				w.Header().Set("Retry-After", tc.header)
			}
			http.Error(w, "error code: "+strconv.Itoa(tc.code), tc.code)
		}))
		_, err := HTTPClient{BaseURL: srv.URL, Style: StyleMessages, Key: "k", HTTP: srv.Client()}.Complete(context.Background(), Request{})
		srv.Close()
		var se *StatusError
		if !errors.As(err, &se) || se.Code != tc.code || se.RetryAfter < tc.min || se.RetryAfter > tc.max {
			t.Errorf("%d with Retry-After %q: %v (%+v), want %v-%v", tc.code, tc.header, err, se, tc.min, tc.max)
		}
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

// A reply that spent its whole cap on thinking has no text: it is a truncated
// reply (a ReplyError a caller can rerun at a larger cap), while one that ended
// without text is a plain error.
func TestNoTextAtCap(t *testing.T) {
	for _, tc := range []struct {
		stop      string
		truncated bool
	}{{"max_tokens", true}, {"end_turn", false}} {
		sse := `data: {"type":"message_start","message":{"model":"m","usage":{"input_tokens":5,"output_tokens":1}}}` + "\n\n" +
			`data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}` + "\n\n" +
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"weighing the build files"}}` + "\n\n" +
			`data: {"type":"message_delta","delta":{"stop_reason":"` + tc.stop + `"},"usage":{"output_tokens":100}}` + "\n\n" +
			`data: {"type":"message_stop"}` + "\n\n"
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte(sse))
		}))
		c := HTTPClient{BaseURL: srv.URL, Style: StyleMessages, Key: "k", HTTP: srv.Client()}
		r, err := c.Complete(context.Background(), Request{Model: "m", MaxTokens: 100, Stream: true})
		srv.Close()
		var re *ReplyError
		if err == nil || errors.As(err, &re) != tc.truncated || tc.truncated && !re.Truncated || r.TokensOut != 100 {
			t.Errorf("%s: want truncated=%v, got %+v %v", tc.stop, tc.truncated, r, err)
		}
	}
}

// A streamed Messages reply decodes to the same Response as the plain JSON
// one: text deltas in order, cached input counted, the final output tokens
// and stop reason from message_delta. A gateway that ignores "stream" and
// answers JSON still decodes; an error event is an error, returned with the
// usage so far.
func TestMessagesStream(t *testing.T) {
	sse := "event: message_start\n" +
		`data: {"type":"message_start","message":{"model":"claude-opus-5-5","usage":{"input_tokens":5,"cache_read_input_tokens":100,"cache_creation_input_tokens":300,"cache_creation":{"ephemeral_5m_input_tokens":100,"ephemeral_1h_input_tokens":200},"output_tokens":1}}}` + "\n\n" +
		"event: ping\ndata: {\"type\":\"ping\"}\n\n" +
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}` + "\n\n" +
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"{\"a\":"}}` + "\n\n" +
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" 1}"}}` + "\n\n" +
		`data: {"type":"message_delta","delta":{"stop_reason":"max_tokens"},"usage":{"output_tokens":42}}` + "\n\n" +
		`data: {"type":"message_stop"}` + "\n\n"
	for _, tc := range []struct {
		name, ctype, body, text string
		in, out                 int
		truncated, wantErr      bool
	}{
		{"stream", "text/event-stream; charset=utf-8", sse, `{"a": 1}`, 405, 42, true, false},
		{"json", "application/json", `{"model":"m","content":[{"type":"text","text":"{}"}],"stop_reason":"end_turn","usage":{"input_tokens":3,"output_tokens":4}}`, "{}", 3, 4, false, false},
		// An error after message_start keeps the usage so far: the attempt may be billed.
		{"error", "text/event-stream", sse[:strings.Index(sse, "event: ping")] + `data: {"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}` + "\n\n", "", 405, 1, false, true},
	} {
		var sent map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&sent)
			w.Header().Set("Content-Type", tc.ctype)
			_, _ = w.Write([]byte(tc.body))
		}))
		c := HTTPClient{BaseURL: srv.URL, Style: StyleMessages, Key: "k", HTTP: srv.Client()}
		r, err := c.Complete(context.Background(), Request{Model: "m", MaxTokens: 100, Stream: true})
		srv.Close()
		if sent["stream"] != true {
			t.Errorf("%s: request did not ask to stream: %v", tc.name, sent)
		}
		if tc.wantErr {
			if err == nil || !strings.Contains(err.Error(), "overloaded_error") || r.TokensIn != tc.in || r.TokensOut != tc.out || r.CachedIn != 100 {
				t.Errorf("%s: want the stream's error with its usage so far, got %+v %v", tc.name, r, err)
			}
			continue
		}
		if err != nil || r.Text != tc.text || r.TokensIn != tc.in || r.TokensOut != tc.out || r.Truncated != tc.truncated {
			t.Errorf("%s: %+v %v", tc.name, r, err)
		}
		// A streamed reply's cache reads and writes bill at their own rates too (the survey,
		// session, and ux evals stream): Opus 5.5 input 4, read 0.20, 5-minute write 5, 1-hour write 8.
		if tc.name == "stream" {
			want := (5*4 + 100*0.20 + 100*5 + 200*8 + 42*20) / 1e6
			if usd, _ := Cost(nil, "claude-opus-5-5", r); math.Abs(usd-want) > 1e-12 {
				t.Errorf("stream cost %.6f, want %.6f", usd, want)
			}
		}
	}
}

// A stream cut off inside a data line, or before message_stop, is a dropped
// connection, not a reply: the read error (or an unexpected EOF when the
// stream just ends) comes back, with the usage so far, so the caller retries it.
func TestMessagesStreamCutInsideAnEvent(t *testing.T) {
	start := `data: {"type":"message_start","message":{"model":"m","usage":{"input_tokens":7}}}` + "\n\n"
	partial := start + `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_d`
	for name, tc := range map[string]struct {
		body io.Reader
		want error
	}{
		"reset": {io.MultiReader(strings.NewReader(partial), iotest.ErrReader(syscall.ECONNRESET)), syscall.ECONNRESET},
		"ended": {strings.NewReader(partial), io.ErrUnexpectedEOF},
		// A prefix that ends between events, before message_stop, even one that is valid JSON so far.
		"no stop": {strings.NewReader(start + `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"{}"}}` + "\n\n"), io.ErrUnexpectedEOF},
	} {
		r, err := decodeMessagesStream(tc.body)
		if !errors.Is(err, tc.want) || r.TokensIn != 7 {
			t.Errorf("%s: %+v %v, want %v with the usage so far", name, r, err, tc.want)
		}
	}
	if r, err := decodeMessagesStream(strings.NewReader(start + "data: {not json}\n\ndata: {\"type\":\"message_stop\"}\n\n")); err == nil || !strings.Contains(err.Error(), "decode messages stream") || r.TokensIn != 7 {
		t.Errorf("a malformed event mid-stream is still a decode error, with the usage so far: %+v %v", r, err)
	}
}

// Anthropic's cached prompt input counts as input tokens, so cost and the
// [budget] breaker see it, each at its own rate. The usage block has the shape the gateway returns
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
	// Opus 5.5's list price: input 4, cache write 5, cache read 0.20, output 20 USD per million tokens.
	want := (2*4 + 1548*5 + 310*0.20 + 1696*20) / 1e6 // 0.04173; 0.033928 without the cache
	if usd, src := Cost(nil, r.Model, r); usd < want-1e-9 || usd > want+1e-9 || src != CostListPrice {
		t.Fatalf("cost %.6f (%s), want %.6f", usd, src, want)
	}
	// A per-story limit between the two costs trips only when cached input counts.
	dir := t.TempDir()
	log, _ := actions.Open(filepath.Join(dir, "actions.jsonl"), actions.Options{})
	clock, _ := config.NewClock("Europe/London", "Mon-Fri 09:00-18:00")
	br := &Breaker{Budget: config.Budget{PerStoryUSD: 0.04}, Clock: clock, Now: time.Now, PauseFile: filepath.Join(dir, "paused")}
	gw := &Gateway{Resolver: Resolver{}, Actions: log, Breaker: br, Key: func(string) (string, error) { return "k", nil },
		Client: func(Route, string) Client {
			return HTTPClient{BaseURL: srv.URL, Style: StyleMessages, Key: "k", HTTP: srv.Client()}
		}}
	_, _, err = gw.Do(context.Background(), Call{Story: "#9", Lane: verbs.LaneSpec, Host: verbs.HostClaude, Req: Request{Messages: []Message{{Role: "user", Content: "draft"}}}})
	if !errors.Is(err, ErrBudget) || !br.Paused() {
		t.Fatalf("breaker did not trip on 0.0417 USD against 0.04: %v", err)
	}
}

// A call that fails after using tokens (a stream that errored after
// message_start) is charged too: the action ledger records its cost and the
// breaker counts it, and the error stays the provider's.
func TestGatewayChargesAFailedCall(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "actions.jsonl")
	log, _ := actions.Open(path, actions.Options{})
	clock, _ := config.NewClock("Europe/London", "Mon-Fri 09:00-18:00")
	br := &Breaker{Budget: config.Budget{PerStoryUSD: 0.01}, Clock: clock, Now: time.Now, PauseFile: filepath.Join(dir, "paused")}
	gw := &Gateway{Resolver: Resolver{}, Actions: log, Breaker: br, Key: func(string) (string, error) { return "k", nil },
		Client: func(Route, string) Client {
			return failing{Response{Model: "claude-opus-5-5", TokensIn: 5000}, &StreamError{Type: "overloaded_error", Message: "Overloaded"}}
		}}
	_, _, err := gw.Do(context.Background(), Call{Story: "#9", Lane: verbs.LaneSpec, Host: verbs.HostClaude, Req: Request{Messages: []Message{{Role: "user", Content: "draft"}}}})
	var st *StreamError
	if !errors.As(err, &st) || !errors.Is(err, ErrBudget) || !br.Paused() {
		t.Fatalf("5000 input tokens at 4 USD per million (0.02) against 0.01: %v, paused %v", err, br.Paused())
	}
	b, _ := os.ReadFile(path)
	var rec struct {
		Status  string  `json:"status"`
		CostUSD float64 `json:"cost_usd"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(b))), &rec); err != nil || rec.Status != actions.StatusFailed || math.Abs(rec.CostUSD-0.02) > 1e-9 {
		t.Fatalf("ledger record %s: %+v %v", b, rec, err)
	}
}

// failing returns its partial response with its error.
type failing struct {
	resp Response
	err  error
}

func (f failing) Complete(context.Context, Request) (Response, error) { return f.resp, f.err }

// Each provider's usage block is priced at that model's list rates: OpenAI's
// implicit cache writes at 1.25x input and reads at its cached rate, xAI's
// reasoning tokens (in total_tokens, not completion_tokens) as output, a long
// prompt at the long-context rates for the whole call, an unknown model at the
// fallback rate, and a [[models.prices]] entry over the list price.
func TestCostAtListPrices(t *testing.T) {
	cases := []struct {
		name, style, body string
		prices            []config.Price
		want              float64
		source            string
	}{
		{"gpt-6.1-sol, cache write and read", StyleResponses,
			`{"model":"gpt-6.1-sol","status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"ok"}]}],
			"usage":{"input_tokens":15000,"input_tokens_details":{"cached_tokens":12000,"cache_write_tokens":3000},"output_tokens":900,"total_tokens":15900}}`,
			nil, (12000*0.10 + 3000*2.50 + 900*10) / 1e6, CostListPrice},
		// Anthropic splits cache writes by TTL; the gateway's 1-hour writes bill at 2x input (8), the
		// 5-minute ones at 1.25x (5). Charging all 1,548 at 5 would give 0.04173.
		{"claude-opus-5-5, 5-minute and 1-hour cache writes", StyleMessages,
			`{"model":"claude-opus-5-5","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":2,
			"cache_creation_input_tokens":1548,"cache_read_input_tokens":310,"output_tokens":1696,
			"cache_creation":{"ephemeral_5m_input_tokens":548,"ephemeral_1h_input_tokens":1000}}}`,
			nil, (2*4 + 548*5 + 1000*8 + 310*0.20 + 1696*20) / 1e6, CostListPrice},
		{"claude-haiku-5-5, long prompt with 1-hour writes", StyleMessages,
			`{"model":"claude-haiku-5-5","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":90000,
			"cache_creation_input_tokens":20000,"output_tokens":1000,"cache_creation":{"ephemeral_1h_input_tokens":20000}}}`,
			nil, (90000*0.50 + 20000*1 + 1000*2.50) / 1e6, CostListPrice},
		{"config price with one cache-write rate bills 1-hour writes at it", StyleMessages,
			`{"model":"claude-opus-5-5","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":10,
			"cache_creation_input_tokens":1000,"output_tokens":100,"cache_creation":{"ephemeral_1h_input_tokens":1000}}}`,
			[]config.Price{{Model: "claude-opus-5-5", InputPerMTok: 3, CacheWritePerMTok: 6, OutputPerMTok: 12}}, (10*3 + 1000*6 + 100*12) / 1e6, CostPriceTable},
		{"grok-4.7, reasoning outside completion_tokens", StyleChat,
			`{"model":"grok-4.7","choices":[{"message":{"content":"ok"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":4000,"prompt_tokens_details":{"cached_tokens":1000},"completion_tokens":300,"total_tokens":5300}}`,
			nil, (3000*2 + 1000*0.50 + 1300*6) / 1e6, CostListPrice},
		{"grok-4.7 served at priority: 2x every rate, the cached one included", StyleChat,
			`{"model":"grok-4.7","service_tier":"priority","choices":[{"message":{"content":"ok"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":4000,"prompt_tokens_details":{"cached_tokens":1000},"completion_tokens":300,"total_tokens":5300}}`,
			nil, (3000*4 + 1000*1 + 1300*12) / 1e6, CostListPrice},
		// Gemini through the gateway's chat endpoint: thinking at high effort is in total_tokens (and
		// completion_tokens_details), not completion_tokens; it bills as output.
		{"gemini-3.8-flash, high-effort reasoning tokens", StyleChat,
			`{"model":"gemini-3.8-flash","choices":[{"message":{"content":"ok"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":3917,"completion_tokens":1200,"completion_tokens_details":{"reasoning_tokens":4845},"total_tokens":9962}}`,
			nil, (3917*0.75 + 6045*3.75) / 1e6, CostListPrice},
		{"claude-haiku-5-5, long prompt", StyleMessages,
			`{"model":"claude-haiku-5-5","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":100001,"output_tokens":1000}}`,
			nil, (100001*0.50 + 1000*2.50) / 1e6, CostListPrice},
		{"unknown model", StyleMessages,
			`{"model":"acme-1","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":1000,"output_tokens":100}}`,
			nil, (1000*FallbackInputPerMTok + 100*FallbackOutputPerMTok) / 1e6, CostFallback},
		{"a reported cost below the priority rate is floored at it", StyleChat,
			`{"model":"grok-4.7","service_tier":"priority","choices":[{"message":{"content":"ok"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":1000,"completion_tokens":100,"total_tokens":1100,"cost":0.000001}}`,
			nil, (1000*4 + 100*12) / 1e6, CostListPrice},
		{"a configured grok-4.7 rate keeps the priority multiplier", StyleChat,
			`{"model":"grok-4.7","service_tier":"priority","choices":[{"message":{"content":"ok"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":1000,"completion_tokens":100,"total_tokens":1100}}`,
			[]config.Price{{Model: "grok-4.7", InputPerMTok: 3, OutputPerMTok: 9}}, (1000*3 + 100*9) * 2 / 1e6, CostPriceTable},
		{"config overrides the list price", StyleResponses,
			`{"model":"gpt-6.1-sol","status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"ok"}]}],
			"usage":{"input_tokens":1000,"input_tokens_details":{"cached_tokens":400},"output_tokens":100}}`,
			[]config.Price{{Model: "gpt-6.1-sol", InputPerMTok: 3, OutputPerMTok: 12}}, (1000*3 + 100*12) / 1e6, CostPriceTable},
	}
	for _, c := range cases {
		r, err := decode(c.style, []byte(c.body))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if usd, src := Cost(c.prices, r.Model, r); math.Abs(usd-c.want) > 1e-12 || src != c.source {
			t.Errorf("%s: %.6f USD (%s), want %.6f (%s)", c.name, usd, src, c.want, c.source)
		}
	}
}

// TestUIReviewLaneEffort: a ui-review call that sets no effort goes out at the
// lane's (high for gemini-3.8-flash), on the chat route Gemini uses and on a
// Responses route a gateway may map it to.
func TestUIReviewLaneEffort(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = nil
		_ = json.Unmarshal(b, &body)
		if r.URL.Path == "/v1/responses" {
			_, _ = w.Write([]byte(`{"model":"gemini-3.8-flash","status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":1,"output_tokens":1}}`))
			return
		}
		_, _ = w.Write([]byte(`{"model":"gemini-3.8-flash","choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`))
	}))
	defer srv.Close()
	log, _ := actions.Open(filepath.Join(t.TempDir(), "actions.jsonl"), actions.Options{})
	for _, styles := range [][]string{nil, {"google=responses"}} {
		m := config.Models{Gateway: config.Gateway{BaseURL: srv.URL + "/v1", Secret: "ai-gateway", Styles: styles}}
		gw := &Gateway{Resolver: Resolver{Models: m}, Models: m, Actions: log, Key: func(string) (string, error) { return "k", nil }}
		_, res, err := gw.Do(context.Background(), Call{Story: "#1", Lane: verbs.LaneUIReview, Host: verbs.HostClaude,
			Req: Request{Messages: []Message{{Role: "user", Content: "critique"}}, MaxTokens: 10}})
		if err != nil || res.Model != verbs.ModelGemini || res.Effort != "high" {
			t.Fatalf("%v: resolved %+v, %v", styles, res, err)
		}
		effort := body["reasoning_effort"]
		if r, ok := body["reasoning"].(map[string]any); ok {
			effort = r["effort"]
		}
		if effort != "high" {
			t.Errorf("%v: the request carries effort %v, want high: %v", styles, effort, body)
		}
	}
}

// On Copilot, [models.<lane>.copilot] may name Auto, an Auto tier, or
// HydraFusion, which resolve as host-managed; a misspelt one is an error.
func TestCopilotSelectors(t *testing.T) {
	for model, ok := range map[string]bool{"auto": true, "auto:balance": true, "hydrafusion": true, "auto:balanced": false, "hydrafusion-extra": false} {
		r := Resolver{Models: config.Models{Lanes: map[string]map[string]config.ModelChoice{"review": {"copilot": {Model: model}}}}}
		res, err := r.Resolve(verbs.LaneReview, verbs.HostCopilot)
		if ok != (err == nil) || ok && res.Family != verbs.FamilyHostManaged {
			t.Errorf("%s: %+v %v", model, res, err)
		}
	}
}

// TestStreamPastAProxyIdleLimit (Bug #608): behind a proxy that answers 524
// when no byte arrives within its limit, a slow reply over Chat Completions
// or Responses gets through when it streams, decodes to what the plain JSON
// reply decodes to, and a plain JSON answer to a streamed request still
// decodes; the 524 itself says the reply timed out.
func TestStreamPastAProxyIdleLimit(t *testing.T) {
	const reply = `{"findings":[{"id":"a","severity":"P1"}]}`
	chatJSON := `{"model":"grok-4.7","service_tier":"priority","choices":[{"message":{"content":` + strconv.Quote(reply) + `},"finish_reason":"stop"}],"usage":{"prompt_tokens":11,"completion_tokens":7,"total_tokens":30}}`
	respJSON := `{"model":"grok-4.7","service_tier":"priority","status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":` + strconv.Quote(reply) + `}]}],"usage":{"input_tokens":11,"output_tokens":7}}`
	half := len(reply) / 2
	events := map[string][]string{
		StyleChat: {
			`{"model":"grok-4.7","service_tier":"priority","choices":[{"delta":{"role":"assistant","content":""}}]}`,
			`: xai-usage {"prompt_tokens":11}`,
			`{"model":"grok-4.7","choices":[{"delta":{"reasoning_content":"weighing the diff"}}]}`,
			`{"model":"grok-4.7","choices":[{"delta":{"content":` + strconv.Quote(reply[:half]) + `}}]}`,
			`{"model":"grok-4.7","choices":[{"delta":{"content":` + strconv.Quote(reply[half:]) + `},"finish_reason":"stop"}]}`,
			`{"model":"grok-4.7","choices":[],"usage":{"prompt_tokens":11,"completion_tokens":7,"total_tokens":30}}`,
			`[DONE]`},
		StyleResponses: {
			`{"type":"response.created","response":{"status":"in_progress"}}`,
			`: xai-usage {"input_tokens":11}`,
			`{"type":"response.reasoning_summary_text.delta","delta":"weighing the diff"}`,
			`{"type":"response.output_text.delta","delta":` + strconv.Quote(reply[:half]) + `}`,
			`{"type":"response.output_text.delta","delta":` + strconv.Quote(reply[half:]) + `}`,
			`{"type":"response.completed","response":` + respJSON + `}`},
	}
	plain := map[string]string{StyleChat: chatJSON, StyleResponses: respJSON}
	const slow, limit = 400 * time.Millisecond, 150 * time.Millisecond
	for _, style := range []string{StyleChat, StyleResponses} {
		// Each request runs in its own server goroutine (and a 524'd one keeps sleeping),
		// so what the handler shares with the test is guarded.
		var mu sync.Mutex
		var asked map[string]any
		var jsonOnly atomic.Bool
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			mu.Lock()
			asked = body
			mu.Unlock()
			if body["stream"] != true || jsonOnly.Load() {
				time.Sleep(slow) // a long generation, silent until done
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, plain[style])
				return
			}
			w.Header().Set("Content-Type", "text/event-stream")
			for _, ev := range events[style] {
				if strings.HasPrefix(ev, ":") { // an SSE comment, as the gateway forwards xAI's
					_, _ = io.WriteString(w, ev+"\n\n")
				} else {
					_, _ = io.WriteString(w, "data: "+ev+"\n\n")
				}
				w.(http.Flusher).Flush()
				time.Sleep(slow / time.Duration(len(events[style])))
			}
		}))
		// The proxy gives up when the upstream sends nothing within its limit.
		proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			req, _ := http.NewRequestWithContext(r.Context(), r.Method, upstream.URL+r.URL.Path, r.Body)
			req.Header = r.Header.Clone()
			resp, err := (&http.Transport{ResponseHeaderTimeout: limit}).RoundTrip(req)
			if err != nil {
				w.WriteHeader(524)
				_, _ = io.WriteString(w, "error code: 524")
				return
			}
			defer func() { _ = resp.Body.Close() }()
			w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
			buf := make([]byte, 256)
			for {
				n, err := resp.Body.Read(buf)
				_, _ = w.Write(buf[:n])
				w.(http.Flusher).Flush()
				if err != nil {
					return
				}
			}
		}))
		ctx := context.Background()
		req := Request{Model: "m", MaxTokens: 100, Messages: []Message{{Role: "user", Content: "review"}}}
		direct := HTTPClient{BaseURL: upstream.URL, Style: style, Key: "k"}
		want, err := direct.Complete(ctx, req)
		if err != nil {
			t.Fatalf("%s: the plain reply: %v", style, err)
		}
		via := HTTPClient{BaseURL: proxy.URL, Style: style, Key: "k"}
		_, err = via.Complete(ctx, req)
		var se *StatusError
		if !errors.As(err, &se) || se.Code != 524 || !strings.Contains(err.Error(), "a timeout waiting for the reply") {
			t.Errorf("%s: an unstreamed slow reply: %v, want a 524 that says it timed out", style, err)
		}
		req.Stream = true
		got, err := via.Complete(ctx, req)
		if err != nil || got.Text != want.Text || got.Text != reply || got.TokensIn != want.TokensIn || got.TokensOut != want.TokensOut || got.Model != want.Model || got.StopReason != want.StopReason || got.ServiceTier != "priority" || want.ServiceTier != "priority" {
			t.Errorf("%s: streamed %+v %v, want %+v", style, got, err, want)
		}
		mu.Lock()
		last := asked
		mu.Unlock()
		if style == StyleChat && last["stream_options"] == nil {
			t.Errorf("chat: the request did not ask for the usage chunk: %v", last)
		}
		jsonOnly.Store(true) // an endpoint that ignores stream: the JSON still decodes
		if got, err := direct.Complete(ctx, req); err != nil || got.Text != reply {
			t.Errorf("%s: a JSON answer to a streamed request: %+v %v", style, got, err)
		}
		proxy.Close()
		upstream.Close()
	}
	// A stream cut off before its end is a dropped connection, not a reply.
	if _, err := decodeChatStream(strings.NewReader("data: " + events[StyleChat][3] + "\n\n")); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("a cut chat stream: %v", err)
	}
	if _, err := decodeResponsesStream(strings.NewReader("data: " + events[StyleResponses][3] + "\n\n")); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("a cut responses stream: %v", err)
	}
	// Cut between the finish reason and the usage chunk: the paid call's
	// usage never arrived, so it is not a reply.
	sse := func(evs ...string) io.Reader {
		var b strings.Builder
		for _, e := range evs {
			b.WriteString("data: " + e + "\n\n")
		}
		return strings.NewReader(b.String())
	}
	if _, err := decodeChatStream(sse(events[StyleChat][0], events[StyleChat][3], events[StyleChat][4])); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("a chat stream cut before its usage: %v", err)
	}
	// Cut inside an event: a dropped connection, so it stays retryable.
	if _, err := decodeChatStream(strings.NewReader("data: " + events[StyleChat][3] + "\n\ndata: {\"model\":\"gro")); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("a chat stream cut inside an event: %v", err)
	}
	if _, err := decodeResponsesStream(strings.NewReader("data: {\"type\":\"response.out")); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("a responses stream cut inside an event: %v", err)
	}
	// A malformed event with more after it is a bad reply, not a lost delta.
	if _, err := decodeChatStream(sse(events[StyleChat][3], `{bad`, events[StyleChat][5], "[DONE]")); err == nil || !strings.Contains(err.Error(), "decode chat stream") {
		t.Errorf("a malformed chat event: %v", err)
	}
	// An error after usage arrived comes back with that usage: the attempt may be billed.
	r, err := decodeChatStream(sse(events[StyleChat][5], `{"error":{"type":"overloaded","message":"busy"}}`))
	if err == nil || !strings.Contains(err.Error(), "stream error: overloaded") || r.TokensIn != 11 || r.TokensOut != 19 || r.Model != "grok-4.7" {
		t.Errorf("a chat stream error: %+v %v", r, err)
	}
	r, err = decodeResponsesStream(sse(`{"type":"response.failed","response":{"model":"gpt-5.6","status":"failed","error":{"code":"server_error","message":"boom"},"usage":{"input_tokens":11,"output_tokens":3}}}`))
	if err == nil || !strings.Contains(err.Error(), "server_error") || r.TokensIn != 11 || r.TokensOut != 3 {
		t.Errorf("a failed responses stream: %+v %v", r, err)
	}
}

// A caller's own HTTP client with a shorter timeout (the CLI's general
// 5-minute client) does not cut a model call before CallTimeout: the merge
// gate's CI review streamed past 5 minutes and was cut on every PR (#674, #682, #666).
func TestCallTimeoutOutlastsTheCallersClient(t *testing.T) {
	defer func(d time.Duration) { CallTimeout = d }(CallTimeout)
	CallTimeout = 5 * time.Second
	events := []string{
		`{"model":"grok-4.7","choices":[{"delta":{"role":"assistant","content":""}}]}`,
		`{"model":"grok-4.7","choices":[{"delta":{"content":"done"},"finish_reason":"stop"}]}`,
		`{"model":"grok-4.7","choices":[],"usage":{"prompt_tokens":11,"completion_tokens":7,"total_tokens":18}}`,
		`[DONE]`}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, ev := range events {
			_, _ = io.WriteString(w, "data: "+ev+"\n\n")
			w.(http.Flusher).Flush()
			time.Sleep(150 * time.Millisecond) // 600 ms in all, past the caller's 200 ms
		}
	}))
	defer srv.Close()
	c := HTTPClient{BaseURL: srv.URL, Style: StyleChat, Key: "k", HTTP: &http.Client{Timeout: 200 * time.Millisecond}}
	resp, err := c.Complete(context.Background(), Request{Model: "grok-4.7", MaxTokens: 100, Stream: true, Messages: []Message{{Role: "user", Content: "review"}}})
	if err != nil || resp.Text != "done" {
		t.Fatalf("a stream past the caller's client timeout: %q %v", resp.Text, err)
	}
}
