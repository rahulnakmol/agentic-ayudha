package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/delegate"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
)

// A Feature mission's Kanha (Story #1004, Decision #915 M1, Y9 and K1 A):
// a person's Start begins it, on the tool the plan records, and never aayu on
// its own. Amp, Claude Code, and Cursor run a cloud Kanha; Codex, Copilot,
// and OpenCode run a local one on the starter's machine (ST13, #1012). The
// Kanha's session is the `parent` in specs/<f>/approved.json, and each story
// thread it starts names it in delegate/<n>/state.json.

// localKanhaTools run their Kanha on the starter's machine: a cloud task of
// theirs takes no follow-up from the CLI (receivers.go, CodexCloud.FollowUp).
var localKanhaTools = []string{"codex", "copilot", "opencode"}

// kanhaCanStart says why the Kanha cannot start on tool here, or "", before
// anything is written.
func (e *Env) kanhaCanStart(tool string) string {
	switch {
	case slices.Contains(localKanhaTools, tool) && e.LocalKanha == nil:
		return fmt.Sprintf("a local Kanha on %s is not built yet (#1012)", tool)
	case !slices.Contains(localKanhaTools, tool) && !slices.Contains([]string{delegate.Amp, delegate.Claude, delegate.Cursor}, tool):
		return fmt.Sprintf("aayu starts no Kanha on %s", tool)
	}
	return ""
}

// startKanha starts the Kanha of Feature f's mission on the plan's tool and
// records it as the approval's parent. On amp it briefs the thread that
// drafted the Feature's intent while that thread is open; else it starts a
// new session, or a local one. The start is one record in the actions log.
// It returns the parent and where the Kanha runs, in words.
func (e *Env) startKanha(ctx context.Context, c *github.Client, w *ledger.Writer, repo string, f int, plan *MissionPlan, appr *ready.Approval, login string) (*ready.Parent, string, error) {
	l, _ := e.Config() // loaded and checked by the presence check before this
	root, tool := config.RepoRoot(e.Dir), plan.Tool
	p := &ready.Parent{Scope: "feature", Key: strconv.Itoa(f), Host: tool, Mode: "cloud", StartedBy: login, StartedAt: e.Now().UTC().Format(time.RFC3339)}
	brief := delegate.KanhaBrief(repo, f, tool)
	how := "new"
	start := func(ctx context.Context) error {
		if slices.Contains(localKanhaTools, tool) {
			host, _ := os.Hostname()
			p.Mode, p.Machine = "local", login+"@"+host
			id, err := e.LocalKanha(ctx, tool, root, brief)
			p.Session = id
			return err
		}
		ad := e.receiverAdapter(l.Config.Delegate, tool, c, repo, root)
		if tool == delegate.Amp {
			if id, url := e.draftingThread(ctx, w, f, appr, ad.(delegate.AmpCLI)); id != "" {
				how, p.Session, p.URL = "drafting", id, url
				_, err := ad.FollowUp(ctx, delegate.State{Task: id, TaskURL: url}, brief)
				return err
			}
		}
		t, err := ad.Handoff(ctx, delegate.Handoff{Repo: repo, Base: e.devBranch(), Scope: p.Scope, Key: p.Key, Title: fmt.Sprintf("Kanha of #%d", f), Brief: brief})
		if tool == delegate.Cursor { // the cloud agent, not its first run: each later event is a run on it
			t.ID, _, _ = strings.Cut(t.ID, "/")
		}
		p.Session, p.URL = t.ID, t.URL
		return err
	}
	log, err := e.Actions()
	if err != nil {
		return nil, "", err
	}
	_, err = log.Do(ctx, actions.Action{Kind: "mission.kanha.start", Target: fmt.Sprintf("%s#%d@%s", repo, f, tool), Story: fmt.Sprintf("#%d", f),
		Seat: "engineering-manager", Host: e.Host, Identity: c.Identity(), Payload: map[string]any{"tool": tool, "brief_sha256": delegate.BriefHash(brief)}},
		func(ctx context.Context) (actions.Outcome, error) {
			err := start(ctx)
			return actions.Outcome{Attempts: 1, Result: map[string]any{"session": p.Session, "url": p.URL, "mode": p.Mode, "machine": p.Machine, "thread": how}}, err
		})
	if err != nil {
		return nil, "", err
	}
	words := fmt.Sprintf("the Kanha of #%d runs on %s: %s", f, tool, p.URL)
	switch {
	case p.Mode == "local":
		words = fmt.Sprintf("the Kanha of #%d runs on this machine; while it is off, #%d pauses at the next merge", f, f)
	case how == "drafting":
		words = fmt.Sprintf("the Kanha of #%d is the thread that drafted its intent, %s: it has its brief", f, p.URL)
	}
	if err := e.writeParent(ctx, w, f, appr.Version, p); err != nil {
		return nil, "", fmt.Errorf("it started (%s %s), and aayu did not record it: %w", tool, p.Session, err)
	}
	return p, words, nil
}

// draftingThread is the Amp thread that drafted Feature f's approved intent
// (its baseline's source session), with its URL, while it is open; "" when
// it is not known, not an Amp thread, or archived.
func (e *Env) draftingThread(ctx context.Context, w *ledger.Writer, f int, appr *ready.Approval, amp delegate.AmpCLI) (string, string) {
	var b ready.Baseline
	for _, ab := range appr.Baselines {
		if ab.Part == "intent" { // an unreadable baseline names no session
			_, _ = readLedgerJSON(ctx, w, ready.BaselinePath(f, "intent", ab.Version, false), &b)
		}
	}
	url, ok := delegate.AmpThread(b.SourceSession)
	if !ok {
		return "", ""
	}
	if archived, err := amp.Archived(ctx, b.SourceSession); err != nil || archived {
		return "", "" // a thread this account cannot read is not known here
	}
	return b.SourceSession, url
}

// writeParent records the Kanha as the parent in specs/<f>/approved.json,
// on the approval it started under; another ledger write in between is read
// again, a new approval is not overwritten.
func (e *Env) writeParent(ctx context.Context, w *ledger.Writer, f, version int, p *ready.Parent) error {
	for range 3 {
		t, err := w.Read(ctx)
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		b, err := t.File(ctx, ready.ApprovalPath(f))
		tip := t.Tip
		t.Close()
		var appr ready.Approval
		if err != nil || json.Unmarshal(b, &appr) != nil || appr.Version != version {
			return Errorf(ExitExternal, "ledger %s: approval %d of #%d is not there to record its Kanha", ready.ApprovalPath(f), version, f)
		}
		appr.Parent = p
		data, _ := json.MarshalIndent(appr, "", "  ")
		_, err = w.Write(ctx, ledger.Commit{Message: fmt.Sprintf("missions: the Kanha of #%d runs on %s (%s)", f, p.Host, p.Mode),
			Files: map[string][]byte{ready.ApprovalPath(f): append(data, '\n')}, Base: tip})
		if !errors.Is(err, ledger.ErrMoved) {
			return Wrap(ExitExternal, err)
		}
	}
	return Errorf(ExitExternal, "the ledger kept moving while aayu recorded the Kanha of #%d", f)
}

// storyMission is the mission issue n belongs to: its parent Feature, plan,
// and approval; plan is nil when its parent has no mission plan.
func storyMission(ctx context.Context, c *github.Client, w *ledger.Writer, repo string, n int) (int, *MissionPlan, *ready.Approval, error) {
	owner, name, _ := github.SplitRepo(repo)
	parent, _, err := c.REST.SubIssue.GetParentIssue(ctx, owner, name, int64(n))
	if err != nil { // a story with no parent is in no mission
		return 0, nil, nil, Wrap(ExitExternal, ignoreMissing(err))
	}
	f := parent.GetNumber()
	plan, appr, _, err := readMission(ctx, w, f)
	if err != nil || plan == nil || appr == nil {
		return f, nil, nil, err
	}
	return f, plan, appr, nil
}

// storyTool is the tool story n runs on: the tool of the person who started
// it in a Tatkal Feature, else the mission's.
func (p *MissionPlan) storyTool(n int) string {
	if i := slices.IndexFunc(p.StoryStarts, func(s MissionStoryStart) bool { return s.Number == n }); i >= 0 {
		return p.StoryStarts[i].Tool
	}
	return p.Tool
}

// startedOn says who started Tatkal story n and on which tool: "@bob on codex".
func startedOn(p *MissionPlan, n int) string {
	i := slices.IndexFunc(p.StoryStarts, func(s MissionStoryStart) bool { return s.Number == n })
	return fmt.Sprintf("@%s started it on %s", p.StoryStarts[i].StartedBy, p.StoryStarts[i].Tool)
}
