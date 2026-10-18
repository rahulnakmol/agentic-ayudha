package cli

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/decide"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// Approval batches (Decision #488, Story #481). The human-only steps an
// agent starts together go into one numbered comment on the first item's
// issue. Where agents act only as a bot and a recorded approver's GitHub
// credential is in no agent's reach, the approver answers the batch with a
// reply that names the batch by its hash (`/aayu approve all <hash>`,
// `/aayu approve 1 3 <hash>`, `/aayu skip 2 <hash>`), which the
// aayudha-approve workflow verifies and applies as the bot (`aayu inbox
// --ci`). Everywhere else, and for one-way doors under the default
// [approvals] device_for, one device code covers the whole batch.

// Batch comment markers. A batch is open until a result comment with its hash
// answers it, or for batchOpenFor.
const (
	batchMarker       = "<!-- aayudha:batch "
	batchDataMarker   = "<!-- aayudha:batch-data "
	batchResultMarker = "<!-- aayudha:batch-result "
	batchMax          = 10
	batchOpenFor      = 7 * 24 * time.Hour
)

// Batch modes.
const (
	batchByComment = "comment" // an approver's reply approves it
	batchByDevice  = "device"  // one device code approves it
)

// batchItem is one human-only step in a batch. The hash covers the fields
// with JSON names; the rest is what the comment shows.
type batchItem struct {
	Kind   string `json:"kind"` // decision
	Issue  int    `json:"issue"`
	N      int    `json:"n"`      // the option's number in the Decision (decide.Options)
	Option string `json:"option"` // its name, as decide --accept records it
	OneWay bool   `json:"one_way"`
	// Other is set when Option is not the Decision's recommendation: like a
	// one-way door, it is confirmed explicitly (an approve reply that names
	// the item, or the device code).
	Other    bool   `json:"not_recommended"`
	BodyHash string `json:"body_hash"` // the issue's title and body when the batch was made: a later edit voids the approval
	Effect   string `json:"-"`         // plain words, for the terminal and the actions log
	Heading  string `json:"-"`         // Effect with the Decision's own text escaped, for the comment
	Door     string `json:"-"`
	Detail   string `json:"-"`
}

// batch is a group of human-only steps approved together.
type batch struct {
	Repo    string      `json:"repo"`
	Issue   int         `json:"issue"` // the thread the batch comment is on
	Command string      `json:"command"`
	Posted  string      `json:"posted"` // when the batch was made: each batch, even of the same command, has its own hash
	Mode    string      `json:"mode"`
	Items   []batchItem `json:"items"`
	Why     string      `json:"-"` // why the device code, in device mode
	comment int64       // the batch comment aayu posted, once it has (device mode updates exactly that one)
	Mention []string    `json:"-"` // who is asked
}

// decisionHash covers what the batch shows of a Decision, its title and
// body, and its parent's type, which decides its authority (labels are read
// again when the reply arrives).
func decisionHash(d decide.Decision) string {
	sum := sha256.Sum256([]byte(d.Title + "\x00" + d.Body + "\x00" + d.ParentType))
	return hex.EncodeToString(sum[:8])
}

// data is the batch's canonical record; hash is the short hash of it, shown
// in the comment and checked when the batch is approved.
func (b *batch) data() []byte {
	out, _ := json.Marshal(b)
	return out
}

func (b *batch) hash() string {
	sum := sha256.Sum256(b.data())
	return hex.EncodeToString(sum[:6])
}

func (b *batch) marker() string { return batchMarker + b.hash() + " -->" }

// batchDecision describes accepting option o of Decision d in plain words.
func batchDecision(d decide.Decision, o workitems.Option) batchItem {
	title := strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(d.Title, "Decision:")), ".")
	it := batchItem{Kind: "decision", Issue: d.Number, N: o.N, Option: o.Name, OneWay: !d.Reversible() || d.Waiver(), Other: !o.Recommended, BodyHash: decisionHash(d),
		Effect: fmt.Sprintf("Accepts option %d (%s) of Decision #%d: %s.", o.N, termText(o.Name), d.Number, termText(title))}
	it.Heading = fmt.Sprintf("Accepts option %d (%s) of Decision #%d: %s.", o.N, mdText(o.Name), d.Number, mdText(title))
	if it.Other {
		it.Effect += " This is not the recommendation."
		it.Heading += " This is not the recommendation."
	}
	// Accepting also closes every Decision it supersedes (acceptDecision).
	links, _ := workitems.ParseLinks(d.Item.Fields[workitems.FieldLinks])
	for _, lk := range links {
		if lk.Kind == "supersedes" {
			also := fmt.Sprintf(" Also closes #%d as superseded.", lk.Number)
			it.Effect += also
			it.Heading += also
		}
	}
	switch {
	case d.Waiver():
		it.Door = "One-way door: a floor waiver is never reversible."
	case it.OneWay:
		it.Door = "One-way door: the Decision is marked not reversible."
	default:
		it.Door = "Two-way door: a later Decision can change it."
	}
	var b strings.Builder
	if os, _ := decide.Options(d.Item); len(os) > 0 {
		b.WriteString("Options:\n")
		for _, o := range os {
			b.WriteString(clip(optionLine(o), 300) + "\n")
		}
	}
	if rec := clip(strings.TrimSpace(d.Item.Fields["recommendation"]), 500); rec != "" {
		b.WriteString("Recommendation: " + strings.ReplaceAll(rec, "\n", " ") + "\n")
	}
	it.Detail = b.String() // untrusted issue text: render shows it only inside a code fence
	return it
}

// named reports whether item it needs to be named in the reply: a pick
// that is not the recommendation, or a one-way door, is never approved by
// `all` or `skip`.
func (it batchItem) named() bool { return it.Other || it.OneWay }

// fence shows untrusted text (an issue's options, a command line) as an inert
// code block: no Markdown, HTML, or mention in it takes effect, and no
// backtick run in it can close the fence.
func fence(s string) string {
	run, longest := 0, 0
	for _, r := range s {
		if r == '`' {
			run++
			longest = max(longest, run)
		} else {
			run = 0
		}
	}
	f := strings.Repeat("`", max(3, longest+1))
	return f + "text\n" + strings.TrimRight(s, "\n") + "\n" + f + "\n"
}

// mdEscape makes untrusted text (a Decision's title, an option) plain in
// Markdown: no HTML, emphasis, links, or code spans.
var mdEscape = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\\", "\\\\", "`", "\\`", "*", "\\*", "_", "\\_",
	"[", "\\[", "]", "\\]", "#", "\\#", "|", "\\|", "~", "\\~", "!", "\\!", "@", "@\u200b")

func mdText(s string) string { return mdEscape.Replace(termText(s)) }

// termText makes untrusted text safe to show: control and format characters
// (escape sequences, carriage returns, bidi overrides) show as U+FFFD instead
// of moving the cursor or reordering what the person reads.
func termText(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) { // Cf: bidi overrides and isolates, zero-width marks
			return '\uFFFD'
		}
		return r
	}, s)
}

// batchStatus is what the batch comment says under its items: how to
// approve it, or what happened.
type batchStatus struct {
	Code    *deviceCodeReply
	Expires time.Time
	Done    string // a final line that replaces the instructions
	Next    string
}

// render is the batch comment.
func (b *batch) render(st batchStatus) string {
	var w strings.Builder
	w.WriteString(b.marker() + "\n")
	who := ""
	for _, m := range b.Mention {
		who += "@" + strings.TrimPrefix(m, "@") + " "
	}
	noun := "step waits"
	if len(b.Items) > 1 {
		noun = "steps wait"
	}
	fmt.Fprintf(&w, "%s**%d human-only %s for your approval** (batch `%s`).\n\n", who, len(b.Items), noun, b.hash())
	for i, it := range b.Items {
		fmt.Fprintf(&w, "%d. **%s** %s\n", i+1, it.Heading, it.Door)
		if it.Detail != "" {
			w.WriteString("\n")
			for _, line := range strings.Split(strings.TrimRight(fence(it.Detail), "\n"), "\n") {
				w.WriteString("   " + line + "\n")
			}
		}
		w.WriteString("\n")
	}
	switch {
	case st.Done != "":
		w.WriteString("**" + st.Done + "**\n")
		if st.Next != "" {
			w.WriteString("\nNext: " + st.Next + "\n")
		}
	case b.Mode == batchByComment:
		h := b.hash()
		w.WriteString("**To approve, reply on this issue** with one line that ends in this batch's hash:\n\n")
		fmt.Fprintf(&w, "- `/aayu approve all %s`\n", h)
		if len(b.Items) > 1 {
			fmt.Fprintf(&w, "- `/aayu approve 1 3 %s` approves only those items\n- `/aayu skip 2 %s` approves all but those items\n", h, h)
		}
		for i, it := range b.Items {
			if it.named() {
				why := "is a one-way door"
				if it.Other {
					why = "is not the recommendation"
				}
				fmt.Fprintf(&w, "\nItem %d %s, so `all` and `skip` leave it out; to approve it, name it: `/aayu approve %d %s`.\n", i+1, why, i+1, h)
			}
		}
		w.WriteString("\nItem numbers are this batch's; each item names the Decision's own option number. Only a reply from a recorded approver counts, it must name this batch, and it must not be edited. Nothing happens until you reply.\n")
	case st.Code != nil:
		fmt.Fprintf(&w, "**To approve all of it, enter this code** at %s before %s (%d minutes):\n\n```text\n%s\n```\n\n",
			st.Code.VerificationURI, st.Expires.UTC().Format("15:04 UTC"), max(st.Code.ExpiresIn/60, 1), st.Code.UserCode)
		w.WriteString("GitHub's page names the app, not this batch, and asks for no permissions. If it asks for any, or you did not ask for this, choose Cancel there; nothing is done.\n")
	}
	if b.Mode == batchByDevice && b.Why != "" {
		w.WriteString("\nComment approval is off for this batch: " + b.Why + "\n")
	}
	w.WriteString("\nCommand:\n\n" + fence(b.Command))
	w.WriteString(batchDataMarker + base64.StdEncoding.EncodeToString(b.data()) + " -->\n")
	return w.String()
}

// parseBatch reads a batch comment back: its record, and the hash it shows.
func parseBatch(body string) (*batch, string, bool) {
	first, _, _ := strings.Cut(body, "\n")
	if !strings.HasPrefix(first, batchMarker) {
		return nil, "", false
	}
	shown := strings.TrimSuffix(strings.TrimPrefix(first, batchMarker), " -->")
	// The record is the footer render writes last; untrusted text above it
	// may contain the marker too.
	i := strings.LastIndex(body, batchDataMarker)
	if i < 0 {
		return nil, shown, false
	}
	enc, rest, ok := strings.Cut(body[i+len(batchDataMarker):], " -->")
	if !ok || strings.TrimSpace(rest) != "" {
		return nil, shown, false
	}
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return nil, shown, false
	}
	var b batch
	if json.Unmarshal(raw, &b) != nil {
		return nil, shown, false
	}
	return &b, shown, true
}

// commentApprovalsOff says why the batch cannot be approved by a comment, or
// "" when it can: the bot identity writes for agents, an approver is
// recorded, this session reaches no approver's credential, and no item is a
// door [approvals] device_for keeps on the device code.
func (e *Env) commentApprovalsOff(ctx context.Context, cfg *config.Config, bot identity.Bot, items []batchItem) string {
	ap := cfg.Approvals
	switch {
	case slices.Contains(ap.DeviceFor, "all"):
		return "`[approvals] device_for = [\"all\"]` keeps every batch on the device code."
	case len(ap.Approvers) == 0:
		return "no approver is recorded in `[approvals] approvers`."
	case bot.Kind == identity.KindHumanFallback || bot.Login == "":
		return fmt.Sprintf("agents here write as @%s (no bot identity), so a reply from that account could be an agent's.", bot.Login)
	case bot.Kind != identity.KindApp:
		return fmt.Sprintf("the bot is the machine user @%s, and the aayudha-approve workflow can act only as the aayudha GitHub App.", bot.Login)
	}
	if slices.Contains(ap.DeviceFor, "one-way") {
		for i, it := range items {
			if it.OneWay {
				return fmt.Sprintf("item %d is a one-way door, and `[approvals] device_for` keeps one-way doors on the device code.", i+1)
			}
		}
	}
	repo, _ := e.RepoName(ctx)
	if ex := identity.Exposed(e.reachable(ctx, repo), ap.Approvers); len(ex) > 0 {
		if ex[0].Login == "" {
			return fmt.Sprintf("this session can reach a GitHub credential (%s) whose account GitHub could not confirm just now, so it may be an approver's; `aayu doctor` shows it.", ex[0].Source)
		}
		return fmt.Sprintf("this session can reach @%s's GitHub credential (%s), so an agent here could write the reply; `aayu doctor` shows it.", ex[0].Login, ex[0].Source)
	}
	return ""
}

// reachable lists the GitHub credentials this process can reach, by source
// and account, never by value (Decision #488).
func (e *Env) reachable(ctx context.Context, repo string) []identity.Reach {
	run := e.exec()
	sources := map[string]func(context.Context) (string, error){}
	// gh stores several accounts and `gh auth token` gives only the active
	// one; any of them, active or not, can write a reply.
	accounts, err := e.ghAccounts(ctx)
	var unread []identity.Reach // gh names the account but would not give its token now: it is still reachable
	if err != nil {             // gh is there but would not list its accounts: any of them may be an approver's
		unread = append(unread, identity.Reach{Source: "gh auth status (could not list gh's accounts now)"})
	}
	for login, tok := range accounts {
		if tok == "" {
			unread = append(unread, identity.Reach{Source: "gh auth token --user " + login + " (unreadable now)", Login: login})
			continue
		}
		sources["gh auth token --user "+login] = func(context.Context) (string, error) { return tok, nil }
	}
	for name, f := range map[string]func(context.Context) (string, error){
		"gh auth token": func(ctx context.Context) (string, error) {
			if e.GHToken != nil {
				return e.GHToken(ctx)
			}
			out, err := run(ctx, e.Dir, "gh", "auth", "token")
			return strings.TrimSpace(out), err
		},
		"aayu's GitHub token": func(ctx context.Context) (string, error) { // AAYU_GITHUB_TOKEN, gh, or the store: whatever aayu would act as
			t, err := e.Resolver().Human(ctx)
			return t.Value, err
		},
		"aayu secrets store": func(context.Context) (string, error) {
			st := e.Resolver().Secrets
			if st == nil {
				return "", errors.New("no secrets store")
			}
			v, _, err := st.Get(identity.HumanTokenSecret)
			return v, err
		},
		"git credential helper": func(ctx context.Context) (string, error) {
			if e.GitCredential != nil {
				return e.GitCredential(ctx, repo)
			}
			path := ""
			if repo != "" {
				path = "path=" + repo + "\n"
			}
			out, err := run(ctx, e.Dir, "sh", "-c", "printf 'protocol=https\\nhost=github.com\\n"+path+"\\n' | GIT_TERMINAL_PROMPT=0 git credential fill")
			if err != nil {
				return "", err
			}
			for _, line := range strings.Split(out, "\n") {
				if v, ok := strings.CutPrefix(line, "password="); ok {
					return v, nil
				}
			}
			return "", errors.New("no password")
		},
	} {
		sources[name] = f
	}
	return append(identity.Reachable(ctx, e.Environ, sources, func(ctx context.Context, t identity.Token) (identity.User, error) {
		return identity.WhoAmI(ctx, e.HTTP, e.githubAPI(), t)
	}), unread...)
}

// ghAccounts is every account gh stores for github.com, login to token ("" when unreadable).
func (e *Env) ghAccounts(ctx context.Context) (map[string]string, error) {
	if e.GHAccounts != nil {
		return e.GHAccounts(ctx)
	}
	if _, err := e.lookPath()("gh"); err != nil {
		return nil, nil // no gh, no stored accounts
	}
	run := e.exec()
	// gh exits 1 when any stored account fails its check; the list is still printed.
	out, _ := run(ctx, e.Dir, "gh", "auth", "status", "--hostname", "github.com", "--json", "hosts")
	var st struct {
		Hosts map[string][]struct {
			Login string `json:"login"`
		} `json:"hosts"`
	}
	if err := json.Unmarshal([]byte(out), &st); err != nil {
		return nil, err
	}
	toks := map[string]string{}
	for _, a := range st.Hosts["github.com"] {
		if a.Login == "" { // gh's placeholder when nothing is logged in
			continue
		}
		t, _ := run(ctx, e.Dir, "gh", "auth", "token", "--hostname", "github.com", "--user", a.Login)
		toks[a.Login] = strings.TrimSpace(t) // "" when gh could not read it now: it still counts
	}
	return toks, nil
}

// openBatch is the open batch on issue n written by botLogin, with its
// comment, or nil: the one whose hash is id, or with id "" the newest. It is
// open until a result comment answers its hash, and for batchOpenFor after
// it was posted.
func openBatch(ctx context.Context, c *github.Client, repo string, n int, botLogin, id string, now time.Time) (*batch, *gh.IssueComment, string, error) {
	owner, name, _ := github.SplitRepo(repo)
	comments, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.IssueComment, *gh.Response, error) {
		return c.REST.Issues.ListComments(ctx, owner, name, n, &gh.IssueListCommentsOptions{ListOptions: o})
	})
	if err != nil {
		return nil, nil, "", err
	}
	answered := map[string]bool{} // only the bot's own result closes a batch: anyone can copy the marker
	for _, cm := range comments {
		if first, _, _ := strings.Cut(cm.GetBody(), "\n"); strings.HasPrefix(first, batchResultMarker) && strings.EqualFold(cm.GetUser().GetLogin(), botLogin) {
			answered[strings.TrimSuffix(strings.TrimPrefix(first, batchResultMarker), " -->")] = true
		}
	}
	for i := len(comments) - 1; i >= 0; i-- {
		cm := comments[i]
		b, shown, ok := parseBatch(cm.GetBody())
		if !ok || !strings.EqualFold(cm.GetUser().GetLogin(), botLogin) || b.Mode != batchByComment || id != "" && shown != id {
			continue
		}
		if answered[shown] || now.Sub(cm.GetCreatedAt().Time) > batchOpenFor {
			return nil, nil, "", nil
		}
		return b, cm, shown, nil
	}
	return nil, nil, "", nil
}

// approveCommand is a parsed `/aayu approve ...` or `/aayu skip ...` reply.
type approveCommand struct {
	All   bool
	Pick  []int  // approve only these
	Skip  []int  // approve all but these
	Batch string // the hash of the batch the approver read; "" when the reply names none
}

// approveLine is a reply: the items, then the batch's hash. The hash binds the
// reply to the batch the person read, not to a newer one posted meanwhile.
var approveLine = regexp.MustCompile(`^/aayu\s+(approve|skip)((?:\s+(?:all|[0-9]{1,2}))+)(?:\s+([0-9a-f]{12}))?\s*$`)

func parseApprove(body string) (approveCommand, bool) {
	first, _, _ := strings.Cut(strings.TrimSpace(body), "\n")
	m := approveLine.FindStringSubmatch(strings.TrimSpace(first))
	if m == nil {
		return approveCommand{}, false
	}
	a := approveCommand{Batch: m[3]}
	for _, f := range strings.Fields(m[2]) {
		if f == "all" {
			a.All = true
			continue
		}
		n, _ := strconv.Atoi(f)
		if m[1] == "skip" {
			a.Skip = append(a.Skip, n)
		} else {
			a.Pick = append(a.Pick, n)
		}
	}
	if m[1] == "skip" && a.All || a.All && len(a.Pick) > 0 || !a.All && len(a.Pick) == 0 && len(a.Skip) == 0 {
		return approveCommand{}, false
	}
	return a, true
}

// selected is the 1-based item numbers the command approves, or an error
// naming a number the batch does not have.
func (a approveCommand) selected(n int) ([]int, error) {
	for _, i := range append(append([]int{}, a.Pick...), a.Skip...) {
		if i < 1 || i > n {
			return nil, fmt.Errorf("the batch has items 1 to %d, not %d", n, i)
		}
	}
	var out []int
	for i := 1; i <= n; i++ {
		switch {
		case a.All, slices.Contains(a.Pick, i), len(a.Skip) > 0 && !slices.Contains(a.Skip, i):
			out = append(out, i)
		}
	}
	return out, nil
}

// approvalEvent is the part of GitHub's issue_comment event the approval
// workflow reads.
type approvalEvent struct {
	Action  string `json:"action"`
	Comment struct {
		ID int64 `json:"id"`
	} `json:"comment"`
	Issue struct {
		Number int `json:"number"`
	} `json:"issue"`
}

// approveFromComment is `aayu inbox --ci` in the aayudha-approve workflow: it
// verifies an `/aayu approve` reply on a batch and applies it as the bot.
// Every check passes or nothing is applied; a refusal is answered on the
// thread with what happened and the one next step, except for an author
// without write access or a bot, who get no reply.
func (e *Env) approveFromComment(ctx context.Context) error {
	if name := e.Getenv("GITHUB_EVENT_NAME"); name != "issue_comment" {
		return Errorf(ExitUsage, "inbox --ci applies an `/aayu approve` reply from the aayudha-approve workflow's issue_comment event, not %q", name)
	}
	raw, err := os.ReadFile(e.Getenv("GITHUB_EVENT_PATH"))
	if err != nil {
		return Wrap(ExitUsage, fmt.Errorf("inbox --ci: reading the event: %w", err))
	}
	var ev approvalEvent
	if err := json.Unmarshal(raw, &ev); err != nil {
		return Wrap(ExitUsage, fmt.Errorf("inbox --ci: the event: %w", err))
	}
	if ev.Action != "created" {
		fmt.Fprintf(e.Out, "inbox --ci: a %s comment is never an approval; only a new reply counts\n", ev.Action)
		return nil
	}
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	l, err := e.Config()
	if err != nil {
		return err
	}
	cfg := &l.Config
	owner, name, _ := github.SplitRepo(repo)
	hc, _, err := e.HumanGitHub(ctx) // the workflow's own token: reads, and replies when there is no bot
	if err != nil {
		return err
	}
	cm, _, err := hc.REST.Issues.GetComment(ctx, owner, name, ev.Comment.ID) // as it is now, not as the event saw it
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	cmd, ok := parseApprove(cm.GetBody())
	if !ok {
		fmt.Fprintln(e.Out, "inbox --ci: not an `/aayu approve` or `/aayu skip` reply; nothing to do")
		return nil
	}
	author := cm.GetUser().GetLogin()
	if identity.IsBot(identity.Account{Login: author, Type: cm.GetUser().GetType()}, cfg.Identity.BotLogins) {
		fmt.Fprintf(e.Out, "inbox --ci: @%s is a bot identity; its reply is never an approval\n", author)
		return nil
	}
	perm, _, err := hc.REST.Repositories.GetPermissionLevel(ctx, owner, name, author)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	if p := permissionOf(perm); p != "admin" && p != "write" {
		fmt.Fprintf(e.Out, "inbox --ci: @%s has %s access; only a person with write access can approve, and others get no reply\n", author, p)
		return nil
	}
	n := ev.Issue.Number
	bc, bot, berr := e.BotGitHub(ctx, "")
	noBot := berr != nil || bot.Kind == identity.KindHumanFallback || bot.Login == ""
	writer := bc
	if noBot {
		writer = hc
	}
	refuse := func(why, next string) error {
		body := fmt.Sprintf("@%s, nothing was approved: %s\n\nNext: %s\n", author, why, next)
		_ = postComment(ctx, writer, repo, n, "github.comment.batch-refused", fmt.Sprintf("<!-- aayudha:batch-refused %d -->\n%s", cm.GetID(), body))
		return Errorf(ExitBlocked, "%s", why)
	}
	switch {
	case noBot:
		return refuse("this repository has no bot identity, so agents write as a person, and a reply cannot prove it was yours.",
			"approve with the device code: ask the agent to run the command again; or set up the aayudha App (`aayu init --identity`) and point agents at it.")
	case slices.Contains(cfg.Approvals.DeviceFor, "all"):
		return refuse("replies are off: `[approvals] device_for = [\"all\"]` keeps every batch on the device code.", "ask the agent to run the command again; it posts a batch with a device code.")
	case len(cfg.Approvals.Approvers) == 0:
		return refuse("no approver is recorded in `[approvals] approvers`.", "record the people who approve by reply in .aayudha/config.toml through a reviewed PR.")
	case !slices.ContainsFunc(cfg.Approvals.Approvers, func(a string) bool { return strings.EqualFold(strings.TrimPrefix(a, "@"), author) }):
		return refuse(fmt.Sprintf("@%s is not a recorded approver (`[approvals] approvers`).", author), "ask a recorded approver to reply, or record @"+author+" through a reviewed PR to .aayudha/config.toml.")
	case cm.GetUpdatedAt().After(cm.GetCreatedAt().Time):
		return refuse("your reply was edited, and an edited reply never counts.", "post a new reply; do not edit it.")
	}
	if cmd.Batch == "" {
		return refuse("your reply does not name a batch.", "copy the reply line from the batch, with its hash at the end, for example `/aayu approve all 1a2b3c4d5e6f`.")
	}
	b, bcm, shown, err := openBatch(ctx, hc, repo, n, bot.Login, cmd.Batch, e.Now())
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	switch {
	case b == nil:
		return refuse(fmt.Sprintf("there is no open batch `%s` from @%s on this issue.", cmd.Batch, bot.Login), "copy the reply line from an open batch, or ask the agent to start the steps again; it posts a new batch.")
	case !bcm.GetCreatedAt().Before(cm.GetCreatedAt().Time):
		return refuse("your reply is older than the batch.", "reply again below the batch.")
	case bcm.GetUpdatedAt().After(bcm.GetCreatedAt().Time):
		return refuse("the batch comment was edited after it was posted, so it may not say what was asked.", "ask the agent to start the steps again; it posts a new batch.")
	case shown != b.hash() || b.Repo != repo || b.Issue != n:
		return refuse("the batch's hash does not match its record.", "ask the agent to start the steps again; it posts a new batch.")
	}
	picked, err := cmd.selected(len(b.Items))
	if err != nil {
		return refuse(err.Error()+".", fmt.Sprintf("reply with numbers from the batch, or `/aayu approve all %s`.", shown))
	}
	// A pick that is not the recommendation, or a one-way door, is confirmed
	// only by naming it.
	var unnamed []string
	picked = slices.DeleteFunc(picked, func(i int) bool {
		if b.Items[i-1].named() && !slices.Contains(cmd.Pick, i) {
			why := "is a one-way door"
			if b.Items[i-1].Other {
				why = "is not the recommendation"
			}
			unnamed = append(unnamed, fmt.Sprintf("Item %d was left out: it %s, so only `/aayu approve %d %s` approves it.", i, why, i, shown))
			return true
		}
		return false
	})
	if len(picked) == 0 {
		return refuse(strings.Join(unnamed, " "), fmt.Sprintf("name the items you approve, for example `/aayu approve 1 %s`.", shown))
	}
	type ready struct {
		it  batchItem
		d   decide.Decision
		pos int
	}
	var todo []ready
	for _, i := range picked {
		it := b.Items[i-1]
		is, _, err := hc.REST.Issues.Get(ctx, owner, name, it.Issue)
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		d, err := loadDecision(ctx, hc, repo, is)
		if err != nil {
			return err
		}
		// The door as the Decision is now, not as the batch recorded it: a
		// waiver label added since makes it one-way (the body hash does not
		// cover labels).
		if oneWay := it.OneWay || !d.Reversible() || d.Waiver(); oneWay && slices.Contains(cfg.Approvals.DeviceFor, "one-way") {
			return refuse(fmt.Sprintf("item %d is a one-way door, and `[approvals] device_for` keeps one-way doors on the device code.", i), "approve it with the device code.")
		}
		if decisionHash(d) != it.BodyHash {
			return refuse(fmt.Sprintf("Decision #%d changed after the batch was posted.", it.Issue), "ask the agent to start the steps again; it posts a new batch.")
		}
		if o, err := acceptable(cfg, d, strconv.Itoa(it.N), author); err != nil {
			return refuse(strings.TrimPrefix(err.Error(), "decide: ")+".", "ask the Decision's authority to reply.")
		} else if o.Name != it.Option {
			return refuse(fmt.Sprintf("option %d of Decision #%d is no longer %s.", it.N, it.Issue, mdText(it.Option)), "ask the agent to start the steps again; it posts a new batch.")
		}
		todo = append(todo, ready{it, d, i})
	}
	// The comment must show exactly what its record holds. Its hash is no
	// signature: anyone with the bot's token could post a record whose
	// visible text says something else. So rebuild the comment from the
	// record and the Decisions as they are, and compare.
	if why, err := e.presentedAsRecorded(ctx, hc, cfg, b, bcm.GetBody()); err != nil {
		return err
	} else if why != "" {
		return refuse("the batch comment does not show what its record holds ("+why+").", "ask the agent to start the steps again; it posts a new batch.")
	}
	url := cm.GetHTMLURL()
	lines := unnamed
	for _, r := range todo {
		// Each Decision again at its write: an earlier write takes time, and
		// an edit in between is not what the approver saw.
		d, why, err := unchangedDecision(ctx, hc, repo, r.it)
		if err != nil {
			return err
		}
		if why != "" {
			lines = append(lines, fmt.Sprintf("%d. #%d was not accepted: %s since the batch was posted", r.pos, r.it.Issue, why))
			return e.batchResult(ctx, writer, repo, n, shown, author, lines, picked, len(b.Items), Errorf(ExitBlocked, "Decision #%d %s", r.it.Issue, why))
		}
		r.d = d
		if _, err := acceptDecision(ctx, e, bc, repo, r.d, r.it.Option, author, "approved in "+url); err != nil {
			lines = append(lines, fmt.Sprintf("%d. #%d was not accepted: %v", r.pos, r.it.Issue, err))
			return e.batchResult(ctx, writer, repo, n, shown, author, lines, picked, len(b.Items), err)
		}
		lines = append(lines, fmt.Sprintf("%d. #%d accepted: option %d (%s), by @%s in %s", r.pos, r.it.Issue, r.it.N, mdText(r.it.Option), author, url))
	}
	return e.batchResult(ctx, writer, repo, n, shown, author, lines, picked, len(b.Items), nil)
}

// presentedAsRecorded says why body is not exactly the comment aayu renders
// for record b with the Decisions as they are now, or "".
func (e *Env) presentedAsRecorded(ctx context.Context, c *github.Client, cfg *config.Config, b *batch, body string) (string, error) {
	owner, name, _ := github.SplitRepo(b.Repo)
	want := *b
	want.Mention = cfg.Approvals.Approvers
	want.Items = make([]batchItem, len(b.Items))
	for i, it := range b.Items {
		is, _, err := c.REST.Issues.Get(ctx, owner, name, it.Issue)
		if err != nil {
			return "", Wrap(ExitExternal, err)
		}
		d, err := loadDecision(ctx, c, b.Repo, is)
		if err != nil {
			return "", err
		}
		o, ok := decide.Pick(d.Item, strconv.Itoa(it.N))
		if !ok || o.Name != it.Option {
			return fmt.Sprintf("item %d", i+1), nil
		}
		r := batchDecision(d, o)
		if r.Kind != it.Kind || r.OneWay != it.OneWay || r.Other != it.Other || r.BodyHash != it.BodyHash {
			return fmt.Sprintf("item %d", i+1), nil
		}
		want.Items[i] = r
	}
	if want.render(batchStatus{}) != body {
		return "its text", nil
	}
	return "", nil
}

// unchangedDecision reloads the Decision of item it and says why it is no
// longer the one in the batch, or "" with the Decision as it is now.
func unchangedDecision(ctx context.Context, c *github.Client, repo string, it batchItem) (decide.Decision, string, error) {
	owner, name, _ := github.SplitRepo(repo)
	is, _, err := c.REST.Issues.Get(ctx, owner, name, it.Issue)
	if err != nil {
		return decide.Decision{}, "", Wrap(ExitExternal, err)
	}
	d, err := loadDecision(ctx, c, repo, is)
	if err != nil {
		return d, "", err
	}
	switch {
	case d.State != "open":
		return d, "it was closed", nil
	case decisionHash(d) != it.BodyHash:
		return d, "its title, body, or parent changed", nil
	case (!d.Reversible() || d.Waiver()) != it.OneWay:
		return d, "its door changed", nil
	}
	return d, "", nil
}

// batchResult answers the batch: what was done, and what was skipped. It
// closes the batch, so a second reply approves nothing more.
func (e *Env) batchResult(ctx context.Context, c *github.Client, repo string, n int, hash, author string, lines []string, picked []int, total int, failed error) error {
	var w strings.Builder
	fmt.Fprintf(&w, "%s%s -->\n@%s, batch `%s`:\n\n", batchResultMarker, hash, author, hash)
	for _, l := range lines {
		w.WriteString(l + "\n")
	}
	var skipped []string
	for i := 1; i <= total; i++ {
		if !slices.Contains(picked, i) {
			skipped = append(skipped, strconv.Itoa(i))
		}
	}
	if len(skipped) > 0 {
		w.WriteString("\nSkipped: " + strings.Join(skipped, ", ") + ".\n")
	}
	if failed != nil {
		w.WriteString("\nThe rest was not applied. Next: ask the agent to start the remaining steps again.\n")
	}
	if err := postComment(ctx, c, repo, n, "github.comment.batch-result", w.String()); err != nil {
		return Wrap(ExitExternal, err)
	}
	fmt.Fprint(e.Out, w.String())
	if failed != nil {
		return Wrap(ExitExternal, failed)
	}
	return nil
}

// postComment posts a new comment on issue n. Batch results and refusals are
// always new comments, never edits of one found by its marker: anyone can
// copy a marker into their own comment, and a new comment notifies.
func postComment(ctx context.Context, c *github.Client, repo string, n int, kind, body string) error {
	_, err := createComment(ctx, c, repo, n, kind, body)
	return err
}

// createComment posts body on issue n and returns the new comment's ID (0 in a dry run).
func createComment(ctx context.Context, c *github.Client, repo string, n int, kind, body string) (int64, error) {
	owner, name, _ := github.SplitRepo(repo)
	var id int64
	_, err := c.Write(ctx, actions.Action{Kind: kind, Target: fmt.Sprintf("%s#%d", repo, n),
		Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/issues/%d/comments", repo, n)}},
		func(ctx context.Context) (any, error) {
			cm, _, err := c.REST.Issues.CreateComment(ctx, owner, name, n, &gh.IssueComment{Body: gh.Ptr(body)})
			id = cm.GetID()
			return id, err
		})
	return id, err
}

// show posts the device batch's comment the first time and then edits that
// exact comment: a marker copied into another comment never redirects it.
func (b *batch) show(ctx context.Context, c *github.Client, body string) error {
	if b.comment == 0 {
		id, err := createComment(ctx, c, b.Repo, b.Issue, "github.comment.batch", body)
		b.comment = id
		return err
	}
	owner, name, _ := github.SplitRepo(b.Repo)
	_, err := c.Write(ctx, actions.Action{Kind: "github.comment.batch", Target: fmt.Sprintf("%s#%d", b.Repo, b.Issue),
		Request: &actions.Request{Method: http.MethodPatch, Path: fmt.Sprintf("/repos/%s/issues/comments/%d", b.Repo, b.comment)}},
		func(ctx context.Context) (any, error) {
			_, _, err := c.REST.Issues.EditComment(ctx, owner, name, b.comment, &gh.IssueComment{Body: gh.Ptr(body)})
			return b.comment, err
		})
	return err
}
