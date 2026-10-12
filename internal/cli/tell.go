package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/tell"
)

func init() { register("tell", runTell) }

// Publication is tell/<kind>/<campaign>.json on the ledger: what was
// published, by whom, and where.
type Publication struct {
	Repo        string   `json:"repo"`
	Kind        string   `json:"kind"`
	Campaign    string   `json:"campaign"`
	Tag         string   `json:"tag,omitempty"`
	SHA256      string   `json:"sha256"`
	Cited       []int    `json:"cited_features"`
	UTM         tell.UTM `json:"utm"`
	Copy        string   `json:"copy"` // ledger path of the published Markdown
	Release     string   `json:"github_release,omitempty"`
	PublishedBy string   `json:"published_by"`
	PublishedAt string   `json:"published_at"`
}

type tellReport struct {
	Kind      string         `json:"kind"`
	Brand     tell.Brand     `json:"brand"`
	UTM       tell.UTM       `json:"utm"`
	Proven    []tell.Feature `json:"proven_features"`
	Unproven  []tell.Feature `json:"unproven_features"`
	ProofNote string         `json:"proof_note,omitempty"`
	Draft     *tell.Draft    `json:"draft,omitempty"`
	Problems  []tell.Problem `json:"problems,omitempty"`
	CopyPath  string         `json:"copy_path,omitempty"`
	Published *Publication   `json:"published,omitempty"`
	DryRun    bool           `json:"dry_run,omitempty"`
}

var campaignRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$`)

// runTell gives the host the brief (tone, proven Features, UTM tags), or
// checks the host's draft: every link to a work item names a Feature whose
// shipped stories all have authoritative packs, no avoided word, and every
// outbound link tagged. --publish (a human's own account only) lands the copy
// on the ledger, and release notes on the tag's GitHub Release, through the
// actions log.
func runTell(ctx context.Context, e *Env, in Values) error {
	kind := in.String("kind")
	publish := in.Bool("publish")
	if publish {
		if err := e.refuseNonHuman("tell --publish"); err != nil {
			return err
		}
		if !in.Has("from") {
			return Errorf(ExitUsage, "tell --publish needs --from: publish only a checked draft")
		}
	}
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	root := config.RepoRoot(e.Dir)
	brand, err := tell.LoadBrand(root)
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	campaign := in.String("campaign")
	if campaign == "" {
		campaign = in.String("tag")
	}
	if campaign == "" {
		campaign = kind + "-" + e.Now().UTC().Format(time.DateOnly)
	}
	if !campaignRE.MatchString(campaign) {
		return Errorf(ExitUsage, "tell: campaign %q: want letters, digits, '.', '_', or '-' (at most 100)", campaign)
	}
	var draft []byte
	if in.Has("from") {
		if draft, err = readTellDraft(in.String("from")); err != nil {
			return err
		}
	}
	_, name, _ := github.SplitRepo(repo)
	rep := tellReport{Kind: kind, Brand: brand, UTM: tell.UTM{Source: name, Medium: kind, Campaign: campaign}}

	c, tok, err := e.HumanGitHub(ctx)
	if err != nil {
		return err
	}
	var me identity.User
	if publish { // before anything is written, even locally
		if me, err = e.humanAccount(ctx, tok, "tell --publish"); err != nil {
			return err
		}
	}
	var proven map[int]tell.Feature
	proven, rep.Unproven, rep.ProofNote, err = e.provenFeatures(ctx, c, repo)
	if err != nil {
		return err
	}
	rep.Proven = sortedFeatures(proven)

	if draft != nil {
		d, probs := tell.Check(string(draft), repo, proven, brand, rep.UTM)
		rep.Draft, rep.Problems = &d, probs
		if len(probs) == 0 {
			rep.CopyPath = filepath.Join(e.stateDir(), "tell", kind, campaign+".md")
			_ = os.MkdirAll(filepath.Dir(rep.CopyPath), 0o755)
			if err := os.WriteFile(rep.CopyPath, []byte(d.Copy), 0o644); err != nil {
				return err
			}
			if publish {
				rep.Published, err = e.publishTell(ctx, c, tok, me.Login, repo, kind, campaign, in.String("tag"), d, rep.UTM)
				if err != nil {
					return err
				}
				rep.DryRun = c.DryRun()
			}
		}
	}
	if err := e.Emit(rep, func(out io.Writer) error { return tellText(out, rep, draft != nil) }); err != nil {
		return err
	}
	if len(rep.Problems) > 0 {
		return Errorf(ExitBlocked, "tell %s: the draft is refused (%d problem(s)); copy claims only proven Features, in the brand's words", kind, len(rep.Problems))
	}
	return nil
}

func tellText(out io.Writer, rep tellReport, checked bool) error {
	if !checked {
		fmt.Fprintf(out, "tell %s brief\n\nVoice (%s): %s\nTone: %s\nNever use: %s\n\n", rep.Kind, rep.Brand.Source, rep.Brand.Voice,
			strings.Join(rep.Brand.Tone, ", "), strings.Join(rep.Brand.Avoid, ", "))
		fmt.Fprintln(out, "Features you may claim (every shipped story has an authoritative pack); link them as #N:")
		if len(rep.Proven) == 0 {
			fmt.Fprintln(out, "  none")
		}
		for _, f := range rep.Proven {
			fmt.Fprintf(out, "  #%d %s\n", f.Number, f.Title)
		}
		if len(rep.Unproven) > 0 {
			fmt.Fprintf(out, "Not yet provable (do not claim or link): %d Feature(s)\n", len(rep.Unproven))
		}
		if rep.ProofNote != "" {
			fmt.Fprintln(out, rep.ProofNote)
		}
		fmt.Fprintf(out, "\nLinks are tagged utm_source=%s&utm_medium=%s&utm_campaign=%s. Write the draft as Markdown, then: aayu tell %s --from <draft.md>\n",
			rep.UTM.Source, rep.UTM.Medium, rep.UTM.Campaign, rep.Kind)
		return nil
	}
	for _, p := range rep.Problems {
		fmt.Fprintf(out, "refused: %s\n", p)
	}
	if len(rep.Problems) > 0 {
		return nil
	}
	fmt.Fprintf(out, "tell %s: draft passes; cites %s; %d link(s) tagged; copy %s\n", rep.Kind, refsList(rep.Draft.Cited), rep.Draft.Tagged, rep.CopyPath)
	if p := rep.Published; p != nil {
		verb := "published"
		if rep.DryRun {
			verb = "would publish"
		}
		fmt.Fprintf(out, "%s as %s by %s", verb, p.Copy, p.PublishedBy)
		if p.Release != "" {
			fmt.Fprintf(out, "; release notes set on %s", p.Release)
		}
		fmt.Fprintln(out)
	}
	return nil
}

func refsList(ns []int) string {
	if len(ns) == 0 {
		return "none"
	}
	parts := make([]string, len(ns))
	for i, n := range ns {
		parts[i] = fmt.Sprintf("#%d", n)
	}
	return strings.Join(parts, ", ")
}

func readTellDraft(p string) ([]byte, error) {
	fi, err := os.Stat(p)
	if err != nil {
		return nil, Wrap(ExitUsage, fmt.Errorf("tell --from: %w", err))
	}
	if fi.IsDir() || fi.Size() > 1<<20 {
		return nil, Errorf(ExitUsage, "tell --from %s: want a Markdown file of at most 1 MiB", p)
	}
	return os.ReadFile(p)
}

func sortedFeatures(m map[int]tell.Feature) []tell.Feature {
	out := make([]tell.Feature, 0, len(m))
	for _, f := range m {
		out = append(out, f)
	}
	sortFeatures(out)
	return out
}

func sortFeatures(fs []tell.Feature) {
	slices.SortFunc(fs, func(a, b tell.Feature) int { return a.Number - b.Number })
}

// provenFeatures splits the repository's Features: proven when at least one
// story or bug under it is done and every done one is covered by an
// authoritative pack (status proof coverage k/k, k > 0). Outside a checkout
// the ledger cannot be read, so nothing is proven.
func (e *Env) provenFeatures(ctx context.Context, c *github.Client, repo string) (map[int]tell.Feature, []tell.Feature, string, error) {
	owner, name, _ := github.SplitRepo(repo)
	issues, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.Issue, *gh.Response, error) {
		return c.REST.Issues.ListByRepo(ctx, owner, name, &gh.IssueListByRepoOptions{State: "all", Labels: []string{"type:feature"}, ListOptions: o})
	})
	if err != nil {
		return nil, nil, "", err
	}
	w, err := e.walker(ctx, c, repo)
	if err != nil {
		return nil, nil, "", err
	}
	note := ""
	if w.Proof == nil {
		note = "No checkout of the repository, so the ledger's packs cannot be read and no Feature is proven."
	}
	proven := map[int]tell.Feature{}
	var unproven []tell.Feature
	for _, is := range issues {
		if is.IsPullRequest() {
			continue
		}
		f := tell.Feature{Number: is.GetNumber(), Title: is.GetTitle(), URL: fmt.Sprintf("https://github.com/%s/issues/%d", repo, is.GetNumber())}
		r, err := w.Roll(ctx, f.Number)
		if err != nil {
			return nil, nil, "", err
		}
		if r.ProofDone > 0 && r.ProofCovered == r.ProofDone {
			proven[f.Number] = f
		} else {
			unproven = append(unproven, f)
		}
	}
	sortFeatures(unproven)
	return proven, unproven, note, nil
}

// refuseNonHuman refuses a human-only act (tell --publish, release) for agent
// lanes and in CI, where the token is a workflow or bot token.
func (e *Env) refuseNonHuman(what string) error {
	if e.Agent() {
		return Errorf(ExitBlocked, "%s is a human's act; it is refused for agents (AAYU_AGENT=1)", what)
	}
	if e.CI {
		return Errorf(ExitBlocked, "%s is a human's act; it is refused in CI, where the token is a workflow or bot token", what)
	}
	return nil
}

// humanAccount checks that tok is a human's own account: not a GitHub bot or
// App and not one of [identity] bot_logins (trust model §0.1).
func (e *Env) humanAccount(ctx context.Context, tok identity.Token, what string) (identity.User, error) {
	me, err := identity.WhoAmI(ctx, e.HTTP, e.githubAPI(), tok)
	if err != nil {
		return me, Wrap(ExitBlocked, fmt.Errorf("%s needs a human's own account: %w", what, err))
	}
	l, err := e.Config()
	if err != nil {
		return me, err
	}
	if identity.IsBot(identity.Account{Login: me.Login, Type: me.Type}, l.Config.Identity.BotLogins) {
		return me, Errorf(ExitBlocked, "%s: %s is a bot identity; it needs a human's own account", what, me.Login)
	}
	return me, nil
}

// tellMarker delimits the notes tell writes into a GitHub Release body.
const tellMarker = "<!-- aayudha:tell -->"

// publishTell lands checked copy on the ledger (tell/<kind>/<campaign>.md and
// its .json record) and, for release notes with a tag, replaces tell's part of
// that GitHub Release's notes. login is the human account humanAccount
// checked; the whole publication is one tell.publish action in the log.
func (e *Env) publishTell(ctx context.Context, c *github.Client, tok identity.Token, login, repo, kind, campaign, tag string, d tell.Draft, utm tell.UTM) (*Publication, error) {
	sum := sha256.Sum256([]byte(d.Copy))
	base := path.Join("tell", kind, campaign)
	pub := &Publication{Repo: repo, Kind: kind, Campaign: campaign, Tag: tag, SHA256: hex.EncodeToString(sum[:]), Cited: append([]int{}, d.Cited...), UTM: utm,
		Copy: base + ".md", PublishedBy: login, PublishedAt: e.Now().UTC().Format(time.RFC3339)}
	log, err := e.Actions()
	if err != nil {
		return nil, err
	}
	_, err = log.Do(ctx, actions.Action{Kind: "tell.publish", Target: repo + "@ledger:" + pub.Copy, Identity: "human",
		Payload: map[string]any{"kind": kind, "campaign": campaign, "tag": tag, "sha256": pub.SHA256, "cited": pub.Cited, "login": login}},
		func(ctx context.Context) (actions.Outcome, error) {
			if kind == "release" && tag != "" {
				u, err := setReleaseNotes(ctx, c, repo, tag, d.Copy)
				if err != nil {
					return actions.Outcome{}, err
				}
				pub.Release = u
			}
			rec, _ := json.MarshalIndent(pub, "", "  ")
			w, err := e.Ledger(ctx, tok, "human", "")
			if err != nil {
				return actions.Outcome{}, err
			}
			sha, err := w.Write(ctx, ledger.Commit{Message: fmt.Sprintf("tell: publish %s %s (%s)", kind, campaign, login),
				Files: map[string][]byte{pub.Copy: []byte(d.Copy), base + ".json": append(rec, '\n')}})
			return actions.Outcome{Result: map[string]any{"ledger_commit": sha, "github_release": pub.Release}}, err
		})
	if err != nil {
		return nil, err
	}
	return pub, nil
}

// setReleaseNotes replaces tell's marked part of the GitHub Release for tag,
// appending it when absent. A tag without a GitHub Release (a rolling-* tag,
// which goreleaser ignores) is left alone and returns "".
func setReleaseNotes(ctx context.Context, c *github.Client, repo, tag, notes string) (string, error) {
	owner, name, _ := github.SplitRepo(repo)
	rel, _, err := c.REST.Repositories.GetReleaseByTag(ctx, owner, name, tag)
	if github.NotFound(err) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	block := tellMarker + "\n" + strings.TrimSpace(notes) + "\n" + tellMarker
	body := strings.TrimLeft(strings.TrimRight(rel.GetBody(), "\n")+"\n\n"+block+"\n", "\n")
	if before, rest, ok := strings.Cut(rel.GetBody(), tellMarker); ok {
		_, after, _ := strings.Cut(rest, tellMarker)
		body = before + block + after
	}
	_, err = c.Write(ctx, actions.Action{Kind: "github.release.notes", Target: repo + "@" + tag,
		Request: &actions.Request{Method: http.MethodPatch, Path: fmt.Sprintf("/repos/%s/releases/%d", repo, rel.GetID())}, Payload: map[string]any{"tag": tag}},
		func(ctx context.Context) (any, error) {
			_, _, err := c.REST.Repositories.UpdateRelease(ctx, owner, name, rel.GetID(), gh.UpdateReleaseRequest{Body: &body})
			return nil, err
		})
	return rel.GetHTMLURL(), err
}
