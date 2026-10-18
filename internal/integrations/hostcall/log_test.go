package hostcall

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var at = time.Date(2026, 10, 17, 9, 30, 0, 0, time.UTC)

func TestNewEntry(t *testing.T) {
	for _, tt := range []struct {
		name    string
		e       Event
		out     string
		failed  bool
		window  string
		limit   int
		rows    int
		citable bool
		why     string
	}{
		{"sentry bounded read", Event{Tool: "mcp__sentry__find_releases", Input: map[string]any{"statsPeriod": "7d", "limit": float64(20)}},
			`[{"version":"1.2.3"},{"version":"1.2.2"}]`, false, "statsPeriod=7d", 20, 2, true, ""},
		{"posthog sql bounded in the query", Event{Tool: "mcp__posthog__execute-sql", Input: map[string]any{"query": "SELECT event, count() FROM events WHERE timestamp >= now() - INTERVAL 7 DAY GROUP BY event LIMIT 50"}},
			`{"content":[{"type":"text","text":"{\"results\":[[\"$pageview\",120],[\"signed_up\",9]]}"}]}`, false, "in the query", 50, 2, true, ""},
		{"posthog sql with no limit", Event{Tool: "mcp__posthog__execute-sql", Input: map[string]any{"query": "SELECT count() FROM events WHERE timestamp > now() - INTERVAL 1 DAY"}},
			`{"results":[[3]]}`, false, "in the query", 0, 1, false, "no row limit"},
		{"sentry releases, limit only", Event{Tool: "mcp__sentry__find_releases", Input: map[string]any{"limit": float64(5)}},
			`[]`, false, "", 5, 0, false, "no time window"},
		{"posthog flags, unbounded", Event{Tool: "mcp__posthog__feature-flag-get-all", Input: map[string]any{}},
			`{"results":[]}`, false, "", 0, 0, false, "no time window and no row limit"},
		{"sentry natural-language search", Event{Tool: "mcp__sentry__search_issues", Input: map[string]any{"naturalLanguageQuery": "unresolved errors this week", "limit": float64(10)}},
			`not json`, false, "", 10, -1, false, "LLM"},
		{"sentry proxy reads the inner arguments", Event{Tool: "mcp__sentry__execute_sentry_tool", Input: map[string]any{"name": "find_releases", "arguments": map[string]any{"statsPeriod": "24h", "limit": "5"}}},
			`[]`, false, "statsPeriod=24h", 5, 0, true, ""},
		{"posthog query tool, parameters inside query", Event{Tool: "mcp__posthog__query-logs", Input: map[string]any{"query": map[string]any{"dateRange": map[string]any{"date_from": "-1h"}, "limit": float64(100)}}},
			`{"results":[{"body":"x"}]}`, false, `dateRange={"date_from":"-1h"}`, 100, 1, true, ""},
		{"failed call", Event{Tool: "mcp__sentry__find_releases", Input: map[string]any{"statsPeriod": "7d", "limit": float64(5)}},
			`{"isError":true}`, true, "statsPeriod=7d", 5, -1, false, "failed"},
		{"cli with flags", Event{Shell: "sentry-cli releases list --limit 10 --period 14d"},
			`[]`, false, "period=14d", 10, 0, true, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			v := Decide(tt.e, Options{})
			if !v.Allow {
				t.Fatalf("Decide refused the call: %s", v.Reason)
			}
			en := NewEntry(tt.e, v, Result{Output: []byte(tt.out), Failed: tt.failed}, "claude", "s1", at)
			if en.Window != tt.window || en.Limit != tt.limit || en.Rows != tt.rows || en.Citable != tt.citable || !strings.Contains(en.Why, tt.why) {
				t.Fatalf("entry %+v; want window %q limit %d rows %d citable %v why ~%q", en, tt.window, tt.limit, tt.rows, tt.citable, tt.why)
			}
			if en.Bytes != len(tt.out) || len(en.Digest) != 64 || en.Provider != v.Provider || !en.At.Equal(at) {
				t.Errorf("entry %+v: want bytes %d, a SHA-256 digest, provider %s", en, len(tt.out), v.Provider)
			}
		})
	}
	// Arguments are stored redacted; the result never is stored.
	pat := "ghp_" + strings.Repeat("0", 36) // built at run time so secret scanners see no token
	e := Event{Tool: "mcp__sentry__find_releases", Input: map[string]any{"query": "token " + pat}}
	en := NewEntry(e, Decide(e, Options{}), Result{Output: []byte(`[{"email":"person@example.com"}]`)}, "claude", "s1", at)
	b, _ := json.Marshal(en)
	if strings.Contains(string(b), pat) || strings.Contains(string(b), "person@example.com") {
		t.Errorf("the entry holds a secret or the result: %s", b)
	}
}

func TestAppend(t *testing.T) {
	dir := t.TempDir()
	for i, want := range []string{"c1", "c2"} {
		en, err := Append(dir, Entry{Session: "../../etc/passwd", Provider: Sentry, Call: "find_releases", At: at.Add(time.Duration(i) * time.Minute)})
		if err != nil || en.ID != want {
			t.Fatalf("append %d: %s %v; want %s", i, en.ID, err, want)
		}
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if len(files) != 1 || filepath.Dir(files[0]) != dir || strings.Contains(filepath.Base(files[0]), "/") {
		t.Fatalf("log files %v; want one file inside %s", files, dir)
	}
	f, _ := os.Open(files[0])
	defer func() { _ = f.Close() }()
	var ids []string
	for sc := bufio.NewScanner(f); sc.Scan(); {
		var en Entry
		_ = json.Unmarshal(sc.Bytes(), &en)
		ids = append(ids, en.ID)
	}
	if strings.Join(ids, ",") != "c1,c2" {
		t.Errorf("logged IDs %v; want c1,c2", ids)
	}
	// A host with no session ID shares one file per day; a very long ID is
	// cut, so a file name never outgrows the file system's limit.
	for _, tt := range []struct{ session, want string }{
		{"", "no-session-"},
		{strings.Repeat("s", 300), strings.Repeat("s", 80) + ".jsonl"},
	} {
		if _, err := Append(dir, Entry{Session: tt.session}); err != nil {
			t.Fatal(err)
		}
		if m, _ := filepath.Glob(filepath.Join(dir, tt.want+"*")); len(m) != 1 {
			t.Errorf("session %.12q…: files %v; want one named %s…", tt.session, m, tt.want)
		}
	}
	// A log directory that cannot be created is an error for the caller to report.
	blocker := filepath.Join(dir, "file")
	_ = os.WriteFile(blocker, nil, 0o600)
	if _, err := Append(filepath.Join(blocker, "calls"), Entry{Session: "s"}); err == nil {
		t.Error("Append under a file succeeded; want an error")
	}
}

func TestFromPostHook(t *testing.T) {
	for _, tt := range []struct {
		name, event, session, out string
		failed                    bool
	}{
		{"claude", `{"session_id":"abc","tool_name":"mcp__sentry__find_releases","tool_input":{},"tool_response":[{"version":"1"}]}`, "abc", `[{"version":"1"}]`, false},
		{"claude mcp error", `{"session_id":"abc","tool_name":"mcp__sentry__find_releases","tool_input":{},"tool_response":{"isError":true,"content":[]}}`, "abc", `{"isError":true,"content":[]}`, true},
		{"cursor", `{"conversation_id":"c9","hook_event_name":"afterMCPExecution","tool_name":"find_releases","tool_input":"{}","url":"https://mcp.sentry.dev/mcp","result_json":"[]"}`, "c9", `[]`, false},
		{"amp", `{"thread_id":"T-1","tool_name":"mcp__posthog__feature-flag-get-all","tool_input":{},"status":"error","output":null}`, "T-1", `null`, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, r, session, ok := FromPostHook([]byte(tt.event))
			if !ok || session != tt.session || string(r.Output) != tt.out || r.Failed != tt.failed {
				t.Fatalf("FromPostHook = %q %q failed=%v ok=%v; want %q %q failed=%v", session, r.Output, r.Failed, ok, tt.session, tt.out, tt.failed)
			}
		})
	}
	if _, _, _, ok := FromPostHook([]byte("not an event")); ok {
		t.Error("FromPostHook read a call from text that is not JSON")
	}
}
