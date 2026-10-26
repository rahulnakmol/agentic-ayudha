package delegate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
)

// Handoff is what a receiver is given: the brief and where to work. A story's
// hand-off names the story; a coordinator's (a Feature's Kanha, Story #1004;
// Kala, #911) names its scope (feature or repo) and work key, and opens no PR.
type Handoff struct {
	Repo   string // owner/name
	Base   string // the branch the PR targets
	Story  int
	Scope  string // a coordinator's scope; empty for a story
	Key    string // a coordinator's work key: the Feature's number
	Title  string
	Brief  string
	Branch string // the suggested branch
}

// ref is the work the hand-off is for: "#20", or a Feature Kanha's "#889".
func (h Handoff) ref() string {
	if h.Key != "" {
		return "#" + h.Key
	}
	return fmt.Sprintf("#%d", h.Story)
}

// Task statuses a receiver reports.
const (
	TaskWorking = "working"
	TaskDone    = "done"
	TaskFailed  = "failed"
)

// Task is what a receiver reports about its work.
type Task struct {
	ID     string `json:"id"`
	URL    string `json:"url,omitempty"`
	Status string `json:"status"`
	Branch string `json:"branch,omitempty"`
	PRURL  string `json:"pr_url,omitempty"`
	Diff   string `json:"-"` // Codex: the change, which aayu applies and commits (diff return)
	Detail string `json:"detail,omitempty"`
}

// Adapter is one receiver's transport. Every adapter sends the same brief;
// none is given a GitHub token or merge rights by aayu.
type Adapter interface {
	Handoff(ctx context.Context, h Handoff) (Task, error)
	Status(ctx context.Context, st State) (Task, error)
	FollowUp(ctx context.Context, st State, msg string) (Task, error)
}

// Exec runs a command: dir, environment (nil inherits), stdin, then argv.
type Exec func(ctx context.Context, dir string, env []string, stdin, name string, args ...string) (string, error)

// ErrNotConnected means a receiver's cloud agent is not set up here; the
// caller falls back to the local CLI or names what to connect.
var ErrNotConnected = errors.New("not connected")

// --- Cursor: the cloud agents API (public beta, v1) --------------------------

// CursorAPI talks to Cursor's cloud agents API: POST /v1/agents with
// autoCreatePR, GET the run for its branch and PR, POST a run to follow up.
type CursorAPI struct {
	BaseURL string // https://api.cursor.com
	Key     string // a Cursor API key (dashboard or service account)
	Model   string // empty: Cursor's default model
	HTTP    *http.Client
}

func (c CursorAPI) do(ctx context.Context, method, path string, in, out any) error {
	if c.Key == "" {
		return fmt.Errorf("cursor: %w: no API key (aayu auth set-secret cursor-api-key)", ErrNotConnected)
	}
	var body io.Reader
	if in != nil {
		b, _ := json.Marshal(in)
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.BaseURL, "/")+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Key)
	req.Header.Set("Content-Type", "application/json")
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("cursor: %s %s: %s: %s", method, path, resp.Status, actions.RedactString(strings.TrimSpace(string(b[:min(len(b), 300)]))))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(b, out)
}

type cursorRun struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Git    struct {
		Branches []struct {
			Branch string `json:"branch"`
			PRURL  string `json:"prUrl"`
		} `json:"branches"`
	} `json:"git"`
}

// Handoff starts a cloud agent on the repository's base branch.
func (c CursorAPI) Handoff(ctx context.Context, h Handoff) (Task, error) {
	req := map[string]any{
		"prompt":       map[string]any{"text": h.Brief},
		"repos":        []map[string]any{{"url": "https://github.com/" + h.Repo, "startingRef": h.Base}},
		"autoCreatePR": h.Scope == "", // a coordinator writes no code
	}
	if c.Model != "" {
		req["model"] = map[string]any{"id": c.Model}
	}
	var out struct {
		Agent struct {
			ID, Status, URL string
		} `json:"agent"`
		Run cursorRun `json:"run"`
	}
	if err := c.do(ctx, http.MethodPost, "/v1/agents", req, &out); err != nil {
		return Task{}, err
	}
	if out.Agent.ID == "" || out.Run.ID == "" {
		return Task{}, errors.New("cursor: POST /v1/agents answered without an agent and run ID")
	}
	return Task{ID: out.Agent.ID + "/" + out.Run.ID, URL: out.Agent.URL, Status: TaskWorking}, nil
}

// Status reads the agent's latest run: its status, branch, and PR.
func (c CursorAPI) Status(ctx context.Context, st State) (Task, error) {
	agent, run, ok := strings.Cut(st.Task, "/")
	if !ok {
		return Task{}, fmt.Errorf("cursor: task %q is not <agent>/<run>", st.Task)
	}
	var r cursorRun
	if err := c.do(ctx, http.MethodGet, "/v1/agents/"+agent+"/runs/"+run, nil, &r); err != nil {
		return Task{}, err
	}
	t := Task{ID: st.Task, URL: st.TaskURL, Status: TaskWorking, Detail: "run " + strings.ToLower(r.Status)}
	switch r.Status {
	case "FINISHED":
		t.Status = TaskDone
	case "ERROR", "CANCELLED", "EXPIRED":
		t.Status = TaskFailed
	}
	for _, b := range r.Git.Branches {
		if b.Branch != "" {
			t.Branch = b.Branch
		}
		if b.PRURL != "" {
			t.PRURL = b.PRURL
		}
	}
	return t, nil
}

// FollowUp starts a new run of the same agent with the findings.
func (c CursorAPI) FollowUp(ctx context.Context, st State, msg string) (Task, error) {
	agent, _, _ := strings.Cut(st.Task, "/")
	var out struct {
		ID  string    `json:"id"`
		Run cursorRun `json:"run"`
	}
	if err := c.do(ctx, http.MethodPost, "/v1/agents/"+agent+"/runs", map[string]any{"prompt": map[string]any{"text": msg}}, &out); err != nil {
		return Task{}, err
	}
	run := out.Run.ID
	if run == "" {
		run = out.ID
	}
	if run == "" {
		return Task{}, errors.New("cursor: the follow-up answered without a run ID")
	}
	return Task{ID: agent + "/" + run, URL: st.TaskURL, Status: TaskWorking}, nil
}

// --- Amp: an orb thread through the amp CLI ----------------------------------

// AmpCLI starts an Amp thread in an orb (`amp -ox --project`) and continues it
// for follow-ups. Amp has no status call aayu relies on: the PR, found by
// `Refs #n`, is the signal.
type AmpCLI struct {
	Exec    Exec
	Bin     string // amp
	OrbSize string // e.g. a1.small; empty: the project's default
	Env     []string
}

var ampThread = regexp.MustCompile(`https://ampcode\.com/threads/(T-[0-9a-f-]{36})`)

// Handoff starts the orb thread with the brief.
func (a AmpCLI) Handoff(ctx context.Context, h Handoff) (Task, error) {
	args := []string{"-ox", h.Brief, "--project", h.Repo, "--title", fmt.Sprintf("aayu %s: %s", h.ref(), h.Title)}
	if a.OrbSize != "" {
		args = append(args, "--orb-size", a.OrbSize)
	}
	out, err := a.Exec(ctx, "", a.Env, "", a.bin(), args...)
	if err != nil {
		return Task{}, fmt.Errorf("amp: %w", err)
	}
	m := ampThread.FindStringSubmatch(out)
	if m == nil {
		return Task{}, fmt.Errorf("amp: no thread URL in the CLI's output: %s", clip(out, 200))
	}
	return Task{ID: m[1], URL: m[0], Status: TaskWorking}, nil
}

// Status is "working" until the PR appears; the thread itself is the record.
func (a AmpCLI) Status(_ context.Context, st State) (Task, error) {
	return Task{ID: st.Task, URL: st.TaskURL, Status: TaskWorking}, nil
}

// FollowUp continues the thread in its orb.
func (a AmpCLI) FollowUp(ctx context.Context, st State, msg string) (Task, error) {
	if _, err := a.Exec(ctx, "", a.Env, "", a.bin(), "threads", "continue", st.Task, "-ox", msg); err != nil {
		return Task{}, fmt.Errorf("amp: %w", err)
	}
	return Task{ID: st.Task, URL: st.TaskURL, Status: TaskWorking}, nil
}

// AmpThread is the URL of an Amp thread ID, and whether the ID is one.
func AmpThread(id string) (string, bool) {
	u := "https://ampcode.com/threads/" + id
	return u, ampThread.FindString(u) == u
}

// Archived reports whether the Amp thread is archived (`amp threads export`),
// so a brief sent to it would wake nothing.
func (a AmpCLI) Archived(ctx context.Context, thread string) (bool, error) {
	out, err := a.Exec(ctx, "", a.Env, "", a.bin(), "threads", "export", thread)
	if err != nil {
		return false, fmt.Errorf("amp: %w", err)
	}
	var t struct {
		ID       string `json:"id"`
		Archived bool   `json:"archived"`
	}
	if json.Unmarshal([]byte(out), &t) != nil || t.ID != thread {
		return false, fmt.Errorf("amp: threads export %s: not the thread's export: %s", thread, clip(out, 200))
	}
	return t.Archived, nil
}

func (a AmpCLI) bin() string {
	if a.Bin == "" {
		return "amp"
	}
	return a.Bin
}

// --- Claude Code: a cloud session on the web (claude --cloud) ---------------

// GitHubOps is the GitHub access the Claude adapter needs: the branches on
// origin, read through aayu's client.
type GitHubOps interface {
	Branches(ctx context.Context, prefix string) ([]string, error)
}

// ClaudeCloud starts a Claude Code on the web session from the user's own
// signed-in CLI (`claude --cloud "<brief>"`), run in the repository so the
// session clones its GitHub remote. It needs a claude.ai sign-in and no key in
// the repository's CI (claude-code-action, which does, is not used; Appendix B
// D-5.5c.4). A session pushes its branch; aayu opens the PR from it. Follow-ups
// queue a message on the session (`claude -p <msg> --cloud <session>`).
type ClaudeCloud struct {
	Exec   Exec
	Bin    string // claude
	Dir    string // the repository root
	Env    []string
	GitHub GitHubOps
}

var (
	claudeSessionURL = regexp.MustCompile(`https://claude\.ai/code/(session_[A-Za-z0-9_-]+)`)
	claudeSessionID  = regexp.MustCompile(`Session ID:\s*(session_[A-Za-z0-9_-]+)`)
)

// Handoff creates the cloud session with the brief.
func (c ClaudeCloud) Handoff(ctx context.Context, h Handoff) (Task, error) {
	out, err := c.Exec(ctx, c.Dir, c.Env, "", c.bin(), "--cloud", h.Brief)
	if err != nil {
		return Task{}, fmt.Errorf("claude: %w", err)
	}
	t := Task{Status: TaskWorking}
	if m := claudeSessionURL.FindStringSubmatch(out); m != nil {
		t.ID, t.URL = m[1], m[0]
	}
	if m := claudeSessionID.FindStringSubmatch(out); m != nil {
		t.ID = m[1]
	}
	if t.ID == "" {
		return Task{}, fmt.Errorf("claude: no cloud session in the CLI's output: %s", clip(out, 200))
	}
	if t.URL == "" {
		t.URL = "https://claude.ai/code/" + t.ID
	}
	return t, nil
}

// Status finds the branch the brief named once the session has pushed it.
func (c ClaudeCloud) Status(ctx context.Context, st State) (Task, error) {
	t := Task{ID: st.Task, URL: st.TaskURL, Status: TaskWorking}
	if st.Branch == "" {
		return t, nil
	}
	bs, err := c.GitHub.Branches(ctx, st.Branch)
	if err != nil {
		return t, err
	}
	if slices.Contains(bs, st.Branch) {
		t.Branch = st.Branch
	}
	return t, nil
}

// FollowUp queues the findings on the session.
func (c ClaudeCloud) FollowUp(ctx context.Context, st State, msg string) (Task, error) {
	out, err := c.Exec(ctx, c.Dir, c.Env, "", c.bin(), "-p", msg, "--cloud", st.Task, "--output-format", "json")
	if err != nil {
		return Task{}, fmt.Errorf("claude: %w", err)
	}
	var r struct {
		OK        bool   `json:"ok"`
		SessionID string `json:"session_id"`
		URL       string `json:"url"`
	}
	if json.Unmarshal([]byte(strings.TrimSpace(out)), &r) != nil || !r.OK {
		return Task{}, fmt.Errorf("claude: the follow-up was not queued: %s", clip(out, 200))
	}
	return Task{ID: st.Task, URL: st.TaskURL, Status: TaskWorking}, nil
}

func (c ClaudeCloud) bin() string {
	if c.Bin == "" {
		return "claude"
	}
	return c.Bin
}

// --- Codex: cloud tasks through the codex CLI (experimental), diff return ---

// CodexCloud submits a Codex cloud task (`codex cloud exec`), polls it
// (`codex cloud status`), and, when it is done, reads its diff (`codex cloud
// diff`) for aayu to apply, commit, and open as a PR: a cloud task's PR is a
// click in the Codex UI, which aayu cannot make.
type CodexCloud struct {
	Exec  Exec
	Bin   string // codex
	EnvID string // the Codex cloud environment ID ([delegate] codex_env)
	Env   []string
}

var (
	codexTaskURL = regexp.MustCompile(`https://chatgpt\.com/codex/tasks/([A-Za-z0-9_-]+)`)
	codexTaskID  = regexp.MustCompile(`\b(task_[A-Za-z0-9_-]+)\b`)
	codexDone    = regexp.MustCompile(`(?i)\b(completed|complete|ready|succeeded|done)\b`)
	codexFailed  = regexp.MustCompile(`(?i)\b(failed|error|cancelled|canceled)\b`)
)

func (c CodexCloud) bin() string {
	if c.Bin == "" {
		return "codex"
	}
	return c.Bin
}

func (c CodexCloud) submit(ctx context.Context, branch, prompt string) (Task, error) {
	if c.EnvID == "" {
		return Task{}, fmt.Errorf("codex: %w: no cloud environment ([delegate] codex_env)", ErrNotConnected)
	}
	out, err := c.Exec(ctx, "", c.Env, "", c.bin(), "cloud", "exec", "--env", c.EnvID, "--branch", branch, prompt)
	if err != nil {
		return Task{}, fmt.Errorf("codex: %w", err)
	}
	if m := codexTaskURL.FindStringSubmatch(out); m != nil {
		return Task{ID: m[1], URL: m[0], Status: TaskWorking}, nil
	}
	if m := codexTaskID.FindStringSubmatch(out); m != nil {
		return Task{ID: m[1], URL: "https://chatgpt.com/codex/tasks/" + m[1], Status: TaskWorking}, nil
	}
	return Task{}, fmt.Errorf("codex: no task ID in the CLI's output: %s", clip(out, 200))
}

// Handoff submits the brief as a cloud task on the base branch.
func (c CodexCloud) Handoff(ctx context.Context, h Handoff) (Task, error) {
	return c.submit(ctx, h.Base, h.Brief)
}

// Status polls the task and, when it is done, fetches its diff.
func (c CodexCloud) Status(ctx context.Context, st State) (Task, error) {
	t := Task{ID: st.Task, URL: st.TaskURL, Status: TaskWorking}
	out, err := c.Exec(ctx, "", c.Env, "", c.bin(), "cloud", "status", st.Task)
	if err != nil {
		return t, fmt.Errorf("codex: %w", err)
	}
	t.Detail = clip(strings.TrimSpace(out), 120)
	switch {
	case codexFailed.MatchString(out):
		t.Status = TaskFailed
	case codexDone.MatchString(out):
		diff, err := c.Exec(ctx, "", c.Env, "", c.bin(), "cloud", "diff", st.Task)
		if err != nil {
			return t, fmt.Errorf("codex: %w", err)
		}
		t.Status, t.Diff = TaskDone, diff
	}
	return t, nil
}

// FollowUp submits a new task on the PR's branch with the findings.
func (c CodexCloud) FollowUp(ctx context.Context, st State, msg string) (Task, error) {
	return c.submit(ctx, st.Branch, msg)
}

func clip(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
