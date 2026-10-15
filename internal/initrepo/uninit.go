package initrepo

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/hosts"
)

// Uninit removes what init created, matched by the IDs in state.json. Adopted
// objects, files someone has edited since, and a ledger that has been written
// to since init are kept and reported. External objects (an App, projects, a
// bucket) are listed, never deleted: their removal needs their own console.
func (in *Init) Uninit(ctx context.Context) error {
	r := in.Repo
	st := in.State
	var labels []*gh.Label
	// The ledger branch goes last, after the rulesets: the history ruleset
	// blocks its deletion for everyone, admins included (D-4.2b.h).
	objs := slices.Clone(st.Objects)
	last := func(o Object) int { return map[bool]int{true: 1}[o.Kind == "ledger"] }
	slices.SortStableFunc(objs, func(a, b Object) int { return cmp.Compare(last(a), last(b)) })
	for _, o := range objs {
		if o.Adopted {
			in.note("uninit", o.Kind, o.Name, "kept", "existed before init")
			if !in.dry() {
				st.Remove(o.Kind, o.Name)
			}
			continue
		}
		if o.External {
			in.note("uninit", o.Kind, o.Name, "kept", "external: remove it by hand (uninit --external lists it)")
			continue
		}
		var err error
		switch o.Kind {
		case "label":
			if labels == nil {
				labels, err = github.All(ctx, func(ctx context.Context, lo gh.ListOptions) ([]*gh.Label, *gh.Response, error) {
					return in.GH.REST.Issues.ListLabels(ctx, r.Owner, r.Name, &lo)
				})
				if err != nil {
					return err
				}
			}
			i := slices.IndexFunc(labels, func(l *gh.Label) bool { return strconv.FormatInt(l.GetID(), 10) == o.ID })
			if i < 0 {
				st.Remove(o.Kind, o.Name)
				in.note("uninit", o.Kind, o.Name, "unchanged", "already gone")
				continue
			}
			name := labels[i].GetName() // the label may have been renamed; the ID is what init created
			err = in.del(ctx, o, "github.label.delete", "/repos/"+r.FullName+"/labels/"+name, func(ctx context.Context) error {
				_, err := in.GH.REST.Issues.DeleteLabel(ctx, r.Owner, r.Name, name)
				return err
			})
		case "issue-type":
			id, _ := strconv.ParseInt(o.ID, 10, 64)
			err = in.del(ctx, o, "github.issue_type.delete", "/orgs/"+r.Owner+"/issue-types/"+o.ID, func(ctx context.Context) error {
				_, err := in.GH.REST.Organizations.DeleteIssueType(ctx, r.Owner, id)
				return err
			})
		case "ruleset", "workflow-ruleset":
			id, _ := strconv.ParseInt(o.ID, 10, 64)
			err = in.del(ctx, o, "github.ruleset.delete", "/repos/"+r.FullName+"/rulesets/"+o.ID, func(ctx context.Context) error {
				_, err := in.GH.REST.Repositories.DeleteRuleset(ctx, r.Owner, r.Name, id)
				return err
			})
		case "ledger":
			ref, _, gerr := in.GH.REST.Git.GetRef(ctx, r.Owner, r.Name, "heads/ledger")
			switch {
			case github.NotFound(gerr):
				st.Remove(o.Kind, o.Name)
				in.note("uninit", "branch", "ledger", "unchanged", "already gone")
				continue
			case gerr != nil:
				return gerr
			case ref.GetObject().GetSHA() != o.ID:
				in.note("uninit", "branch", "ledger", "kept", "written to since init: it holds proof and snapshots")
				continue
			}
			err = in.del(ctx, o, "github.ledger.delete", "/repos/"+r.FullName+"/git/refs/heads/ledger", func(ctx context.Context) error {
				_, err := in.GH.REST.Git.DeleteRef(ctx, r.Owner, r.Name, "heads/ledger")
				return err
			})
		case "secret", "variable":
			path := "/repos/" + r.FullName + "/actions/" + o.Kind + "s/" + o.ID
			err = in.del(ctx, o, "github."+o.Kind+".delete", path, func(ctx context.Context) error {
				return in.rest(ctx, http.MethodDelete, path, nil, nil)
			})
		default:
			in.note("uninit", o.Kind, o.Name, "kept", "uninit does not remove this kind")
			continue
		}
		if err != nil {
			return err
		}
	}
	for _, f := range slices.Clone(st.Files) {
		if err := in.removeFile(f); err != nil {
			return err
		}
	}
	if in.dry() {
		return nil
	}
	if len(st.Objects) == 0 && len(st.Files) == 0 {
		return os.Remove(filepath.Join(in.Root, StatePath))
	}
	st.Complete = false
	return in.save()
}

func (in *Init) del(ctx context.Context, o Object, kind, path string, do func(context.Context) error) error {
	rec, err := in.GH.Write(ctx, actions.Action{
		Kind: kind, Target: in.Repo.FullName, Request: &actions.Request{Method: http.MethodDelete, Path: path},
		Payload: map[string]any{"name": o.Name, "id": o.ID},
	}, func(ctx context.Context) (any, error) { return nil, do(ctx) })
	if err != nil && !github.NotFound(err) {
		return err
	}
	if rec.Status == actions.StatusPlanned {
		in.note("uninit", o.Kind, o.Name, "planned", "delete")
		return nil
	}
	in.State.Remove(o.Kind, o.Name)
	in.note("uninit", o.Kind, o.Name, "deleted", "")
	return nil
}

func (in *Init) removeFile(f File) error {
	full := filepath.Join(in.Root, filepath.FromSlash(f.Path))
	cur, err := os.ReadFile(full)
	if errors.Is(err, os.ErrNotExist) {
		in.forgetFile(f.Path)
		return nil
	}
	if err != nil {
		return err
	}
	if f.Section != "" {
		next := hosts.RemoveAgentsSection(cur, f.Section)
		if bytes.Equal(next, cur) {
			in.forgetFile(f.Path)
			return nil
		}
		if in.dry() {
			in.note("uninit", "file", f.Path, "planned", "remove section "+f.Section)
			return nil
		}
		if f.Created && strings.TrimSpace(string(next)) == "# AGENTS.md" {
			err = os.Remove(full)
		} else {
			err = os.WriteFile(full, next, 0o644)
		}
		if err != nil {
			return err
		}
		in.forgetFile(f.Path)
		in.note("uninit", "file", f.Path, "deleted", "section "+f.Section)
		return nil
	}
	switch {
	case !f.Created:
		in.note("uninit", "file", f.Path, "kept", "existed before init")
		in.forgetFile(f.Path)
		return nil
	case hash(cur) != f.SHA256:
		in.note("uninit", "file", f.Path, "kept", "edited since init")
		return nil
	case in.dry():
		in.note("uninit", "file", f.Path, "planned", "delete")
		return nil
	}
	if err := os.Remove(full); err != nil {
		return err
	}
	in.forgetFile(f.Path)
	in.note("uninit", "file", f.Path, "deleted", "")
	return nil
}

func (in *Init) forgetFile(path string) {
	if in.dry() {
		return
	}
	in.State.Files = slices.DeleteFunc(in.State.Files, func(f File) bool { return f.Path == path })
}

// External lists the external objects init created (WP2.6): what
// `uninit --external` reports as orphaned once the repository is uninitialised.
func (s *State) External() []Object {
	var out []Object
	for _, o := range s.Objects {
		if o.External {
			out = append(out, o)
		}
	}
	return out
}

// String renders a change for the text report.
func (c Change) String() string {
	s := fmt.Sprintf("%-9s %-11s %s", c.Result, c.Kind, c.Name)
	if c.Detail != "" {
		s += " (" + c.Detail + ")"
	}
	return s
}
