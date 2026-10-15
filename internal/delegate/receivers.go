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
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
)

// Handoff is what a receiver is given: the brief and where to work.
type Handoff struct {
	Repo   string // owner/name
	Base   string // the branch the PR targets
	Story  int
	Title  string
	Brief  string
	Branch string // the suggested branch
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
		"autoCreatePR": true,
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
	args := []string{"-ox", h.Brief, "--project", h.Repo, "--title", fmt.Sprintf("aayu #%d: %s", h.Story, h.Title)}
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

func (a AmpCLI) bin() string {
	if a.Bin == "" {
		return "amp"
	}
	return a.Bin
}

// --- Claude Code: the @claude mention (anthropics/claude-code-action) -------

// GitHubOps is the GitHub access the Claude adapter needs, through aayu's
// actions-logged client.
type GitHubOps interface {
	Comment(ctx context.Context, issue int, body string) (url string, err error)
	Branches(ctx context.Context, prefix string) ([]string, error)
}

// ClaudeMention mentions @claude on the story with the brief; claude-code-action
// pushes a claude/issue-<n>-… branch, from which aayu opens the PR (the action
// does not open PRs by default). Follow-ups mention @claude on the PR, where
// the action pushes to the PR's branch.
type ClaudeMention struct {
	GitHub  GitHubOps
	Trigger string // "@claude" unless the workflow sets another trigger_phrase
}

func (c ClaudeMention) trigger() string {
	if c.Trigger == "" {
		return "@claude"
	}
	return c.Trigger
}

// Handoff comments the brief on the story.
func (c ClaudeMention) Handoff(ctx context.Context, h Handoff) (Task, error) {
	body := fmt.Sprintf("%s please build this story as the brief below says, on a new branch from `%s`.\n\n%s", c.trigger(), h.Base, h.Brief)
	url, err := c.GitHub.Comment(ctx, h.Story, body)
	if err != nil {
		return Task{}, fmt.Errorf("claude: %w", err)
	}
	return Task{ID: url, URL: url, Status: TaskWorking}, nil
}

// Status finds the branch the action pushed for the story.
func (c ClaudeMention) Status(ctx context.Context, st State) (Task, error) {
	t := Task{ID: st.Task, URL: st.TaskURL, Status: TaskWorking}
	bs, err := c.GitHub.Branches(ctx, fmt.Sprintf("claude/issue-%d-", st.Story))
	if err != nil {
		return t, err
	}
	if len(bs) > 0 {
		t.Branch = bs[len(bs)-1] // the newest name sorts last (the action appends a timestamp)
	}
	return t, nil
}

// FollowUp mentions @claude on the PR with the findings.
func (c ClaudeMention) FollowUp(ctx context.Context, st State, msg string) (Task, error) {
	if st.PR == 0 {
		return Task{}, errors.New("claude: no PR to follow up on")
	}
	url, err := c.GitHub.Comment(ctx, st.PR, c.trigger()+" "+msg)
	if err != nil {
		return Task{}, fmt.Errorf("claude: %w", err)
	}
	return Task{ID: st.Task, URL: url, Status: TaskWorking}, nil
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
