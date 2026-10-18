package hostcall

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
)

// Entry is one PostHog or Sentry call an agent made through its host, as
// the post-tool hook logs it (Decision #449): enough to cite it like
// aayudha's own bounded queries (the tool, its window, its row limit) and to
// tell later whether a quoted result is the one the call returned. The
// result itself is never stored: only its digest, size, and row count.
type Entry struct {
	ID       string          `json:"id"` // c1, c2, … within one session's log
	At       time.Time       `json:"at"`
	Host     string          `json:"host"`
	Session  string          `json:"session,omitempty"`
	Channel  string          `json:"channel"` // mcp, cli
	Provider string          `json:"provider"`
	Call     string          `json:"call"`
	Args     json.RawMessage `json:"args,omitempty"` // redacted
	Window   string          `json:"window,omitempty"`
	Limit    int             `json:"limit,omitempty"`
	Rows     int             `json:"rows"`                    // -1 when the result's rows could not be counted
	Digest   string          `json:"result_sha256,omitempty"` // of the result as the host reported it
	Bytes    int             `json:"result_bytes"`
	Failed   bool            `json:"failed,omitempty"`
	Citable  bool            `json:"citable"`
	Why      string          `json:"why_not_citable,omitempty"`
}

// Result is what the host reported back for one call.
type Result struct {
	Output []byte // the tool's output as the host sent it
	Failed bool
}

// translated are Sentry tools that turn natural language into a query with
// an LLM: the same words can run a different query, so their results are for
// exploring and never cited.
var translated = map[string]bool{"search_errors": true, "search_events": true, "search_issue_events": true, "search_issues": true,
	"search_logs": true, "search_metrics": true, "search_profiles": true, "search_replays": true, "search_traces": true}

// windowKeys and limitKeys are the argument names PostHog and Sentry tools
// and CLIs use for a time window and a row limit.
var (
	windowKeys = []string{"dateRange", "date_range", "date_from", "dateFrom", "date_to", "dateTo", "after", "before", "start", "end", "statsPeriod", "period", "since", "until"}
	limitKeys  = []string{"limit", "per_page", "perPage", "max_rows", "maxRows", "first"}
	sqlLimit   = regexp.MustCompile(`(?i)\blimit\s+(\d+)`)
	sqlWindow  = regexp.MustCompile(`(?i)\b(timestamp|created_at|start_time|\$session_start)\b[^;]*?(>=?|between|<=?)|\binterval\b`)
	cliFlag    = regexp.MustCompile(`--(limit|max-rows|period|since|until|start|end)[= ](\S+)`)
)

// NewEntry builds the log entry for a call Decide allowed and the host ran.
// It reads the window and limit the call named, counts the result's rows
// where it can, and says why an entry cannot be cited.
func NewEntry(e Event, v Verdict, r Result, host, session string, at time.Time) Entry {
	en := Entry{At: at.UTC(), Host: host, Session: session, Channel: v.Channel, Provider: v.Provider, Call: v.Call,
		Rows: rows(r.Output), Bytes: len(r.Output), Failed: r.Failed}
	if len(r.Output) > 0 {
		sum := sha256.Sum256(r.Output)
		en.Digest = hex.EncodeToString(sum[:])
	}
	args := e.Input
	if v.Channel == "cli" {
		args = map[string]any{"command": e.Shell}
	}
	if b, err := json.Marshal(actions.Redact(args, nil)); err == nil && len(args) > 0 {
		en.Args = b
	}
	en.Call = actions.RedactString(en.Call)
	en.Window, en.Limit = bounds(e, v)
	tool := v.Call
	if i := strings.LastIndex(tool, " "); i >= 0 {
		tool = tool[i+1:]
	}
	switch {
	case r.Failed:
		en.Why = "the call failed"
	case v.Provider == Sentry && translated[tool]:
		en.Why = "Sentry translated natural language into the query with an LLM, so rerunning it may run another query; explore with it, cite a bounded query instead"
	case en.Window == "" && en.Limit == 0:
		en.Why = "it names no time window and no row limit"
	case en.Window == "":
		en.Why = "it names no time window"
	case en.Limit == 0:
		en.Why = "it names no row limit"
	default:
		en.Citable = true
	}
	return en
}

// bounds reads the window and the row limit a call names.
func bounds(e Event, v Verdict) (window string, limit int) {
	text := e.Shell
	if v.Channel == "mcp" {
		in := e.Input
		if inner, ok := in["arguments"].(map[string]any); ok { // execute_sentry_tool
			in = inner
		}
		if inner, ok := in["query"].(map[string]any); ok { // PostHog query tools: every parameter inside query
			in = inner
		}
		var w []string
		for _, k := range windowKeys {
			if val, ok := in[k]; ok && fmt.Sprint(val) != "" {
				b, _ := json.Marshal(val)
				w = append(w, k+"="+strings.Trim(string(b), `"`))
			}
		}
		window = strings.Join(w, " ")
		for _, k := range limitKeys {
			if n, ok := number(in[k]); ok && n > 0 {
				limit = n
				break
			}
		}
		if q, ok := in["query"].(string); ok {
			text = q
		} else if c, ok := in["command"].(string); ok { // PostHog exec
			text = c
		}
	}
	if m := cliFlag.FindAllStringSubmatch(text, -1); v.Channel == "cli" {
		for _, f := range m {
			if f[1] == "limit" || f[1] == "max-rows" {
				limit, _ = strconv.Atoi(f[2])
			} else {
				window = strings.TrimSpace(window + " " + f[1] + "=" + f[2])
			}
		}
	}
	if limit == 0 {
		if m := sqlLimit.FindStringSubmatch(text); m != nil {
			limit, _ = strconv.Atoi(m[1])
		}
	}
	if window == "" && sqlWindow.MatchString(text) {
		window = "in the query"
	}
	return window, limit
}

func number(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case string:
		i, err := strconv.Atoi(n)
		return i, err == nil
	}
	return 0, false
}

// rows counts the rows of a result: a JSON array, or an object holding one
// under results, data, or items; an MCP result's text blocks are read the
// same way. -1 when it cannot tell.
func rows(out []byte) int {
	var v any
	if json.Unmarshal(out, &v) != nil {
		return -1
	}
	return count(v, 0)
}

func count(v any, depth int) int {
	if depth > 3 {
		return -1
	}
	switch t := v.(type) {
	case []any:
		return len(t)
	case map[string]any:
		for _, k := range []string{"results", "data", "items", "issues", "rows"} {
			if a, ok := t[k].([]any); ok {
				return len(a)
			}
		}
		if c, ok := t["content"].([]any); ok { // an MCP CallToolResult
			for _, b := range c {
				if m, ok := b.(map[string]any); ok && m["type"] == "text" {
					var inner any
					if s, _ := m["text"].(string); json.Unmarshal([]byte(s), &inner) == nil {
						return count(inner, depth+1)
					}
				}
			}
		}
		if s, ok := t["structuredContent"]; ok {
			return count(s, depth+1)
		}
	}
	return -1
}

// Append adds en to the session's log under dir (.aayudha/state/calls),
// numbering it after the entries already there, and returns it with its ID.
func Append(dir string, en Entry) (Entry, error) {
	name := sessionFile(en.Session)
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return en, err
	}
	n := 0
	if f, err := os.Open(path); err == nil {
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 64<<10), 4<<20)
		for sc.Scan() {
			n++
		}
		_ = f.Close()
	}
	en.ID = fmt.Sprintf("c%d", n+1)
	b, err := json.Marshal(en)
	if err != nil {
		return en, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return en, err
	}
	defer func() { _ = f.Close() }()
	_, err = f.Write(append(b, '\n'))
	return en, err
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// sessionFile names a session's log file; a host that sends no session ID
// shares one file per day.
func sessionFile(session string) string {
	s := strings.Trim(unsafeName.ReplaceAllString(session, "_"), "._")
	if s == "" {
		s = "no-session-" + time.Now().UTC().Format("2006-01-02")
	}
	if len(s) > 80 {
		s = s[:80]
	}
	return s + ".jsonl"
}

// FromPostHook reads a host's post-tool event: the call as FromHook reads it,
// the result (Claude Code and Codex tool_response, Cursor result_json or a
// shell's output, the Amp plugin's output, Copilot's toolResult
// textResultForLlm), whether it failed (Copilot: a postToolUseFailure event,
// which carries error and no toolResult), and the session ID.
func FromPostHook(event []byte) (Event, Result, string, bool) {
	e, ok := FromHook(event)
	if !ok {
		return e, Result{}, "", false
	}
	var raw struct {
		Session      string          `json:"session_id"`
		Conversation string          `json:"conversation_id"`
		Thread       string          `json:"thread_id"`
		Response     json.RawMessage `json:"tool_response"`
		ResultJSON   string          `json:"result_json"`
		Output       json.RawMessage `json:"output"`
		Status       string          `json:"status"`
		Error        string          `json:"error"`
		// GitHub Copilot: postToolUse sends toolResult (tool_result in VS Code's
		// form) only for a call that succeeded; postToolUseFailure sends error.
		CopilotSession string         `json:"sessionId"`
		CopilotResult  *copilotResult `json:"toolResult"`
		VSCodeResult   *struct {
			Type string `json:"result_type"`
			Text string `json:"text_result_for_llm"`
		} `json:"tool_result"`
		HookEvent string `json:"hook_event_name"`
	}
	_ = json.Unmarshal(event, &raw)
	if v := raw.VSCodeResult; v != nil && raw.CopilotResult == nil {
		raw.CopilotResult = &copilotResult{Type: v.Type, Text: v.Text}
	}
	r := Result{Failed: raw.Status == "error" || raw.Error != "" || raw.HookEvent == "PostToolUseFailure"}
	switch c := raw.CopilotResult; {
	case c != nil:
		r.Output, r.Failed = []byte(c.Text), r.Failed || c.Type != "success"
	case len(raw.Response) > 0:
		r.Output = raw.Response
	case raw.ResultJSON != "":
		r.Output = []byte(raw.ResultJSON)
	case len(raw.Output) > 0:
		r.Output = raw.Output
	}
	var isErr struct {
		IsError bool `json:"isError"`
	}
	if json.Unmarshal(r.Output, &isErr) == nil && isErr.IsError {
		r.Failed = true
	}
	session := raw.Session
	if session == "" {
		session = raw.CopilotSession
	}
	if session == "" {
		session = raw.Conversation
	}
	if session == "" {
		session = raw.Thread
	}
	return e, r, session, true
}

// copilotResult is Copilot's toolResult.
type copilotResult struct {
	Type string `json:"resultType"`
	Text string `json:"textResultForLlm"`
}
