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

// Media extensions tracked by LFS on a private repository's ledger (V3).
var mediaExts = proof.MediaExts

// LedgerRuleset is the name of the ruleset init creates when no existing
// ruleset already protects the ledger branch.
const LedgerRuleset = "aayudha-ledger"

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
		files[".lfsconfig"] = "# Proof media on this ledger are stored in GitHub LFS (V3: private repository).\n[lfs]\n\tlocksverify = false\n"
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
	b.WriteString("# Proof media are stored in GitHub LFS on private repositories (V3).\n")
	for _, e := range mediaExts {
		fmt.Fprintf(&b, "*.%s filter=lfs diff=lfs merge=lfs -text\n", e)
	}
	return b.String()
}

func ledgerReadme(r Repo) string {
	media := "Proof media are stored in GitHub LFS (private repository)."
	if !r.Private {
		media = "This repository is public: proof media are never stored here or in Git LFS; they go to the private bucket named by `[proof] media` (V3), and git holds `pack.json` and hashes only."
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
		} `json:"ref_name"`
	} `json:"conditions"`
	Rules []struct {
		Type string `json:"type"`
	} `json:"rules"`
}

func (d rulesetDetail) protectsLedger() bool {
	if d.Target != "branch" || d.Enforcement != "active" {
		return false
	}
	if !slices.Contains(d.Conditions.RefName.Include, "refs/heads/ledger") {
		return false
	}
	var types []string
	for _, r := range d.Rules {
		types = append(types, r.Type)
	}
	return slices.Contains(types, "deletion") && slices.Contains(types, "non_fast_forward")
}

func (in *Init) rest(ctx context.Context, method, path string, body, out any) error {
	req, err := in.GH.REST.NewRequest(ctx, method, strings.TrimPrefix(path, "/"), body)
	if err != nil {
		return err
	}
	_, err = in.GH.REST.Do(req, out)
	return err
}

func (in *Init) ruleset(ctx context.Context) error {
	r := in.Repo
	var list []rulesetDetail
	if err := in.rest(ctx, http.MethodGet, "/repos/"+r.FullName+"/rulesets", nil, &list); err != nil {
		return err
	}
	o, recorded := in.State.Object("ruleset", LedgerRuleset)
	var match *rulesetDetail
	for _, s := range list {
		if recorded && strconv.FormatInt(s.ID, 10) == o.ID {
			in.note("ruleset", "ruleset", s.Name, "unchanged", "")
			return nil
		}
		var d rulesetDetail
		if err := in.rest(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/rulesets/%d", r.FullName, s.ID), nil, &d); err != nil {
			return err
		}
		if match == nil && d.protectsLedger() {
			match = &d
		}
	}
	if match != nil {
		in.State.Put(Object{Kind: "ruleset", Name: LedgerRuleset, ID: strconv.FormatInt(match.ID, 10), Adopted: true})
		in.note("ruleset", "ruleset", match.Name, "adopted", "already protects refs/heads/ledger; added nothing alongside it")
		return nil
	}
	body := map[string]any{
		"name": LedgerRuleset, "target": "branch", "enforcement": "active",
		"conditions": map[string]any{"ref_name": map[string]any{"include": []string{"refs/heads/ledger"}, "exclude": []string{}}},
		"rules":      []map[string]any{{"type": "deletion"}, {"type": "non_fast_forward"}, {"type": "update"}},
		// Day one (Appendix B D1): the repository admin role writes the ledger
		// until init --identity adds the bot App as the bypass actor.
		"bypass_actors": []map[string]any{{"actor_id": 5, "actor_type": "RepositoryRole", "bypass_mode": "always"}},
	}
	var created rulesetDetail
	rec, err := in.GH.Write(ctx, actions.Action{
		Kind: "github.ruleset.create", Target: r.FullName, IdempotencyKey: in.key("ruleset", LedgerRuleset),
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
	in.recordCreated(rec, Object{Kind: "ruleset", Name: LedgerRuleset, ID: strconv.FormatInt(created.ID, 10)}, "ruleset")
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
		b.WriteString("\n[proof]\nmedia = \"\" # required: private bucket. Public repository: proof media never go to Git LFS (V3); `proof push` refuses media until this is set.\n")
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
	if err := in.writeFile("config", ".aayudha/.gitignore", []byte("# Local aayudha runtime state (written by aayu init); never committed.\ntelemetry/\nstate/\nledger-outbox/\nproof/\nworktrees/\nconfig.local.toml\n")); err != nil {
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
	b.WriteString("- Every GitHub write goes through `aayu`; agents never write with `gh` directly.")
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
