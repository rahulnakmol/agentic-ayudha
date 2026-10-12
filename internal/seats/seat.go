package seats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"
	"github.com/pelletier/go-toml/v2"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/hosts"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// Repository files `aayu seat` writes. They are protected paths, so they reach
// the default branch only through a reviewed pull request.
const (
	ConfigPath      = ".aayudha/config.toml"
	AgentsPath      = "AGENTS.md"
	OnboardingLabel = "seat:onboarding"
	LedgerLog       = "seats/changes.jsonl"
)

// codeownersPaths are where GitHub looks for CODEOWNERS, in its order.
var codeownersPaths = []string{".github/CODEOWNERS", "CODEOWNERS", "docs/CODEOWNERS"}

// Policies `seat` reports.
const (
	PolicySolo     = "solo"      // one human: protected paths merge through the recorded admin bypass
	PolicyTwoHuman = "two-human" // native CODEOWNER review with one approval; no gate, no bypass
	PolicyPending  = "two-human pending"
	PolicyNA       = "n/a"
)

// Errors.
var (
	ErrUsage = errors.New("seat")
	ErrAdmin = errors.New("changing the solo-mode ruleset needs repository administration rights")
)

// Seat is one run of `aayu seat add|remove`.
type Seat struct {
	GH        *github.Client
	Repo      string // owner/name
	Root      string // checkout root
	Default   string // default branch
	Paired    map[verbs.SeatID]string
	BotLogins []string
	Charter   func(id verbs.SeatID) ([]byte, error)
	Ledger    func(ctx context.Context, c ledger.Commit) (string, error)
	Actor     string // login of the human running it, for the ledger record
	Now       func() time.Time
}

// File is one repository file the run touched.
type File struct {
	Path   string `json:"path"`
	Status string `json:"status"` // written, unchanged, planned
}

// Result is what `seat add|remove` reports.
type Result struct {
	Action       string   `json:"action"`
	Seat         string   `json:"seat"`
	Human        string   `json:"human"`
	Files        []File   `json:"files"`
	Issue        int      `json:"onboarding_issue,omitempty"`
	Humans       []string `json:"humans"`
	Policy       string   `json:"policy"`
	PolicyNote   string   `json:"policy_note,omitempty"`
	Ruleset      string   `json:"ruleset"` // unchanged, updated, planned, absent
	Reassigned   []int    `json:"reassigned,omitempty"`
	LedgerCommit string   `json:"ledger_commit,omitempty"`
	DryRun       bool     `json:"dry_run,omitempty"`
}

func (s *Seat) dry() bool { return s.GH.DryRun() }

// CodeownersPath is the CODEOWNERS file in use at root, or .github/CODEOWNERS.
func CodeownersPath(root string) string {
	for _, p := range codeownersPaths {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(p))); err == nil {
			return p
		}
	}
	return codeownersPaths[0]
}

func (s *Seat) read(path string) (string, error) {
	b, err := os.ReadFile(filepath.Join(s.Root, filepath.FromSlash(path)))
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	return string(b), err
}

func (s *Seat) write(res *Result, path, content string) error {
	old, err := s.read(path)
	if err != nil {
		return err
	}
	f := File{Path: path, Status: "unchanged"}
	if old != content {
		f.Status = "planned"
		if !s.dry() {
			full := filepath.Join(s.Root, filepath.FromSlash(path))
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
				return err
			}
			f.Status = "written"
		}
	}
	res.Files = append(res.Files, f)
	return nil
}

// Add pairs seat with human (WP5.1).
func (s *Seat) Add(ctx context.Context, seat verbs.SeatID, human string) (Result, error) {
	res := Result{Action: "add", Seat: string(seat), Human: human, DryRun: s.dry()}
	u, _, err := s.GH.REST.Users.Get(ctx, human)
	if err != nil {
		return res, fmt.Errorf("GitHub user %s: %w", human, err)
	}
	if u.GetType() != "User" || slices.ContainsFunc(s.BotLogins, func(b string) bool { return strings.EqualFold(b, human) }) {
		return res, fmt.Errorf("%w: %s is a %s or a configured bot login; a seat pairs with a human", ErrUsage, human, u.GetType())
	}
	if cur := s.Paired[seat]; cur != "" && !strings.EqualFold(cur, human) {
		return res, fmt.Errorf("%w: %s is already paired with %s; run `aayu seat remove %s` first", ErrUsage, seat, cur, seat)
	}
	s.Paired[seat] = human
	changed, err := s.files(&res, seat, human)
	if err != nil {
		return res, err
	}
	if res.Issue, err = s.onboard(ctx, seat, human); err != nil {
		return res, err
	}
	if err := s.policy(ctx, &res); err != nil {
		return res, err
	}
	if changed {
		res.LedgerCommit, err = s.record(ctx, res)
	}
	return res, err
}

// Remove returns seat to solo, hands its human's open stories back to the
// twin, and reverses the solo-mode switch when one human remains.
func (s *Seat) Remove(ctx context.Context, seat verbs.SeatID) (Result, error) {
	human := s.Paired[seat]
	res := Result{Action: "remove", Seat: string(seat), Human: human, DryRun: s.dry()}
	delete(s.Paired, seat)
	// Loosen first: with one human left, the founder must be able to merge the
	// CODEOWNERS change through the solo-mode bypass.
	if err := s.policy(ctx, &res); err != nil {
		return res, err
	}
	changed, err := s.files(&res, seat, "")
	if err != nil {
		return res, err
	}
	stillSeated := false
	for _, h := range s.Paired {
		stillSeated = stillSeated || strings.EqualFold(h, human)
	}
	if human != "" && !stillSeated {
		if res.Reassigned, err = s.reassign(ctx, human, TwinOwner(seat)); err != nil {
			return res, err
		}
	}
	if changed || len(res.Reassigned) > 0 {
		res.LedgerCommit, err = s.record(ctx, res)
	}
	return res, err
}

// files writes [seats.<seat>], the CODEOWNERS block, and the AGENTS.md RACI
// section, and reports whether config or CODEOWNERS changed.
func (s *Seat) files(res *Result, seat verbs.SeatID, human string) (bool, error) {
	cfg, err := s.read(ConfigPath)
	if err != nil {
		return false, err
	}
	next := SetSeat(cfg, seat, human)
	var probe map[string]any
	if err := toml.Unmarshal([]byte(next), &probe); err != nil {
		return false, fmt.Errorf("%s: would not parse after the change: %w", ConfigPath, err)
	}
	if err := s.write(res, ConfigPath, next); err != nil {
		return false, err
	}
	coPath := CodeownersPath(s.Root)
	co, err := s.read(coPath)
	if err != nil {
		return false, err
	}
	nextCO := RenderCodeowners(co, s.Paired, s.BotLogins)
	if err := s.write(res, coPath, nextCO); err != nil {
		return false, err
	}
	agents, err := s.read(AgentsPath)
	if err != nil {
		return false, err
	}
	if err := s.write(res, AgentsPath, string(hosts.RenderAgentsMD([]byte(agents), AgentsSection()))); err != nil {
		return false, err
	}
	res.Humans = Humans(nextCO, s.BotLogins)
	return next != cfg || nextCO != co, nil
}

// remoteCodeowners reads CODEOWNERS on the default branch ("" when absent).
func (s *Seat) remoteCodeowners(ctx context.Context) (string, error) {
	owner, name, _ := github.SplitRepo(s.Repo)
	f, _, _, err := s.GH.REST.Repositories.GetContents(ctx, owner, name, CodeownersPath(s.Root), &gh.RepositoryContentGetOptions{Ref: s.Default})
	if github.NotFound(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return f.GetContent()
}

// policy reconciles the solo-mode ruleset (WP0.1, D2) with the humans named
// both in the checkout's CODEOWNERS (res.Humans or the rendered block) and on
// the default branch. Second-human review switches on only when both name two
// humans: until the CODEOWNERS change that makes the second human a code owner
// has merged (through the solo-mode bypass), no one could approve it under
// native review. It switches off as soon as either names one human.
func (s *Seat) policy(ctx context.Context, res *Result) error {
	local := Humans(RenderCodeowners(must(s.read(CodeownersPath(s.Root))), s.Paired, s.BotLogins), s.BotLogins)
	res.Humans = local
	remote, err := s.remoteCodeowners(ctx)
	if err != nil {
		return fmt.Errorf("reading CODEOWNERS on %s: %w", s.Default, err)
	}
	remoteN := len(Humans(remote, s.BotLogins))
	want := len(local) >= 2 && remoteN >= 2
	rs, err := s.gateRuleset(ctx)
	if err != nil {
		return err
	}
	if rs == nil {
		res.Ruleset, res.Policy = "absent", PolicyNA
		res.PolicyNote = "no " + GateRuleset + " ruleset: this repository has no WP0.1 solo-mode gate to switch"
		return nil
	}
	switch {
	case want:
		res.Policy = PolicyTwoHuman
	case len(local) >= 2:
		res.Policy = PolicyPending
		res.PolicyNote = fmt.Sprintf("merge the %s change to %s (a solo-mode bypass merge), then rerun this command to require one approval with native CODEOWNER review and remove the %s gate and the admin bypass",
			CodeownersPath(s.Root), s.Default, GateCheck)
	default:
		res.Policy = PolicySolo
	}
	if TwoHuman(rs) == want {
		res.Ruleset = "unchanged"
		return nil
	}
	body := SwitchRuleset(rs, want)
	id := int64(rs["id"].(float64))
	path := fmt.Sprintf("/repos/%s/rulesets/%d", s.Repo, id)
	rec, err := s.GH.Write(ctx, actions.Action{
		Kind: "github.ruleset.update", Target: s.Repo, Seat: string(verbs.Founder),
		Request: &actions.Request{Method: http.MethodPut, Path: path},
		Payload: map[string]any{"ruleset": GateRuleset, "second_human_review": want},
	}, func(ctx context.Context) (any, error) {
		req, err := s.GH.REST.NewRequest(ctx, http.MethodPut, strings.TrimPrefix(path, "/"), body)
		if err != nil {
			return nil, err
		}
		resp, err := s.GH.REST.Do(req, nil)
		if resp != nil && resp.StatusCode == http.StatusForbidden {
			err = fmt.Errorf("%w (a classic token with repo scope from an admin, or a fine-grained token with Administration: write): %w", ErrAdmin, err)
		}
		return id, err
	})
	if err != nil {
		return err
	}
	res.Ruleset = "updated"
	if rec.Status == actions.StatusPlanned {
		res.Ruleset = "planned"
	}
	return nil
}

func must(s string, _ error) string { return s }

// gateRuleset returns the solo-mode ruleset with its rules, or nil.
func (s *Seat) gateRuleset(ctx context.Context) (map[string]any, error) {
	get := func(path string, out any) error {
		req, err := s.GH.REST.NewRequest(ctx, http.MethodGet, strings.TrimPrefix(path, "/"), nil)
		if err != nil {
			return err
		}
		_, err = s.GH.REST.Do(req, out)
		return err
	}
	var list []map[string]any
	if err := get("/repos/"+s.Repo+"/rulesets", &list); err != nil {
		return nil, fmt.Errorf("listing rulesets: %w", err)
	}
	for _, r := range list {
		if r["name"] == GateRuleset {
			var full map[string]any
			id, _ := r["id"].(float64)
			if err := get(fmt.Sprintf("/repos/%s/rulesets/%d", s.Repo, int64(id)), &full); err != nil {
				return nil, err
			}
			return full, nil
		}
	}
	return nil, nil
}

// onboard opens the onboarding issue once per seat and human: the seat's
// charter and a digest of how the pod works, addressed to the human.
func (s *Seat) onboard(ctx context.Context, seat verbs.SeatID, human string) (int, error) {
	owner, name, _ := github.SplitRepo(s.Repo)
	marker := fmt.Sprintf("<!-- aayudha:seat-onboarding %s %s -->", seat, strings.ToLower(human))
	existing, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.Issue, *gh.Response, error) {
		return s.GH.REST.Issues.ListByRepo(ctx, owner, name, &gh.IssueListByRepoOptions{State: "all", Labels: []string{OnboardingLabel}, ListOptions: o})
	})
	if err != nil {
		return 0, err
	}
	for _, is := range existing {
		if strings.Contains(is.GetBody(), marker) {
			return is.GetNumber(), nil
		}
	}
	charter, err := s.Charter(seat)
	if err != nil {
		return 0, err
	}
	title := fmt.Sprintf("Seat onboarding: %s (@%s)", verbs.SeatName(seat), human)
	body := marker + "\n" + Onboarding(seat, human, string(charter))
	rec, err := s.GH.Write(ctx, actions.Action{
		Kind: "github.issue.create", Target: s.Repo, Seat: string(verbs.Founder),
		Request: &actions.Request{Method: http.MethodPost, Path: "/repos/" + s.Repo + "/issues"},
		Payload: map[string]any{"title": title, "seat": string(seat), "human": human},
	}, func(ctx context.Context) (any, error) {
		is, _, err := s.GH.REST.Issues.Create(ctx, owner, name, gh.CreateIssueRequest{Title: title, Body: &body, Labels: []string{OnboardingLabel}})
		return is.GetNumber(), err
	})
	if err != nil {
		return 0, err
	}
	n, _ := rec.Result.(int)
	return n, nil
}

// Onboarding is the onboarding issue body: the "how we work" digest, the
// seat's layers and verbs, and its charter.
func Onboarding(seat verbs.SeatID, human, charter string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "@%s, you are now the paired human of the **%s** seat. Its agent twin keeps working; you review what it\n", human, verbs.SeatName(seat))
	b.WriteString("does in your seat's layers and hold the human authorities below.\n\n## How we work\n\n")
	b.WriteString(`- Work is GitHub issues of ten types; a Story or Bug starts only when ` + "`aayu ready`" + ` has posted its Ready snapshot.
- Agents build in checkpointed worktrees and open PRs to ` + "`dev`" + `. Deterministic floors (` + "`verify`" + `) and an adversarial
  inspection gate every merge; no model can override a floor miss.
- A PR merges after a human pass: an approving review from a human who is not the author. CODEOWNERS names you for
  your seat's layers, so GitHub asks for your review there.
- Protected paths (` + "`.github/`" + `, ` + "`skills/`" + `, ` + "`ops/`" + `, ` + "`seats/`" + `, ` + "`CONSTRAINTS.md`" + `, ` + "`.aayudha/config.toml`" + `) always need a human
  CODEOWNER review. With two humans, each reviews the other's changes there.
- Every external write goes through ` + "`aayu`" + ` and its actions log; never write to GitHub with ` + "`gh`" + ` directly.
- ` + "`aayu halt`" + ` stops every agent lane and workflow; ` + "`aayu resume`" + ` clears it.

`)
	b.WriteString(CharterRegion(seat))
	b.WriteString("\n## Charter\n\n")
	_, rest, _ := strings.Cut(strings.TrimSpace(charter), "\n") // the charter's own title duplicates the issue's
	b.WriteString(regionRE.ReplaceAllString(strings.TrimSpace(demote(rest)), ""))
	b.WriteString("\n")
	return b.String()
}

var regionRE = regexp.MustCompile(`(?s)\n*<!-- BEGIN GENERATED: raci -->.*<!-- END GENERATED: raci -->`)

// demote shifts a charter's headings down one level to sit under "## Charter".
func demote(md string) string {
	lines := strings.Split(md, "\n")
	for i, l := range lines {
		if strings.HasPrefix(l, "#") {
			lines[i] = "#" + l
		}
	}
	return strings.Join(lines, "\n")
}

// reassign hands the open Stories and Bugs whose Technical Owner names human
// back to twin: in the body's Technical Owner field and in the Ready snapshot
// comment, so no open story keeps a human owner who has left the seat.
func (s *Seat) reassign(ctx context.Context, human, twin string) ([]int, error) {
	owner, name, _ := github.SplitRepo(s.Repo)
	var out []int
	seen := map[int]bool{}
	for _, label := range []string{"type:story", "type:bug"} {
		issues, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.Issue, *gh.Response, error) {
			return s.GH.REST.Issues.ListByRepo(ctx, owner, name, &gh.IssueListByRepoOptions{State: "open", Labels: []string{label}, ListOptions: o})
		})
		if err != nil {
			return nil, err
		}
		for _, is := range issues {
			n := is.GetNumber()
			if seen[n] || is.IsPullRequest() {
				continue
			}
			seen[n] = true
			touched := false
			if body, ok := ReplaceOwner(is.GetBody(), human, twin); ok {
				_, err := s.GH.Write(ctx, actions.Action{
					Kind: "github.issue.edit", Target: fmt.Sprintf("%s#%d", s.Repo, n), Seat: string(verbs.Founder),
					Request: &actions.Request{Method: http.MethodPatch, Path: fmt.Sprintf("/repos/%s/issues/%d", s.Repo, n)},
					Payload: map[string]any{"technical_owner": twin, "was": "@" + human},
				}, func(ctx context.Context) (any, error) {
					_, _, err := s.GH.REST.Issues.Update(ctx, owner, name, n, gh.UpdateIssueRequest{Body: &body})
					return n, err
				})
				if err != nil {
					return nil, err
				}
				touched = true
			}
			comments, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.IssueComment, *gh.Response, error) {
				return s.GH.REST.Issues.ListComments(ctx, owner, name, n, &gh.IssueListCommentsOptions{ListOptions: o})
			})
			if err != nil {
				return nil, err
			}
			for _, c := range comments {
				if !strings.Contains(c.GetBody(), ready.SnapshotMarker) {
					continue
				}
				body, ok := ReplaceSnapshotOwner(c.GetBody(), human, twin)
				if !ok {
					continue
				}
				id := c.GetID()
				_, err := s.GH.Write(ctx, actions.Action{
					Kind: "github.comment.edit", Target: fmt.Sprintf("%s#%d", s.Repo, n), Seat: string(verbs.Founder),
					Request: &actions.Request{Method: http.MethodPatch, Path: fmt.Sprintf("/repos/%s/issues/comments/%d", s.Repo, id)},
					Payload: map[string]any{"technical_owner": twin, "was": "@" + human},
				}, func(ctx context.Context) (any, error) {
					_, _, err := s.GH.REST.Issues.EditComment(ctx, owner, name, id, &gh.IssueComment{Body: &body})
					return id, err
				})
				if err != nil {
					return nil, err
				}
				touched = true
			}
			if touched {
				out = append(out, n)
			}
		}
	}
	slices.Sort(out)
	return out, nil
}

func mentions(text, human string) bool {
	return regexp.MustCompile(`(?i)(^|[^A-Za-z0-9-])@?` + regexp.QuoteMeta(human) + `($|[^A-Za-z0-9-])`).MatchString(text)
}

var headingRE = regexp.MustCompile(`^#{2,3}\s+(.+?)\s*$`)

// ReplaceOwner rewrites the Technical Owner field of an issue body (the
// issue-form layout: a "### Technical Owner" heading and its value) to twin
// when it names human. It reports whether it changed anything.
func ReplaceOwner(body, human, twin string) (string, bool) {
	lines := strings.Split(body, "\n")
	for i := 0; i < len(lines); i++ {
		m := headingRE.FindStringSubmatch(strings.TrimRight(lines[i], "\r"))
		if m == nil || !strings.EqualFold(m[1], "Technical Owner") {
			continue
		}
		j := i + 1
		for j < len(lines) && !headingRE.MatchString(strings.TrimRight(lines[j], "\r")) {
			j++
		}
		if !mentions(strings.Join(lines[i+1:j], "\n"), human) {
			return body, false
		}
		out := append(append(slices.Clone(lines[:i+1]), "", twin, ""), lines[j:]...)
		if j == len(lines) {
			out = out[:len(out)-1]
		}
		return strings.Join(out, "\n"), true
	}
	return body, false
}

// ReplaceSnapshotOwner rewrites the "| Technical Owner | … |" row of a Ready
// snapshot comment to twin when it names human.
func ReplaceSnapshotOwner(body, human, twin string) (string, bool) {
	lines := strings.Split(body, "\n")
	for i, l := range lines {
		if strings.HasPrefix(l, "| Technical Owner |") && mentions(l, human) {
			lines[i] = "| Technical Owner | " + twin + " |"
			return strings.Join(lines, "\n"), true
		}
	}
	return body, false
}

// record appends the seat change to seats/changes.jsonl on the ledger.
func (s *Seat) record(ctx context.Context, res Result) (string, error) {
	if s.Ledger == nil {
		return "", nil
	}
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	line, err := json.Marshal(map[string]any{
		"time": now().UTC().Format(time.RFC3339), "action": res.Action, "seat": res.Seat, "human": res.Human,
		"by": s.Actor, "humans": res.Humans, "policy": res.Policy, "reassigned": res.Reassigned,
	})
	if err != nil {
		return "", err
	}
	return s.Ledger(ctx, ledger.Commit{
		Message: fmt.Sprintf("seats: %s %s %s", res.Action, res.Seat, res.Human),
		Append:  map[string][]byte{LedgerLog: append(line, '\n')},
	})
}

// Row is one line of `seat list`.
type Row struct {
	Seat   string   `json:"seat"`
	Name   string   `json:"name"`
	Mode   string   `json:"mode"`
	Human  string   `json:"human,omitempty"`
	Layers []string `json:"layers"`
}

// List returns every seat with its mode and human.
func List(paired map[verbs.SeatID]string) []Row {
	var out []Row
	for _, s := range verbs.Seats {
		r := Row{Seat: string(s.ID), Name: s.Name, Mode: "solo", Human: paired[s.ID], Layers: []string{}}
		if r.Human != "" {
			r.Mode = "paired"
		}
		for _, l := range verbs.LayersOwned(s.ID) {
			r.Layers = append(r.Layers, string(l.ID))
		}
		out = append(out, r)
	}
	return out
}
