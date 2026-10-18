package initrepo

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	pathpkg "path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/doctor"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/hosts"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// Steps of a core init, in the plan's order: identity, then labels, types, and
// forms, then the ledger, then config.
var Steps = []string{"identity", "labels", "issue-types", "forms", "ledger", "ruleset", "config", "agents"}

// Change is one line of init's report.
type Change struct {
	Step   string `json:"step"`
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Result string `json:"result"` // created, adopted, unchanged, written, planned, kept, deleted
	Detail string `json:"detail,omitempty"`
}

// Repo is what init learns about the repository before it writes.
type Repo struct {
	FullName   string
	Owner      string
	Name       string
	Org        bool
	Private    bool
	Default    string
	Branches   []string // remote branch names (from the coexistence report)
	Coexist    doctor.Coexistence
	HumanLogin string
	BotKind    string // app, machine-user, or "" (human fallback)
}

// Init runs init against one repository checkout.
type Init struct {
	GH      *github.Client
	Repo    Repo
	Root    string // repository checkout; files are written here for the human to commit
	State   *State
	Changes []Change
}

// Media extensions tracked by LFS on a private repository's ledger (V3): design
// mockups and UI baselines. Proof media never go to the ledger (S-4.2b).
var mediaExts = proof.MediaExts

// The rulesets init creates on the ledger branch when no existing ruleset
// already protects it (Appendix B D-4.2b.h). LedgerRuleset restricts updates
// to its bypass actors: the write role and up (and the bot App), so a human
// with write access can run ship, while the Actions workflow token, which
// cannot be a bypass actor on a personal repository (D1), never writes the
// ledger. A bypass actor bypasses every rule of its ruleset, so force pushes
// and deletion are blocked by LedgerHistoryRuleset, which has no bypass
// actors at all.
const (
	LedgerRuleset        = "aayudha-ledger"
	LedgerHistoryRuleset = "aayudha-ledger-history"
)

// protectedRefs are the branches both ledger rulesets cover: the ledger and
// the brain vault `aayu brain` writes, protected alike (same rules, same
// bypass actors).
var protectedRefs = []string{"refs/heads/ledger", "refs/heads/brain"}

// brainRef is the brain branch; an adopted ruleset that lacks it is reported,
// never edited.
const brainRef = "refs/heads/brain"

// writeRole is the repository's write role as a ruleset bypass actor; it
// covers maintain and admin too (current_user_can_bypass is "always" for the
// admin with only this actor listed, D-4.2b.h).
var writeRole = map[string]any{"actor_id": 4, "actor_type": "RepositoryRole", "bypass_mode": "always"}

// NewState starts the state for a first run.
func NewState(repo string) *State {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return &State{SchemaVersion: StateVersion, Repo: repo, Run: hex.EncodeToString(b), Objects: []Object{}, Files: []File{}}
}

func (in *Init) dry() bool { return in.GH.DryRun() }

func (in *Init) note(step, kind, name, result, detail string) {
	in.Changes = append(in.Changes, Change{step, kind, name, result, detail})
}

func (in *Init) save() error {
	if in.dry() {
		return nil
	}
	return in.State.Save(in.Root)
}

// Run performs the steps not yet done (all of them unless resuming), saving
// state after every object so a failure can be resumed.
func (in *Init) Run(ctx context.Context, resume bool) error {
	steps := map[string]func(context.Context) error{
		"identity": in.identity, "labels": in.labels, "issue-types": in.issueTypes, "forms": in.forms,
		"ledger": in.ledger, "ruleset": in.ruleset, "config": in.config, "agents": in.agents,
	}
	for _, s := range Steps {
		if resume && in.State.IsDone(s) {
			in.note(s, "step", s, "unchanged", "done in an earlier run")
			continue
		}
		if err := steps[s](ctx); err != nil {
			_ = in.save()
			return fmt.Errorf("init step %s: %w (state saved; rerun with --resume)", s, err)
		}
		in.State.MarkDone(s)
		if err := in.save(); err != nil {
			return err
		}
	}
	in.State.Complete = true
	return in.save()
}

func (in *Init) key(parts ...string) string {
	return "init:" + in.State.Run + ":" + strings.Join(parts, ":")
}

func (in *Init) identity(context.Context) error {
	in.State.Login = in.Repo.HumanLogin
	in.State.Identity = in.Repo.BotKind
	in.State.Visibility = map[bool]string{true: "private", false: "public"}[in.Repo.Private]
	if in.Repo.BotKind == "" {
		in.State.Identity = "human-fallback"
		in.note("identity", "identity", in.Repo.HumanLogin, "unchanged", "no bot identity: agent writes run as "+in.Repo.HumanLogin+", tagged identity:human-fallback (init --identity creates the App)")
		return nil
	}
	in.note("identity", "identity", in.Repo.BotKind, "unchanged", "bot identity configured")
	return nil
}

func (in *Init) labels(ctx context.Context) error {
	r := in.Repo
	existing, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.Label, *gh.Response, error) {
		return in.GH.REST.Issues.ListLabels(ctx, r.Owner, r.Name, &o)
	})
	if err != nil {
		return err
	}
	for _, def := range workitems.Labels() {
		if r.Org && strings.HasPrefix(def.Name, "type:") {
			continue // organizations use issue types (R1)
		}
		if o, ok := in.State.Object("label", def.Name); ok {
			if slices.ContainsFunc(existing, func(l *gh.Label) bool { return strconv.FormatInt(l.GetID(), 10) == o.ID }) {
				in.note("labels", "label", def.Name, "unchanged", "")
				continue
			}
		}
		if i := slices.IndexFunc(existing, func(l *gh.Label) bool { return strings.EqualFold(l.GetName(), def.Name) }); i >= 0 {
			in.State.Put(Object{Kind: "label", Name: def.Name, ID: strconv.FormatInt(existing[i].GetID(), 10), Adopted: true})
			in.note("labels", "label", def.Name, "adopted", "already present; left as it is")
			continue
		}
		var id int64
		rec, err := in.GH.Write(ctx, actions.Action{
			Kind: "github.label.create", Target: r.FullName, IdempotencyKey: in.key("label", def.Name),
			Request: &actions.Request{Method: http.MethodPost, Path: "/repos/" + r.FullName + "/labels"},
			Payload: map[string]any{"name": def.Name, "color": def.Color, "description": def.Description},
		}, func(ctx context.Context) (any, error) {
			l, _, err := in.GH.REST.Issues.CreateLabel(ctx, r.Owner, r.Name, gh.CreateIssueLabelRequest{Name: def.Name, Color: gh.Ptr(def.Color), Description: gh.Ptr(def.Description)})
			id = l.GetID()
			return id, err
		})
		if err != nil {
			return err
		}
		if in.recordCreated(rec, Object{Kind: "label", Name: def.Name, ID: strconv.FormatInt(id, 10)}, "labels") {
			if err := in.save(); err != nil {
				return err
			}
		}
	}
	return nil
}

// recordCreated notes a created (or planned) object and records it unless dry.
func (in *Init) recordCreated(rec actions.Record, o Object, step string) bool {
	if rec.Status == actions.StatusPlanned {
		in.note(step, o.Kind, o.Name, "planned", "")
		return false
	}
	in.State.Put(o)
	in.note(step, o.Kind, o.Name, "created", "")
	return true
}

func (in *Init) issueTypes(ctx context.Context) error {
	r := in.Repo
	if !r.Org {
		in.note("issue-types", "issue-type", "*", "unchanged", "personal repository: the type:<id> labels stand in for issue types (R1)")
		return nil
	}
	existing, _, err := in.GH.REST.Organizations.ListIssueTypes(ctx, r.Owner)
	if err != nil {
		return err
	}
	for _, def := range workitems.IssueTypes() {
		if o, ok := in.State.Object("issue-type", def.Name); ok {
			if slices.ContainsFunc(existing, func(t *gh.IssueType) bool { return strconv.FormatInt(t.GetID(), 10) == o.ID }) {
				in.note("issue-types", "issue-type", def.Name, "unchanged", "")
				continue
			}
		}
		if i := slices.IndexFunc(existing, func(t *gh.IssueType) bool { return strings.EqualFold(t.GetName(), def.Name) }); i >= 0 {
			in.State.Put(Object{Kind: "issue-type", Name: def.Name, ID: strconv.FormatInt(existing[i].GetID(), 10), Adopted: true})
			in.note("issue-types", "issue-type", def.Name, "adopted", "already defined for "+r.Owner)
			continue
		}
		var id int64
		rec, err := in.GH.Write(ctx, actions.Action{
			Kind: "github.issue_type.create", Target: r.Owner, IdempotencyKey: in.key("type", def.Name),
			Request: &actions.Request{Method: http.MethodPost, Path: "/orgs/" + r.Owner + "/issue-types"},
			Payload: map[string]any{"name": def.Name, "description": def.Description, "color": def.Color},
		}, func(ctx context.Context) (any, error) {
			t, _, err := in.GH.REST.Organizations.CreateIssueType(ctx, r.Owner, &gh.CreateOrUpdateIssueTypesOptions{Name: def.Name, IsEnabled: true, Description: gh.Ptr(def.Description), Color: gh.Ptr(def.Color)})
			id = t.GetID()
			return id, err
		})
		if err != nil {
			return err
		}
		if in.recordCreated(rec, Object{Kind: "issue-type", Name: def.Name, ID: strconv.FormatInt(id, 10)}, "issue-types") {
			if err := in.save(); err != nil {
				return err
			}
		}
	}
	return nil
}

// writeFile writes a repository file init owns. A file init wrote before and
// that is unchanged since is updated; a file someone else wrote or edited is
// kept as it is and reported.
func (in *Init) writeFile(step, path string, content []byte) error {
	full := filepath.Join(in.Root, filepath.FromSlash(path))
	old, err := os.ReadFile(full)
	exists := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	switch {
	case exists && bytes.Equal(old, content):
		if !slices.ContainsFunc(in.State.Files, func(f File) bool { return f.Path == path }) && !in.dry() {
			in.State.PutFile(File{Path: path, SHA256: hash(content), Created: false})
		}
		in.note(step, "file", path, "unchanged", "")
		return nil
	case exists && !in.ownsUnchanged(path, old):
		in.note(step, "file", path, "kept", "exists with other content; not overwritten")
		return nil
	}
	if in.dry() {
		in.note(step, "file", path, "planned", "")
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(full, content, 0o644); err != nil {
		return err
	}
	in.State.PutFile(File{Path: path, SHA256: hash(content), Created: !exists})
	in.note(step, "file", path, "written", "")
	return nil
}

func (in *Init) ownsUnchanged(path string, current []byte) bool {
	for _, f := range in.State.Files {
		if f.Path == path {
			return f.SHA256 == hash(current)
		}
	}
	return false
}

// FormPath is where init installs a type's issue form.
func FormPath(t workitems.Type) string { return ".github/ISSUE_TEMPLATE/" + workitems.FormFile(t) }

func (in *Init) forms(context.Context) error {
	for _, t := range workitems.Types {
		if err := in.writeFile("forms", FormPath(t), workitems.Form(t, workitems.FormOptions{IssueTypes: in.Repo.Org})); err != nil {
			return err
		}
	}
	return nil
}

func (in *Init) ledger(ctx context.Context) error {
	r := in.Repo
	ref, _, err := in.GH.REST.Git.GetRef(ctx, r.Owner, r.Name, "heads/ledger")
	switch {
	case err == nil:
		sha := ref.GetObject().GetSHA()
		if _, ok := in.State.Object("ledger", "ledger"); ok {
			in.note("ledger", "branch", "ledger", "unchanged", "")
			return nil
		}
		in.State.Put(Object{Kind: "ledger", Name: "ledger", ID: sha, Adopted: true})
		in.note("ledger", "branch", "ledger", "adopted", "already exists at "+sha[:min(7, len(sha))]+"; left as it is")
		return nil
	case !github.NotFound(err):
		return err
	}
	files := map[string]string{"README.md": ledgerReadme(r)}
	if r.Private {
		files[".gitattributes"] = lfsAttributes()
		files[".lfsconfig"] = "# Design and UI baseline media on this ledger are stored in GitHub LFS (V3: private repository).\n[lfs]\n\tlocksverify = false\n"
	}
	var entries []*gh.TreeEntry
	for _, p := range slices.Sorted(func(yield func(string) bool) {
		for p := range files {
			if !yield(p) {
				return
			}
		}
	}) {
		entries = append(entries, &gh.TreeEntry{Path: gh.Ptr(p), Mode: gh.Ptr("100644"), Type: gh.Ptr("blob"), Content: gh.Ptr(files[p])})
	}
	var sha string
	rec, err := in.GH.Write(ctx, actions.Action{
		Kind: "github.ledger.create", Target: r.FullName + "@ledger", IdempotencyKey: in.key("ledger"),
		Request: &actions.Request{Method: http.MethodPost, Path: "/repos/" + r.FullName + "/git/refs"},
		Payload: map[string]any{"ref": "refs/heads/ledger", "files": len(entries), "lfs": r.Private},
	}, func(ctx context.Context) (any, error) {
		tree, _, err := in.GH.REST.Git.CreateTree(ctx, r.Owner, r.Name, "", entries)
		if err != nil {
			return nil, err
		}
		c, _, err := in.GH.REST.Git.CreateCommit(ctx, r.Owner, r.Name, gh.Commit{Message: gh.Ptr("ledger: create (aayu init)"), Tree: tree}, nil)
		if err != nil {
			return nil, err
		}
		sha = c.GetSHA()
		_, _, err = in.GH.REST.Git.CreateRef(ctx, r.Owner, r.Name, gh.CreateRef{Ref: "refs/heads/ledger", SHA: sha})
		return sha, err
	})
	if err != nil {
		return err
	}
	in.recordCreated(rec, Object{Kind: "ledger", Name: "ledger", ID: sha}, "ledger")
	return nil
}

func lfsAttributes() string {
	var b strings.Builder
	b.WriteString("# Design and UI baseline media are stored in GitHub LFS on private repositories (V3); proof media never come here.\n")
	for _, e := range mediaExts {
		fmt.Fprintf(&b, "*.%s filter=lfs diff=lfs merge=lfs -text\n", e)
	}
	return b.String()
}

func ledgerReadme(r Repo) string {
	media := "Proof media are never stored here: they stay in each merge gate run's `aayudha-pack` artifact, and `aayu ship` lands `pack.json` and its data only. Design mockups and UI baselines are stored in GitHub LFS (private repository)."
	if !r.Private {
		media = "This repository is public, so no media are stored here or in Git LFS (V3). Proof media stay in each merge gate run's `aayudha-pack` artifact, and `aayu ship` lands `pack.json` and its data only."
	}
	return "# ledger\n\nOrphan branch written by aayudha: Ready snapshots (`specs/<id>/ready.md`), proof packs, actions segments\n(`actions/<date>/<host>-<run>.jsonl`), and audit reports. Not for code; never merge it.\n\n" + media + "\n"
}

// rulesetDetail is the part of a ruleset init inspects.
type rulesetDetail struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Target      string `json:"target"`
	Enforcement string `json:"enforcement"`
	Conditions  struct {
		RefName struct {
			Include []string `json:"include"`
			Exclude []string `json:"exclude"`
		} `json:"ref_name"`
	} `json:"conditions"`
	Rules []struct {
		Type string `json:"type"`
	} `json:"rules"`
	BypassActors []struct {
		ActorID    int64  `json:"actor_id"`
		ActorType  string `json:"actor_type"`
		BypassMode string `json:"bypass_mode"`
	} `json:"bypass_actors"`
}

func (d rulesetDetail) ruleTypes() []string {
	var types []string
	for _, r := range d.Rules {
		types = append(types, r.Type)
	}
	slices.Sort(types)
	return types
}

func (d rulesetDetail) protectsLedger() bool {
	if d.Target != "branch" || d.Enforcement != "active" {
		return false
	}
	if !slices.Contains(d.Conditions.RefName.Include, "refs/heads/ledger") {
		return false
	}
	types := d.ruleTypes()
	return slices.Contains(types, "deletion") && slices.Contains(types, "non_fast_forward")
}

// brainAdvice tells the person how to protect the brain branch with an
// adopted ruleset that does not cover it yet; aayudha never edits the team's
// own ruleset. It is empty when the ruleset already includes the branch.
func (d rulesetDetail) brainAdvice() string {
	if slices.Contains(d.Conditions.RefName.Include, brainRef) {
		return ""
	}
	return fmt.Sprintf("; it does not protect %s: add %q to ruleset %q (id %d) conditions.ref_name.include so the brain branch cannot be force-pushed or deleted either", brainRef, brainRef, d.Name, d.ID)
}

func (in *Init) rest(ctx context.Context, method, path string, body, out any) error {
	req, err := in.GH.REST.NewRequest(ctx, method, strings.TrimPrefix(path, "/"), body)
	if err != nil {
		return err
	}
	_, err = in.GH.REST.Do(req, out)
	return err
}

// ruleset applies the ledger rulesets. Where the plan has no rulesets (a
// private repository on GitHub Free answers 403 "Upgrade to GitHub Pro or make
// this repository public"), init says so, records why in state.json for
// `aayu doctor`, and carries on: the ledger branch is then unprotected, and a
// later `aayu init` applies the rulesets once the plan allows them
// (Appendix B D-2.5b).
func (in *Init) ruleset(ctx context.Context) error {
	err := in.applyRulesets(ctx)
	var er *gh.ErrorResponse
	if errors.As(err, &er) && er.Response != nil && er.Response.StatusCode == http.StatusForbidden && strings.Contains(er.Message, "Upgrade to GitHub") {
		in.State.RulesetsUnapplied = "GitHub answered 403: " + er.Message
		in.note("ruleset", "ruleset", LedgerRuleset, "not applied",
			"this repository's GitHub plan has no rulesets ("+er.Message+"), so the ledger and brain branches are unprotected: anyone with write access can force-push or delete them. Make the repository public or upgrade to GitHub Pro, then run aayu init again; aayu doctor reports it until then")
		return nil
	}
	if err == nil {
		in.State.RulesetsUnapplied = ""
	}
	return err
}

func (in *Init) applyRulesets(ctx context.Context) error {
	r := in.Repo
	var list []rulesetDetail
	if err := in.rest(ctx, http.MethodGet, "/repos/"+r.FullName+"/rulesets", nil, &list); err != nil {
		return err
	}
	listed := func(id string) bool {
		return slices.ContainsFunc(list, func(s rulesetDetail) bool { return strconv.FormatInt(s.ID, 10) == id })
	}
	o, recorded := in.State.Object("ruleset", LedgerRuleset)
	if recorded && o.Adopted && listed(o.ID) {
		// The detail only adds the brain advice; failing to read it changes nothing.
		d := rulesetDetail{Name: LedgerRuleset}
		advice := ""
		if err := in.rest(ctx, http.MethodGet, "/repos/"+r.FullName+"/rulesets/"+o.ID, nil, &d); err == nil {
			advice = d.brainAdvice()
		}
		in.note("ruleset", "ruleset", d.Name, "unchanged", "the team's own ruleset; aayudha leaves it as it is"+advice)
		return nil
	}
	if !recorded || o.Adopted {
		for _, s := range list {
			var d rulesetDetail
			if err := in.rest(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/rulesets/%d", r.FullName, s.ID), nil, &d); err != nil {
				return err
			}
			if d.protectsLedger() {
				in.State.Put(Object{Kind: "ruleset", Name: LedgerRuleset, ID: strconv.FormatInt(d.ID, 10), Adopted: true})
				in.note("ruleset", "ruleset", d.Name, "adopted", "already protects refs/heads/ledger; added nothing alongside it"+d.brainAdvice())
				return nil
			}
		}
	}
	// History first, so a 0.1 ruleset loses its force-push and deletion rules
	// only once the history ruleset holds them.
	if err := in.ensureRuleset(ctx, listed, LedgerHistoryRuleset, []string{"deletion", "non_fast_forward"}, nil); err != nil {
		return err
	}
	return in.ensureRuleset(ctx, listed, LedgerRuleset, []string{"update"}, writeRole)
}

// ensureRuleset creates the ledger ruleset name with exactly rules, or
// brings the recorded one to them, adding bypass to its bypass actors and
// keeping the rest (the bot App, D-4.2b.d).
func (in *Init) ensureRuleset(ctx context.Context, listed func(string) bool, name string, rules []string, bypass map[string]any) error {
	r := in.Repo
	body := map[string]any{
		"name": name, "target": "branch", "enforcement": "active",
		"conditions":    map[string]any{"ref_name": map[string]any{"include": protectedRefs, "exclude": []string{}}},
		"rules":         []map[string]any{},
		"bypass_actors": []map[string]any{},
	}
	for _, t := range rules {
		body["rules"] = append(body["rules"].([]map[string]any), map[string]any{"type": t})
	}
	if bypass != nil {
		body["bypass_actors"] = []map[string]any{bypass}
	}
	if o, ok := in.State.Object("ruleset", name); ok && listed(o.ID) {
		var d rulesetDetail
		path := "/repos/" + r.FullName + "/rulesets/" + o.ID
		if err := in.rest(ctx, http.MethodGet, path, nil, &d); err != nil {
			return err
		}
		has := bypass == nil
		for _, a := range d.BypassActors {
			if a.ActorType == bypass["actor_type"] && a.ActorID == int64(bypass["actor_id"].(int)) {
				has = true
				continue
			}
			body["bypass_actors"] = append(body["bypass_actors"].([]map[string]any), map[string]any{"actor_id": a.ActorID, "actor_type": a.ActorType, "bypass_mode": a.BypassMode})
		}
		// Keep any refs the ruleset already includes and add the missing ones
		// (an older init covered refs/heads/ledger only). The include list is
		// part of the update's idempotency key, so a run that already updated
		// this ruleset (the 0.1 migration) still adds the brain branch.
		include := slices.Clone(d.Conditions.RefName.Include)
		for _, ref := range protectedRefs {
			if !slices.Contains(include, ref) {
				include = append(include, ref)
			}
		}
		// An exclusion beats an inclusion (GitHub ref-name conditions), so an
		// exclusion of a protected ref itself is dropped; a pattern that also
		// matches other refs is kept, and the note says what it still lets through.
		var exclude, shadow []string
		for _, x := range d.Conditions.RefName.Exclude {
			if slices.Contains(protectedRefs, x) {
				continue
			}
			exclude = append(exclude, x)
			for _, ref := range protectedRefs {
				if ok, _ := pathpkg.Match(x, ref); ok {
					shadow = append(shadow, fmt.Sprintf("exclusion %q still exempts %s; remove it from ruleset %q to protect that branch", x, ref, name))
				}
			}
		}
		if exclude == nil {
			exclude = []string{}
		}
		body["conditions"] = map[string]any{"ref_name": map[string]any{"include": include, "exclude": exclude}}
		warn := strings.Join(shadow, "; ")
		if has && slices.Equal(d.ruleTypes(), rules) && len(include) == len(d.Conditions.RefName.Include) && len(exclude) == len(d.Conditions.RefName.Exclude) {
			in.note("ruleset", "ruleset", name, "unchanged", warn)
			return nil
		}
		if _, err := in.GH.Write(ctx, actions.Action{Kind: "github.ruleset.update", Target: r.FullName, IdempotencyKey: in.key("ruleset-update", name, strings.Join(include, ",")+"!"+strings.Join(exclude, ",")),
			Request: &actions.Request{Method: http.MethodPut, Path: path}, Payload: body},
			func(ctx context.Context) (any, error) { return nil, in.rest(ctx, http.MethodPut, path, body, nil) }); err != nil {
			return err
		}
		msg := "write-access humans write the ledger and brain branches; force pushes and deletion stay blocked"
		if warn != "" {
			msg = warn
		}
		in.note("ruleset", "ruleset", name, "updated", msg)
		return nil
	}
	var created rulesetDetail
	rec, err := in.GH.Write(ctx, actions.Action{
		Kind: "github.ruleset.create", Target: r.FullName, IdempotencyKey: in.key("ruleset", name),
		Request: &actions.Request{Method: http.MethodPost, Path: "/repos/" + r.FullName + "/rulesets"}, Payload: body,
	}, func(ctx context.Context) (any, error) {
		err := in.rest(ctx, http.MethodPost, "/repos/"+r.FullName+"/rulesets", body, &created)
		var er *gh.ErrorResponse
		if errors.As(err, &er) && er.Response != nil && er.Response.StatusCode == http.StatusForbidden {
			err = fmt.Errorf("%w: creating the ledger ruleset needs repository administration rights (a classic token with repo scope from an admin, or a fine-grained token with Administration: write)", err)
		}
		return created.ID, err
	})
	if err != nil {
		return err
	}
	in.recordCreated(rec, Object{Kind: "ruleset", Name: name, ID: strconv.FormatInt(created.ID, 10)}, "ruleset")
	return nil
}

// ConfigPath is the repository config file.
const ConfigPath = ".aayudha/config.toml"

func (in *Init) config(context.Context) error {
	r := in.Repo
	var b strings.Builder
	fmt.Fprintf(&b, "# aayudha configuration for %s, written by `aayu init` (WP2.5). It layers on the defaults built into\n", r.FullName)
	b.WriteString("# the binary; `aayu config show --origin` shows where each value comes from. Protected path: changes need a\n# human CODEOWNER review.\nversion = 1\n")
	if !r.Private {
		b.WriteString("\n[proof]\nmedia = \"\" # optional private bucket. Proof media stay in the merge gate's run artifact without one; design and UI baseline media need one here, since a public repository never puts media in Git LFS (V3).\n")
	}
	if !r.Coexist.HasDev && r.Default != "" && r.Default != "dev" {
		mainBranch := r.Default
		if slices.Contains(r.Branches, "main") {
			mainBranch = "main"
		}
		rolling := ""
		if slices.Contains(r.Branches, "rolling") {
			rolling = "rolling"
		}
		fmt.Fprintf(&b, "\n# No dev branch: the existing branches are mapped, nothing was renamed (coexistence, WP2.5).\n[branches]\ndev = %q\nrolling = %q\nmain = %q\n", r.Default, rolling, mainBranch)
	}
	full := filepath.Join(in.Root, ConfigPath)
	if cur, err := os.ReadFile(full); err == nil {
		if in.ownsUnchanged(ConfigPath, cur) {
			in.note("config", "file", ConfigPath, "unchanged", "")
		} else {
			in.note("config", "file", ConfigPath, "kept", "existing configuration left as it is")
		}
	} else if err := in.writeFile("config", ConfigPath, []byte(b.String())); err != nil {
		return err
	}
	if err := in.writeFile("config", ".aayudha/.gitignore", []byte("# Local aayudha runtime state (written by aayu init); never committed.\ntelemetry/\nstate/\nledger-outbox/\nproof/\nworktrees/\nconfig.local.toml\nconfig.local.json\n")); err != nil {
		return err
	}
	return in.writeFile("config", SeedReadmePath, []byte(SeedReadme))
}

// SeedReadmePath is the seed-data scaffold init writes (WP4.5).
const SeedReadmePath = ".aayudha/seed/README.md"

// SeedReadme explains the seed-data contract of the web floors.
const SeedReadme = `# Seed data for proof captures

` + "`aayu verify`" + ` captures screenshots, recordings, and screen-reader transcripts of the running app. Those captures may
only show seeded, synthetic data (docs/BUILD-PLAN.md V3, WP4.5). Put the command that loads it here and name it in
` + "`.aayudha/config.toml`" + `:

` + "```toml" + `
[verify]
seed = ["./.aayudha/seed/load.sh"]   # argv; runs once before the first capture

[verify.environments.local]
url = "http://localhost:3000"        # AAYU_ENV picks the environment (default local)

[verify.environments.prod]
url = "https://app.example.com"
production = true                    # verify refuses to capture or seed it (exit 4)
` + "```" + `

Use reserved example domains (` + "`example.com`" + `, ` + "`*.test`" + `) for email addresses. The redaction pass blocks a pack
whose captures show any other address, a payment card number, or a national ID number.
`

// AgentsSectionID is the AGENTS.md section init owns.
const AgentsSectionID = "work-items"

// AgentsSection is init's section of AGENTS.md (rendered by internal/hosts).
func AgentsSection(r Repo) hosts.AgentsSection {
	var b strings.Builder
	b.WriteString("## Work items (aayudha)\n\n")
	b.WriteString("Work is tracked as GitHub issues of ten types (Goal, Initiative, Feature, Story, Bug, Decision, Experiment,\n")
	b.WriteString("Release, Feedback, Incident). File them with the `aayudha-<type>` issue forms. ")
	if r.Org {
		b.WriteString("Each form sets its GitHub issue type.\n")
	} else {
		b.WriteString("The `type:<type>` label is the type.\n")
	}
	b.WriteString("\n- Priority, Effort, Risk, Health, and Horizon are labels (`priority:P1`, `effort:S`, …), one per prefix;\n")
	b.WriteString("  Delivery Status is the `status:<value>` label.\n")
	b.WriteString("- A Story or Bug needs a Completion Goal of numbered criteria, one per line:\n")
	b.WriteString("  `N. [screenshot|recording|sr-transcript|api-trace|metric] state: …; expected: …`.\n")
	b.WriteString("- Run `aayu ready <issue> --check` before asking for Ready; `aayu ready <issue>` posts the Ready snapshot. Do not\n")
	b.WriteString("  edit a Ready issue's body: the Ready guard strips `status:ready` when it drifts.\n")
	b.WriteString("- The `ledger` branch holds snapshots, proof, and actions segments. Never commit code to it or merge it.\n")
	b.WriteString("- Every GitHub write goes through `aayu`; agents never write with `gh` directly.\n")
	b.WriteString("- Every pull request that is not docs-only gets a review by a model of another family than the builder's: by\n")
	b.WriteString("  default the merge gate's CI review (the `code-review` environment's model, rung B), or the agent app that\n")
	b.WriteString("  `[inspect] reviewer` names (rung A). When no other family can be shown, the review is a labelled same-family\n")
	b.WriteString("  review (rung C). Rung C never counts toward the streak; it counts for merge on proof only where\n")
	b.WriteString("  `[autonomy] proof_rungs` lists C. `aayu build` asks for the review after each push and writes `Aayudha-Host`\n")
	b.WriteString("  and `Aayudha-Model` trailers. For a pull request `aayu build` did\n")
	b.WriteString("  not open, put `Aayudha-Host: <host>` and `Aayudha-Model: <model>` lines in its body, one per model used, and\n")
	b.WriteString("  run `aayu inspect <pr>` after the last push.")
	return hosts.AgentsSection{ID: AgentsSectionID, Body: b.String()}
}

func (in *Init) agents(context.Context) error {
	full := filepath.Join(in.Root, "AGENTS.md")
	old, err := os.ReadFile(full)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	next := hosts.RenderAgentsMD(old, AgentsSection(in.Repo))
	if bytes.Equal(old, next) {
		in.note("agents", "file", "AGENTS.md", "unchanged", "")
		return nil
	}
	if in.dry() {
		in.note("agents", "file", "AGENTS.md", "planned", "section "+AgentsSectionID)
		return nil
	}
	if err := os.WriteFile(full, next, 0o644); err != nil {
		return err
	}
	in.State.PutFile(File{Path: "AGENTS.md", SHA256: hash(next), Created: len(old) == 0, Section: AgentsSectionID})
	in.note("agents", "file", "AGENTS.md", "written", "section "+AgentsSectionID)
	return nil
}
