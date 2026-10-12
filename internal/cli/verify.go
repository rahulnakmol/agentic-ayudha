package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
	"github.com/rahulnakmol/agentic-ayudha/internal/verify"
)

func init() { register("verify", runVerify) }

var (
	storyRef     = regexp.MustCompile(`(?im)^\s*(?:refs|closes|fixes|resolves|story:?)\s+#([0-9]+)\b`)
	storyTrailer = regexp.MustCompile(`(?m)^Aayudha-Story:\s*#?([0-9]+)\s*$`)
	checkpoint   = regexp.MustCompile(`(?m)^Aayudha-Checkpoint:\s*([0-9]+/[0-9]+)\s*$`)
	storyBranch  = regexp.MustCompile(`^(?:story|bug)/([0-9]+)(?:-|$)`)
)

// VerifyResult is what verify reports.
type VerifyResult struct {
	Story      int           `json:"story,omitempty"`
	PR         int           `json:"pr,omitempty"`
	Head       string        `json:"head"`
	Base       string        `json:"base"`
	Footprint  []string      `json:"footprint"`
	Checks     []proof.Check `json:"checks"`
	Hold       bool          `json:"floors_hold"`
	Pack       string        `json:"pack,omitempty"` // local pack directory
	Unmapped   []int         `json:"criteria_without_evidence,omitempty"`
	Problems   []string      `json:"problems,omitempty"`
	NoSnapshot bool          `json:"no_ready_snapshot,omitempty"`
}

func gitIn(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return "", fmt.Errorf("git %s: %s", args[0], strings.TrimSpace(string(ee.Stderr)))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// StoryOf finds the story a change serves: "Refs #n" (or Closes/Fixes) in the
// PR body, else an Aayudha-Story trailer on the head commit, else a
// story/<n>-… branch name.
func StoryOf(body, headMessage, branch string) int {
	if m := storyRef.FindStringSubmatch(body); m != nil {
		return issueNumber(m[1])
	}
	if m := storyTrailer.FindStringSubmatch(headMessage); m != nil {
		return issueNumber(m[1])
	}
	if m := storyBranch.FindStringSubmatch(branch); m != nil {
		return issueNumber(m[1])
	}
	return 0
}

func (e *Env) devBranch() string {
	if l, err := e.Config(); err == nil && l.Config.Branches.Dev != "" {
		return l.Config.Branches.Dev
	}
	return "dev"
}

// readClient is a read-only client: the workflow token in CI, else the human's.
func (e *Env) readClient(ctx context.Context) (*github.Client, identity.Token, error) {
	tok, err := e.Resolver().Human(ctx)
	if err != nil {
		return nil, tok, Wrap(ExitBlocked, err)
	}
	ident := "human"
	if e.CI {
		ident = "ci"
	}
	c, err := e.newGitHub(tok, ident, "")
	return c, tok, err
}

func runVerify(ctx context.Context, e *Env, in Values) error {
	root := config.RepoRoot(e.Dir)
	if root == "" {
		return Errorf(ExitUsage, "verify runs in a checkout of the change")
	}
	l, err := e.Config()
	if err != nil {
		return err
	}
	head, err := gitIn(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	res := VerifyResult{Head: head}
	var (
		body, branch string
		labels       []string
		pr           *gh.PullRequest
		client       *github.Client
		tok          identity.Token
	)
	repo, _ := e.RepoName(ctx)
	if in.Has("pr") {
		res.PR = issueNumber(in.String("pr"))
		if client, tok, err = e.readClient(ctx); err != nil {
			return err
		}
		owner, name, err := github.SplitRepo(repo)
		if err != nil {
			return Wrap(ExitUsage, err)
		}
		if pr, _, err = client.REST.PullRequests.Get(ctx, owner, name, res.PR); err != nil {
			return Wrap(ExitExternal, err)
		}
		if pr.GetHead().GetSHA() != head {
			return Errorf(ExitBlocked, "verify %d: this checkout is at %s, but the PR head is %s; check out the PR head first", res.PR, short(head), short(pr.GetHead().GetSHA()))
		}
		res.Base, body, branch = pr.GetBase().GetSHA(), pr.GetBody(), pr.GetHead().GetRef()
		for _, lb := range pr.Labels {
			labels = append(labels, lb.GetName())
		}
		if mb, err := gitIn(ctx, root, "merge-base", res.Base, head); err == nil {
			res.Base = mb
		}
	} else {
		branch, _ = gitIn(ctx, root, "rev-parse", "--abbrev-ref", "HEAD")
		base := "origin/" + e.devBranch()
		if res.Base, err = gitIn(ctx, root, "merge-base", base, head); err != nil {
			return Errorf(ExitUsage, "verify: no merge base with %s (fetch it, or pass the PR): %v", base, err)
		}
	}
	msg, _ := gitIn(ctx, root, "log", "-1", "--format=%B", head)
	res.Story = StoryOf(body, msg, branch)

	r := &verify.Run{Root: root, Base: res.Base, Head: head, Floors: l.Config.Verify.Floors, Verify: l.Config.Verify, Story: res.Story, Labels: labels, Now: e.Now, Env: e.Getenv,
		CI: e.CI, Nightly: e.Getenv("GITHUB_EVENT_NAME") == "schedule"}
	if r.Changed, r.Lines, err = verify.Changes(ctx, root, res.Base, head); err != nil {
		return Wrap(ExitUsage, err)
	}
	r.Footprint = verify.ReadLayers(root).Footprint(r.Changed)
	res.Footprint = r.Footprint
	if res.Footprint == nil {
		res.Footprint = []string{}
	}

	var snap ready.LedgerSnapshot
	if res.Story > 0 {
		if client == nil {
			if _, tok, err = e.readClient(ctx); err != nil {
				return err
			}
		}
		w, err := e.Ledger(ctx, tok, "human", "")
		if err != nil {
			return err
		}
		if t, err := w.Read(ctx); err == nil {
			if b, err := t.File(ctx, ready.LedgerPath(res.Story)); err == nil {
				if snap, err = ready.ParseSnapshot(string(b)); err != nil {
					return err
				}
				r.Criteria = snap.Criteria
			}
			t.Close()
		}
		res.NoSnapshot = snap.CompletionGoalHash == ""
	}

	// Captures never run against production (WP4.5, V3): refuse before
	// seeding, capturing, or writing anything.
	if _, err := verify.ResolveTarget(r); err != nil {
		return Wrap(ExitBlocked, err)
	}

	home, err := os.MkdirTemp("", "aayudha-verify-home-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(home) }()
	work, err := os.MkdirTemp("", "aayudha-verify-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(work) }()
	r.Sandbox, r.Work = &verify.Sandbox{Getenv: e.Getenv, Home: home}, work

	floors := verify.Standard()
	if f := in.String("floor"); f != "" {
		if floors, err = verify.ByName(floors, f); err != nil {
			return Wrap(ExitUsage, err)
		}
	}
	rep := verify.Execute(ctx, r, floors)
	ev, problems, err := verify.TestEvidence(r)
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	rep.Artifacts = append(rep.Artifacts, ev...)
	res.Checks, res.Hold, res.Problems = rep.Checks, rep.Hold, problems

	if res.Story > 0 && !res.NoSnapshot {
		ident := identity.KindHumanFallback
		if identity.Configured(l.Config.Identity) != "" {
			ident = "bot"
		}
		if e.CI {
			ident = "ci"
		}
		host := e.Host
		if host == "" {
			host = "cli"
		}
		p := proof.Pack{Repo: repo, PR: res.PR, Seat: string(verbs.AINativeEngineer), Host: host, Identity: ident,
			CompletionGoalHash: snap.CompletionGoalHash, CreatedAt: e.Now().UTC().Format("2006-01-02T15:04:05Z"),
			Links: proof.Links{Issue: fmt.Sprintf("%s#%d", repo, res.Story), ReadySnapshot: ready.LedgerPath(res.Story)}}
		if res.PR > 0 {
			p.Links.PR = fmt.Sprintf("%s#%d", repo, res.PR)
		}
		if m := checkpoint.FindStringSubmatch(msg); m != nil {
			p.Checkpoint = m[1]
		}
		pack, files := verify.Pack(r, rep, p)
		res.Unmapped = verify.Unmapped(r, pack)
		res.Pack = proof.LocalDir(root, res.Story)
		if err := proof.WriteLocal(res.Pack, pack, files); err != nil {
			return err
		}
	}
	if err := e.Emit(res, func(w io.Writer) error { return renderVerify(w, res) }); err != nil {
		return err
	}
	if !res.Hold {
		return Errorf(ExitFloor, "floor miss: %s", failing(res.Checks))
	}
	return nil
}

func failing(cs []proof.Check) string {
	var out []string
	for _, c := range cs {
		if c.Result == proof.Fail || c.Result == proof.Blocked || proof.BlocksShip(c) {
			out = append(out, c.Floor+" "+c.Result)
		}
	}
	return strings.Join(out, ", ")
}

func renderVerify(w io.Writer, res VerifyResult) error {
	who := "no story"
	if res.Story > 0 {
		who = fmt.Sprintf("story #%d", res.Story)
	}
	fmt.Fprintf(w, "verify %s..%s (%s; footprint %s)\n", short(res.Base), short(res.Head), who, strings.Join(res.Footprint, " "))
	for _, c := range res.Checks {
		fmt.Fprintf(w, "  %-14s %-8s value %-8g threshold %-6g %s %s\n", c.Floor, c.Result, c.Value, c.Threshold, c.Tool, c.Version)
		if c.Detail != "" {
			fmt.Fprintf(w, "  %14s %s\n", "", c.Detail)
		}
	}
	for _, p := range res.Problems {
		fmt.Fprintf(w, "  evidence: %s\n", p)
	}
	switch {
	case res.Pack != "":
		fmt.Fprintf(w, "pack written to %s", res.Pack)
		if len(res.Unmapped) > 0 {
			fmt.Fprintf(w, " (criteria without evidence: %v)", res.Unmapped)
		}
		fmt.Fprintln(w, "; `aayu proof push` lands it on the ledger (advisory)")
	case res.Story > 0 && res.NoSnapshot:
		fmt.Fprintf(w, "no pack: #%d has no Ready snapshot on the ledger (run `aayu ready %d`)\n", res.Story, res.Story)
	default:
		fmt.Fprintln(w, "no pack: no story found (Refs #n in the PR body, an Aayudha-Story trailer, or a story/<n>-… branch)")
	}
	return nil
}
