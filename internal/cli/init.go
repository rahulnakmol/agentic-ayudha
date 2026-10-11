package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

func init() { register("init", runInit) }

// RepoName returns the repository this invocation targets: --repo, else the
// owner/name of the current checkout's origin remote.
func (e *Env) RepoName(ctx context.Context) (string, error) {
	if e.Repo != "" {
		return e.Repo, nil
	}
	if root := config.RepoRoot(e.Dir); root != "" {
		if out, err := e.exec()(ctx, root, "git", "remote", "get-url", "origin"); err == nil {
			if m := originRepo.FindStringSubmatch(strings.TrimSpace(out)); m != nil {
				return m[1], nil
			}
		}
	}
	return "", Errorf(ExitUsage, "no repository: pass --repo owner/name or run inside a checkout with a GitHub origin")
}

// ownerIsOrg reports whether the repository owner is an organization, which
// decides between GitHub issue types and the type:<id> label fallback (R1).
func (e *Env) ownerIsOrg(ctx context.Context, repo string) (bool, error) {
	owner, _, _ := strings.Cut(repo, "/")
	var u struct {
		Type string `json:"type"`
	}
	var err error
	if tok, terr := e.Resolver().Human(ctx); terr == nil {
		err = e.githubGet(ctx, tok, "/users/"+owner, &u)
	} else {
		err = getJSON(ctx, e.HTTP, e.githubAPI()+"/users/"+owner, &u)
	}
	if err != nil {
		return false, Wrap(ExitExternal, fmt.Errorf("owner %s: %w", owner, err))
	}
	return u.Type == "Organization", nil
}

// labelTypeActions is the WP2.2 part of init's plan: the label set, and the
// issue types on an organization owner (on a personal repo the type:<id>
// labels stand in for issue types).
func labelTypeActions(repo string, org bool) []actions.Action {
	var plan []actions.Action
	owner, _, _ := strings.Cut(repo, "/")
	if org {
		for _, it := range workitems.IssueTypes() {
			plan = append(plan, actions.Action{
				Kind: "github.issue_type.create", Target: owner, IdempotencyKey: "init:" + repo + ":type:" + it.Name,
				Request: &actions.Request{Method: http.MethodPost, Path: "/orgs/" + owner + "/issue-types"},
				Payload: map[string]any{"name": it.Name, "description": it.Description, "color": it.Color, "is_enabled": true},
			})
		}
	}
	for _, l := range workitems.Labels() {
		if org && strings.HasPrefix(l.Name, "type:") {
			continue
		}
		plan = append(plan, actions.Action{
			Kind: "github.label.create", Target: repo, IdempotencyKey: "init:" + repo + ":label:" + l.Name,
			Request: &actions.Request{Method: http.MethodPost, Path: "/repos/" + repo + "/labels"},
			Payload: map[string]any{"name": l.Name, "color": l.Color, "description": l.Description},
		})
	}
	return plan
}

func runInit(ctx context.Context, e *Env, _ Values) error {
	if !e.DryRun {
		return Errorf(ExitUsage, "init: this binary only plans (--dry-run); the writes land in WP2.5")
	}
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	org, err := e.ownerIsOrg(ctx, repo)
	if err != nil {
		return err
	}
	log, err := e.Actions()
	if err != nil {
		return err
	}
	var recs []actions.Record
	for _, a := range labelTypeActions(repo, org) {
		a.Identity = "human"
		rec, err := log.Do(ctx, a, nil)
		if err != nil {
			return err
		}
		recs = append(recs, rec)
	}
	return e.Emit(map[string]any{"repo": repo, "organization": org, "planned": recs}, func(w io.Writer) error {
		home := "type:<id> labels (personal repository: no issue types)"
		if org {
			home = "GitHub issue types"
		}
		fmt.Fprintf(w, "init --dry-run on %s: work-item types as %s; nothing was written\n", repo, home)
		for _, r := range recs {
			fmt.Fprintf(w, "  %-8s %-24s %v\n", r.Status, r.Kind, r.Payload["name"])
		}
		return nil
	})
}
