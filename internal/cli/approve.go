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

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/decide"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
)

// Approval batches (Decision #488, Story #481). The human-only steps an
// agent starts together go into one numbered comment on the first item's
// issue. Where agents act only as a bot and a recorded approver's GitHub
// credential is in no agent's reach, the approver answers the batch with a
// reply (`/aayu approve all`, `/aayu approve 1 3`, `/aayu skip 2`), which the
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
	Option string `json:"option"`
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
	Mention []string    `json:"-"` // who is asked
}

// decisionHash covers what the batch shows of a Decision: its title and body
// (labels are read again when the reply arrives).
func decisionHash(d decide.Decision) string {
	sum := sha256.Sum256([]byte(d.Title + "\x00" + d.Body))
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

// batchDecision describes accepting option of Decision d in plain words.
func batchDecision(d decide.Decision, option string) batchItem {
	title := strings.TrimSpace(strings.TrimPrefix(d.Title, "Decision:"))
	it := batchItem{Kind: "decision", Issue: d.Number, Option: option, OneWay: !d.Reversible() || d.Waiver(), BodyHash: decisionHash(d),
		Effect: fmt.Sprintf("Accepts option %s of Decision #%d: %s.", option, d.Number, strings.TrimSuffix(title, "."))}
	it.Heading = fmt.Sprintf("Accepts option %s of Decision #%d: %s.", mdText(option), d.Number, mdText(strings.TrimSuffix(title, ".")))
	if rec := strings.TrimSpace(d.Item.Fields["recommendation"]); !recommends(rec, option) {
		it.Other = true
		it.Effect += " This is not the recommendation."
		it.Heading += " This is not the recommendation."
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
	if opts := clip(strings.TrimSpace(d.Item.Fields["options"]), 1500); opts != "" {
		b.WriteString("Options:\n" + opts + "\n")
	}
	if rec := clip(strings.TrimSpace(d.Item.Fields["recommendation"]), 500); rec != "" {
		b.WriteString("Recommendation: " + strings.ReplaceAll(rec, "\n", " ") + "\n")
	}
	it.Detail = b.String() // untrusted issue text: render shows it only inside a code fence
	return it
}

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

func mdText(s string) string { return mdEscape.Replace(s) }

// recommends reports whether the Recommendation names option first: "B",
// "B: ...", "Option 2 ...", or "2. ...". Anything else counts as another
// option, which is the safe side: it needs an explicit confirmation.
func recommends(rec, option string) bool {
	rec, o := strings.ToLower(rec), strings.ToLower(strings.TrimSpace(option))
	if o == "" {
		return false
	}
	for _, p := range []string{o, "option " + o} {
		if rest, ok := strings.CutPrefix(rec, p); ok && (rest == "" || strings.IndexAny(rest[:1], ".:,;) \n—-") == 0) {
			return true
		}
	}
	return false
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
		w.WriteString("**To approve, reply on this issue** with one line:\n\n")
		w.WriteString("- `/aayu approve all`\n")
		if len(b.Items) > 1 {
			w.WriteString("- `/aayu approve 1 3` approves only those items\n- `/aayu skip 2` approves all but those items\n")
		}
		for i, it := range b.Items {
			if it.Other {
				fmt.Fprintf(&w, "\nItem %d is not the recommendation, so `all` and `skip` leave it out; to approve it, name it: `/aayu approve %d`.\n", i+1, i+1)
			}
		}
		w.WriteString("\nOnly a reply from a recorded approver counts, and it must not be edited. Nothing happens until you reply.\n")
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
	i := strings.Index(body, batchDataMarker)
	if i < 0 {
		return nil, shown, false
	}
	enc, _, _ := strings.Cut(body[i+len(batchDataMarker):], " -->")
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
		return fmt.Sprintf("this session can reach @%s's GitHub credential (%s), so an agent here could write the reply; `aayu doctor` shows it.", ex[0].Login, ex[0].Source)
	}
	return ""
}

// reachable lists the GitHub credentials this process can reach, by source
// and account, never by value (Decision #488).
func (e *Env) reachable(ctx context.Context, repo string) []identity.Reach {
	run := e.exec()
	sources := map[string]func(context.Context) (string, error){
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
	}
	return identity.Reachable(ctx, e.Environ, sources, func(ctx context.Context, t identity.Token) (identity.User, error) {
		return identity.WhoAmI(ctx, e.HTTP, e.githubAPI(), t)
	})
}

// openBatch is the open batch on issue n written by botLogin, with its
// comment, or nil. It is open until a result comment answers its hash, and
// for batchOpenFor after it was posted.
func openBatch(ctx context.Context, c *github.Client, repo string, n int, botLogin string, now time.Time) (*batch, *gh.IssueComment, string, error) {
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
		if !ok || !strings.EqualFold(cm.GetUser().GetLogin(), botLogin) || b.Mode != batchByComment {
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
	All  bool
	Pick []int // approve only these
	Skip []int // approve all but these
}

var approveLine = regexp.MustCompile(`^/aayu\s+(approve|skip)((?:\s+(?:all|[0-9]+))+)\s*$`)

func parseApprove(body string) (approveCommand, bool) {
	first, _, _ := strings.Cut(strings.TrimSpace(body), "\n")
	m := approveLine.FindStringSubmatch(strings.TrimSpace(first))
	if m == nil {
		return approveCommand{}, false
	}
	var a approveCommand
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
	case len(cfg.Approvals.Approvers) == 0:
		return refuse("no approver is recorded in `[approvals] approvers`.", "record the people who approve by reply in .aayudha/config.toml through a reviewed PR.")
	case !slices.ContainsFunc(cfg.Approvals.Approvers, func(a string) bool { return strings.EqualFold(strings.TrimPrefix(a, "@"), author) }):
		return refuse(fmt.Sprintf("@%s is not a recorded approver (`[approvals] approvers`).", author), "ask a recorded approver to reply, or record @"+author+" through a reviewed PR to .aayudha/config.toml.")
	case cm.GetUpdatedAt().After(cm.GetCreatedAt().Time):
		return refuse("your reply was edited, and an edited reply never counts.", "post a new reply; do not edit it.")
	}
	b, bcm, shown, err := openBatch(ctx, hc, repo, n, bot.Login, e.Now())
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	switch {
	case b == nil:
		return refuse(fmt.Sprintf("there is no open batch from @%s on this issue.", bot.Login), "ask the agent to start the steps again; it posts a new batch.")
	case !bcm.GetCreatedAt().Before(cm.GetCreatedAt().Time):
		return refuse("your reply is older than the batch.", "reply again below the batch.")
	case bcm.GetUpdatedAt().After(bcm.GetCreatedAt().Time):
		return refuse("the batch comment was edited after it was posted, so it may not say what was asked.", "ask the agent to start the steps again; it posts a new batch.")
	case shown != b.hash() || b.Repo != repo || b.Issue != n:
		return refuse("the batch's hash does not match its record.", "ask the agent to start the steps again; it posts a new batch.")
	}
	picked, err := cmd.selected(len(b.Items))
	if err != nil {
		return refuse(err.Error()+".", "reply with numbers from the batch, or `/aayu approve all`.")
	}
	// A pick that is not the recommendation is confirmed only by naming it.
	var unnamed []string
	picked = slices.DeleteFunc(picked, func(i int) bool {
		if b.Items[i-1].Other && !slices.Contains(cmd.Pick, i) {
			unnamed = append(unnamed, fmt.Sprintf("Item %d was left out: it is not the recommendation, so only `/aayu approve %d` approves it.", i, i))
			return true
		}
		return false
	})
	if len(picked) == 0 {
		return refuse(strings.Join(unnamed, " "), "name the items you approve, for example `/aayu approve 1`.")
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
		if oneWay := it.OneWay || !d.Reversible() || d.Waiver(); oneWay && (slices.Contains(cfg.Approvals.DeviceFor, "one-way") || slices.Contains(cfg.Approvals.DeviceFor, "all")) {
			return refuse(fmt.Sprintf("item %d is a one-way door, and `[approvals] device_for` keeps one-way doors on the device code.", i), "approve it with the device code.")
		}
		if decisionHash(d) != it.BodyHash {
			return refuse(fmt.Sprintf("Decision #%d changed after the batch was posted.", it.Issue), "ask the agent to start the steps again; it posts a new batch.")
		}
		if err := acceptable(cfg, d, it.Option, author); err != nil {
			return refuse(strings.TrimPrefix(err.Error(), "decide: ")+".", "ask the Decision's authority to reply.")
		}
		todo = append(todo, ready{it, d, i})
	}
	url := cm.GetHTMLURL()
	lines := unnamed
	for _, r := range todo {
		if _, err := acceptDecision(ctx, e, bc, repo, r.d, r.it.Option, author, "approved in "+url); err != nil {
			lines = append(lines, fmt.Sprintf("%d. #%d was not accepted: %v", r.pos, r.it.Issue, err))
			return e.batchResult(ctx, writer, repo, n, shown, author, lines, picked, len(b.Items), err)
		}
		lines = append(lines, fmt.Sprintf("%d. #%d accepted: option %s, by @%s in %s", r.pos, r.it.Issue, mdText(r.it.Option), author, url))
	}
	return e.batchResult(ctx, writer, repo, n, shown, author, lines, picked, len(b.Items), nil)
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
	owner, name, _ := github.SplitRepo(repo)
	_, err := c.Write(ctx, actions.Action{Kind: kind, Target: fmt.Sprintf("%s#%d", repo, n),
		Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/issues/%d/comments", repo, n)}},
		func(ctx context.Context) (any, error) {
			cm, _, err := c.REST.Issues.CreateComment(ctx, owner, name, n, &gh.IssueComment{Body: gh.Ptr(body)})
			return cm.GetID(), err
		})
	return err
}
