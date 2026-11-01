package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof/fakes3"
)

// Story #1176: the evening review, on the missionWorld of Story #997.

var (
	head1010 = strings.Repeat("1010", 10)
	head1011 = strings.Repeat("1011", 10)
	cartPNG  = []byte("\x89PNG the cart of #1010")
)

// eveningWorld is the story's day: on Monday 2 November 2026 @rahulnakmol
// starts #889's mission at 09:00, which starts MS1 (#1010, #1011, #1104);
// MS2 (#1012) waits for MS1's gate. #1010 and #1011 merge with proof packs:
// #1010's maps criterion 1 to a screenshot in the media bucket and criterion
// 2 to an API trace on the ledger; #1011's maps only criterion 1, and a newer
// pack of #1011 for another head, the best by authority and age, maps criterion 2. A person stops
// #1104, which does not merge. The clock reads *now, London time.
func eveningWorld(t *testing.T) (*missionWorld, *time.Time) {
	t.Helper()
	m := newMissionWorld(t)
	now := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	m.opts.Now = func() time.Time { m.mu.Lock(); defer m.mu.Unlock(); return now }
	m.mu.Lock()
	m.tool = "amp"
	m.mu.Unlock()
	m.issue(889, "Agents pull the next story", "feature", 0, "")
	for _, n := range []int{1010, 1011, 1104, 1012} {
		m.issue(n, fmt.Sprintf("Story %d", n), "story", 889, "")
		m.status(n, "ready", "")
	}
	m.feature(889, 1010, 1011, 1104, 1012)
	if code, out, errOut := m.approve(889, "--milestones", "MS1=#1010,#1011,#1104 MS2=#1012"); code != 0 {
		t.Fatalf("approve 889: exit %d: %s%s", code, out, errOut)
	}
	if code, out, errOut := m.mission(m.person, "rahulnakmol", "start", 889); code != 0 {
		t.Fatalf("start 889: exit %d: %s%s", code, out, errOut)
	}

	// The media bucket, from dev's committed config (the checkout's is the same) and the person's secret.
	s3 := fakes3.New(t, "sk")
	cfg := "version = 1\n[proof]\nmedia = \"s3://proof-bucket/acme?endpoint=" + s3.Server.URL + "\"\n"
	m.edit(func() { m.g.RefFiles[devTip][".aayudha/config.toml"] = cfg })
	if err := os.MkdirAll(filepath.Join(m.dir, ".aayudha"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(m.dir, ".aayudha", "config.toml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	m.person["AAYU_SECRET_PROOF_MEDIA"] = "ak:sk"
	cart := proof.NewArtifact("media/cart.png", "screenshot", cartPNG)
	cart.Store, cart.Object = proof.StoreBucket, "acme/proof/o/r/1010/"+cart.SHA256
	s3.Mu.Lock()
	s3.Objects["proof-bucket/"+cart.Object] = cartPNG
	s3.Mu.Unlock()

	merged := time.Date(2026, 11, 2, 15, 0, 0, 0, time.UTC)
	for i, n := range []int{1010, 1011} {
		head := []string{head1010, head1011}[i]
		pr := m.g.AddPR(fmt.Sprintf("feat: story %d", n), fmt.Sprintf("Refs #%d\n", n), "agent", fmt.Sprintf("story/%d-x", n), head, "dev")
		m.edit(func() {
			pr.State, pr.Merged, pr.MergedAt, pr.MergeCommitSHA = "closed", true, merged, strings.Repeat("e", 39)+fmt.Sprint(i)
		})
		m.status(n, "merged", "")
	}
	trace := []byte(`{"Action":"pass","Test":"TestCart"}` + "\n")
	putPack(t, m.bare, &proof.Pack{Story: 1010, PRHeadSHA: head1010, Authority: proof.Authority{Kind: proof.Authoritative, RunID: 77},
		Criteria:  []proof.Criterion{{ID: 1, Evidence: []string{"media/cart.png"}}, {ID: 2, Evidence: []string{"evidence/criterion-2.jsonl"}}},
		Artifacts: []proof.Artifact{cart, proof.NewArtifact("evidence/criterion-2.jsonl", "api-trace", trace)}},
		map[string][]byte{"evidence/criterion-2.jsonl": trace})
	putPack(t, m.bare, &proof.Pack{Story: 1011, PRHeadSHA: head1011, Authority: proof.Authority{Kind: proof.Authoritative, RunID: 78},
		Criteria:  []proof.Criterion{{ID: 1, Evidence: []string{"media/page.png"}}},
		Artifacts: []proof.Artifact{proof.NewArtifact("media/page.png", "screenshot", []byte("\x89PNG page"))}}, nil)
	putPack(t, m.bare, &proof.Pack{Story: 1011, PRHeadSHA: strings.Repeat("dead", 10), Authority: proof.Authority{Kind: proof.Authoritative, RunID: 79}, CreatedAt: "2026-11-02T16:00:00Z",
		Criteria:  []proof.Criterion{{ID: 2, Evidence: []string{"evidence/other-head.jsonl"}}},
		Artifacts: []proof.Artifact{proof.NewArtifact("evidence/other-head.jsonl", "api-trace", trace)}},
		map[string][]byte{"evidence/other-head.jsonl": trace})
	m.setPlan(889, func(p *MissionPlan) {
		p.Stopped = append(p.Stopped, MissionStopped{Number: 1104, Reason: "the payment sandbox is down", By: "rahulnakmol", At: "2026-11-02T11:00:00Z"})
	})
	m.mu.Lock()
	now = time.Date(2026, 11, 2, 18, 0, 0, 0, time.UTC) // the end of [time] business_hours (Mon-Fri 09:00-18:00, Europe/London)
	m.mu.Unlock()
	return m, &now
}

// putPack lands p, with its git artifacts' bytes, on the ledger as the merge gate's run or an agent would.
func putPack(t *testing.T, bare string, p *proof.Pack, files map[string][]byte) {
	t.Helper()
	p.Repo, p.Seat, p.Host, p.Identity, p.CompletionGoalHash = "o/r", "ai-native-engineer", "cli", "ci", strings.Repeat("0", 64)
	p.CreatedAt = or(p.CreatedAt, "2026-11-02T15:00:00Z")
	p.Checks = []proof.Check{}
	base := proof.Dir(p.Story, p.PRHeadSHA, p.Authority.Kind)
	for i, a := range p.Artifacts {
		if b, ok := files[a.Path]; ok {
			p.Artifacts[i].Store = proof.StoreGit
			putLedger(t, bare, base+"/artifacts/"+a.Path, string(b))
		}
	}
	b, err := proof.Encode(p)
	if err != nil {
		t.Fatal(err)
	}
	putLedger(t, bare, base+"/"+proof.FileName, string(b))
}

// eveningList is env's `aayu inbox --list --json`, and its evening cards.
func eveningList(t *testing.T, m *missionWorld, env map[string]string) []InboxItem {
	t.Helper()
	code, out, errOut := run(t, env, m.opts, "inbox", "--list", "--json", "--repo", "o/r")
	var l inboxList
	if code != 0 || json.Unmarshal([]byte(out), &l) != nil {
		t.Fatalf("inbox --list: exit %d: %s%s", code, out, errOut)
	}
	return slices.DeleteFunc(l.Items, func(it InboxItem) bool { return it.Kind != InboxEvening })
}

// reviews are the lines of missions/889/reviews.jsonl.
func (m *missionWorld) reviews() []StoryReview {
	m.t.Helper()
	raw, _ := exec.Command("git", "-C", m.bare, "show", "ledger:"+MissionReviewsPath(889)).Output() // none yet: no reviews
	var out []StoryReview
	for _, l := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var r StoryReview
		if l != "" && json.Unmarshal([]byte(l), &r) == nil {
			out = append(out, r)
		}
	}
	return out
}

// Criterion 1: at the end of business hours, `aayu inbox` shows one Evening
// card per story of the milestone that started this morning, with each
// criterion, its evidence kind, and its evidence from the pack at the merged
// head: the image where the terminal shows images, else its path and link.
// A criterion with no evidence is shown too, and nothing comes from a pack of
// another head. Before the end of business hours there is no Evening card.
func TestEveningCardsShowEvidence(t *testing.T) {
	m, now := eveningWorld(t)
	m.mu.Lock()
	*now = time.Date(2026, 11, 2, 17, 59, 0, 0, time.UTC)
	m.mu.Unlock()
	if cards := eveningList(t, m, m.person); len(cards) != 0 {
		t.Fatalf("Evening cards at 17:59: %+v", cards)
	}
	m.mu.Lock()
	*now = time.Date(2026, 11, 2, 18, 0, 0, 0, time.UTC)
	m.mu.Unlock()
	var nums []int
	for _, it := range eveningList(t, m, m.person) {
		nums = append(nums, it.Number)
	}
	if !slices.Equal(nums, []int{1010, 1011, 1104}) { // not #1012: MS2 has not started
		t.Fatalf("Evening cards for %v, want #1010, #1011, #1104", nums)
	}

	code, out, errOut := runIn(t, m.person, m.opts, "sss", "inbox", "--repo", "o/r")
	card1010 := "[1/3] Evening review #1010: Story 1010\n" +
		"  " + inboxKinds[InboxEvening].Line + "\n" +
		"  Agreed this morning: MS1 of #889's mission.\n" +
		"  Merged in #7 at 1010101. Evidence: the authoritative pack proof/1010/" + head1010 + "/authoritative on the ledger.\n" +
		"  1. [screenshot] state: Story 1010 is empty; expected: the \"Your cart is empty\" message and a Browse button\n" +
		"     screenshot: media/cart.png · https://github.com/o/r/actions/runs/77\n" +
		"  2. [api-trace] state: POST /cart with no items; expected: 422 with code cart.empty\n" +
		"     api-trace: evidence/criterion-2.jsonl · https://github.com/o/r/blob/ledger/proof/1010/" + head1010 + "/authoritative/artifacts/evidence/criterion-2.jsonl\n"
	card1011 := "  1. [screenshot] state: Story 1011 is empty; expected: the \"Your cart is empty\" message and a Browse button\n" +
		"     screenshot: media/page.png · https://github.com/o/r/actions/runs/78\n" +
		"  2. [api-trace] state: POST /cart with no items; expected: 422 with code cart.empty\n" +
		"     No evidence in the pack for this criterion.\n"
	choices := "  1. Accept #1010: it does what you agreed (preselected)\n  2. Say what is wrong, in a comment on #1010\n  3. Not now: nothing is written\n  4. Open #1010 on GitHub\n"
	if code != 0 || !strings.Contains(out, card1010) || !strings.Contains(out, card1011) || !strings.Contains(out, choices) ||
		strings.Contains(out, "other-head") || strings.Contains(out, "\x1b]1337") || strings.Contains(out, "#1012") {
		t.Fatalf("inbox: exit %d:\n%s%s", code, out, errOut)
	}

	// Where the terminal shows images, the screenshot is drawn under its line; one with no bytes to read keeps its path and link.
	m.person["TERM_PROGRAM"] = "iTerm.app"
	code, out, errOut = runIn(t, m.person, m.opts, "sss", "inbox", "--repo", "o/r")
	img := "     screenshot: media/cart.png · https://github.com/o/r/actions/runs/77\n   \x1b]1337;File=name="
	if code != 0 || !strings.Contains(out, img) || !strings.Contains(out, ";inline=1:"+base64.StdEncoding.EncodeToString(cartPNG)+"\a\n") ||
		strings.Count(out, "\x1b]1337") != 1 || !strings.Contains(out, "     screenshot: media/page.png · https://github.com/o/r/actions/runs/78\n") {
		t.Fatalf("inbox in iTerm2: exit %d:\n%q%s", code, out, errOut)
	}

	// The next evening, Monday's stories were not agreed that morning; MS2 starts when MS1's gate passes, at 10:00.
	m.mu.Lock()
	*now = time.Date(2026, 11, 3, 18, 0, 0, 0, time.UTC)
	m.mu.Unlock()
	if cards := eveningList(t, m, m.person); len(cards) != 0 {
		t.Fatalf("Evening cards on Tuesday before MS2 starts: %+v", cards)
	}
	m.setPlan(889, func(p *MissionPlan) {
		p.Milestones[0].Gate = MilestoneGate{State: GatePassed, DevSHA: devTip, At: "2026-11-03T10:00:00Z"}
	})
	if cards := eveningList(t, m, m.person); len(cards) != 1 || cards[0].Number != 1012 || !slices.Contains(cards[0].Lines, "Agreed this morning: MS2 of #889's mission.") {
		t.Fatalf("Evening cards on Tuesday after MS2 starts: %+v", cards)
	}
}

// Criterion 2: he accepts #1010, and missions/889/reviews.jsonl gains a line
// with his login, the time, #1010, and accepted; its card leaves the evening.
func TestEveningAcceptRecordsReview(t *testing.T) {
	m, _ := eveningWorld(t)
	code, out, errOut := runIn(t, m.person, m.opts, "1yq", "inbox", "--repo", "o/r")
	if code != 0 || !strings.Contains(out, "done: @rahulnakmol accepted #1010 at 1010101, recorded in missions/889/reviews.jsonl") {
		t.Fatalf("inbox 1 Accept: exit %d:\n%s%s", code, out, errOut)
	}
	want := StoryReview{Story: 1010, PR: 7, Head: head1010, Verdict: ReviewAccepted, By: "rahulnakmol", At: "2026-11-02T18:00:00Z"}
	if got := m.reviews(); !slices.Equal(got, []StoryReview{want}) {
		t.Fatalf("reviews.jsonl: %+v, want %+v", got, want)
	}
	var nums []int
	for _, it := range eveningList(t, m, m.person) {
		nums = append(nums, it.Number)
	}
	if !slices.Equal(nums, []int{1011, 1104}) {
		t.Fatalf("Evening cards after the accept: %v", nums)
	}
}

// Criterion 3: he comments on #1011 `the page names the wrong person`; the
// comment is posted on #1011 as his, once, as he wrote it, and the mission's
// review log points the feedback pass (#1014) at it as one input. Nothing is
// accepted.
func TestEveningCommentIsFeedback(t *testing.T) {
	m, _ := eveningWorld(t)
	code, out, errOut := runIn(t, m.person, m.opts, "s2the page names the wrong person\nq", "inbox", "--repo", "o/r")
	if code != 0 || !strings.Contains(out, "done: your comment is on #1011") {
		t.Fatalf("inbox 2 comment: exit %d:\n%s%s", code, out, errOut)
	}
	m.g.Mu.Lock()
	var mine []*fake.Comment
	for _, c := range m.g.Issues[1011].Comments {
		if c.Body == "the page names the wrong person" {
			mine = append(mine, c)
		}
	}
	m.g.Mu.Unlock()
	if len(mine) != 1 || mine[0].User.Login != "rahulnakmol" {
		t.Fatalf("comments on #1011 with his words: %+v", mine)
	}
	got := m.reviews()
	if len(got) != 1 || got[0].Story != 1011 || got[0].Verdict != ReviewCommented || got[0].By != "rahulnakmol" || got[0].Head != head1011 ||
		got[0].Comment == "" || got[0].Comment != mine[0].HTMLURL {
		t.Fatalf("reviews.jsonl after the comment: %+v", got)
	}
}

// Criterion 4: #1104 was agreed this morning and did not merge; its card says
// `not merged:` with the mission's reason and the next step, lists each of its
// criteria with no evidence, and offers no accept.
func TestEveningNotMergedSaysWhy(t *testing.T) {
	m, _ := eveningWorld(t)
	code, out, errOut := runIn(t, m.person, m.opts, "ssq", "inbox", "--repo", "o/r")
	card := "[3/3] Evening review #1104: Story 1104\n" +
		"  " + inboxKinds[InboxEvening].Line + "\n" +
		"  Agreed this morning: MS1 of #889's mission.\n" +
		"  not merged: @rahulnakmol stopped it: the payment sandbox is down.\n" +
		"  Next: `aayu mission resume 889 --story 1104` when it may go on.\n" +
		"  1. [screenshot] state: Story 1104 is empty; expected: the \"Your cart is empty\" message and a Browse button\n" +
		"     No evidence in the pack for this criterion.\n" +
		"  2. [api-trace] state: POST /cart with no items; expected: 422 with code cart.empty\n" +
		"     No evidence in the pack for this criterion.\n"
	choices := "  1. Comment on #1104 (preselected)\n  2. Not now: nothing is written\n  3. Open #1104 on GitHub\n"
	if code != 0 || !strings.Contains(out, card) || !strings.Contains(out, choices) {
		t.Fatalf("inbox: exit %d:\n%s%s", code, out, errOut)
	}
	// Once the stop is lifted, the reason is the mission's next one: it has not started.
	m.setPlan(889, func(p *MissionPlan) { p.Stopped = nil })
	for _, it := range eveningList(t, m, m.person) {
		if it.Number == 1104 && !slices.Contains(it.Lines, "not merged: it has not started.") {
			t.Fatalf("#1104 with no stop: %q", it.Lines)
		}
	}
}

// Criterion 5: an agent lane runs `aayu inbox --list --json` in the evening:
// it lists the Evening cards, with no accept action, and writes nothing. The
// lane's `aayu inbox` is refused, so it accepts nothing.
func TestEveningListHasNoAccept(t *testing.T) {
	m, _ := eveningWorld(t)
	before := m.commits()
	m.g.Reset()
	cards := eveningList(t, m, m.lane)
	if len(cards) != 3 || len(m.g.Writes()) != 0 || m.commits() != before {
		t.Fatalf("lane's list: %d cards, writes %v", len(cards), m.g.Writes())
	}
	for _, it := range cards {
		if len(it.Criteria) != 2 || slices.ContainsFunc(it.Options, func(a InboxAction) bool { return a.Command == eveningAccept || strings.HasPrefix(strings.ToLower(a.Label), "accept") }) {
			t.Fatalf("lane's card #%d: criteria %d, options %+v", it.Number, len(it.Criteria), it.Options)
		}
	}
	if code, out, errOut := run(t, m.lane, m.opts, "inbox", "--list", "--repo", "o/r"); code != 0 || !strings.Contains(out, "Evening review") || strings.Contains(out, "Accept #") {
		t.Fatalf("lane's Markdown list: exit %d:\n%s%s", code, out, errOut)
	}
	if code, _, errOut := runIn(t, m.lane, m.opts, "1y", "inbox", "--repo", "o/r"); code != ExitBlocked || len(m.reviews()) != 0 {
		t.Fatalf("lane's inbox: exit %d, reviews %+v: %s", code, m.reviews(), errOut)
	}
}
