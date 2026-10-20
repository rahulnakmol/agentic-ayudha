package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	aayudha "github.com/rahulnakmol/agentic-ayudha"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/journey"
)

// Story #796: a Ready to ship card shows its pull request as it is now. Each
// refresh reads it again, also when nothing changed its updated_at (dev took
// another PR, a check finished), and a card that GitHub would refuse says
// why and who acts next, and offers only opening it.
func TestInboxNotReady(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	g.RequiredChecks = []string{"build", "lint", "aayudha/pack", "protected-paths"} // pack also required: judged once, by its worst run
	g.ClassicChecks = []string{"docs"}                                              // classic branch protection requires one more
	heads := map[string]string{"a": strings.Repeat("a", 40), "b": strings.Repeat("b", 39) + "1", "c": strings.Repeat("c", 40), "a2": strings.Repeat("2", 40)}
	marked := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	mark := func(pr *fake.PR, head string, at time.Time) {
		cm := &fake.Comment{ID: int64(900 + len(g.Issues[pr.Number].Comments)), Body: ReadyToShipMarker + " head=" + head + " -->\n**Ready to ship.**", CreatedAt: at}
		cm.User.Login, cm.User.Type = "o", "User"
		g.Issues[pr.Number].Comments = append(g.Issues[pr.Number].Comments, cm)
	}
	checks := func(head, lint string) {
		var runs []fake.CheckRun
		for _, name := range []string{"aayudha/merge-gate", "aayudha/inspect", "aayudha/pack", "build", "lint", "docs", "protected-paths"} {
			r := fake.CheckRun{Name: name, Status: "completed", Conclusion: "success", Suite: &fake.Suite{ID: 1}}
			switch {
			case name == "protected-paths": // only a person's pass merges past it: the reason the card waits for one
				r.Conclusion = "failure"
			case name == "lint" && lint != "success" && lint != "failure":
				r.Status, r.Conclusion = lint, ""
			case name == "lint":
				r.Conclusion = lint
			}
			runs = append(runs, r)
		}
		g.CheckRuns[head] = runs
	}
	yes, no := true, false
	var a, b, c *fake.PR
	for i, name := range []string{"a", "b", "c"} {
		g.AddIssue("Story "+name, "x", "type:story", "status:in-progress")
		pr := g.AddPR("feat: "+name, "Refs #"+[]string{"1", "2", "3"}[i], "o", "feat/"+name, heads[name], "dev")
		g.Mu.Lock()
		pr.Mergeable, pr.MergeableState = &yes, "blocked" // blocked by protected-paths alone: still shippable
		mark(pr, heads[name], marked)
		g.Mu.Unlock()
		switch name {
		case "a":
			a = pr
			checks(heads["a"], "success")
		case "b":
			b = pr
			checks(heads["b"], "in_progress")
		case "c":
			c = pr
			checks(heads["c"], "queued")
		}
	}
	cl, err := github.New(github.Options{Token: "t", BaseURL: g.URL(), WriteGap: -1})
	if err != nil {
		t.Fatal(err)
	}
	src := newInboxSource(cl, "o/r", "dev", config.Identity{})
	card := func(items []InboxItem, n int) InboxItem {
		t.Helper()
		i := slices.IndexFunc(items, func(it InboxItem) bool { return it.Number == n })
		if i < 0 {
			t.Fatalf("no card for #%d: %+v", n, items)
		}
		return items[i]
	}
	ships := func(it InboxItem) bool {
		return slices.ContainsFunc(it.choices(), func(c InboxAction) bool { return strings.HasPrefix(c.Command, "aayu ship") })
	}
	items, err := src.gather(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	// Criterion 3, before: a check still running is not ready, and says so.
	if it := card(items, a.Number); it.Kind != InboxReady || !ships(it) {
		t.Errorf("a green PR blocked only by protected-paths is not shippable: %+v", it)
	}
	for _, n := range []int{b.Number, c.Number} {
		if it := card(items, n); it.Kind != InboxNotReady || ships(it) || it.Why != "isn't ready yet: its checks are still running (lint)." ||
			it.Next != "nothing for you; it comes back as Ready to ship when they are green." {
			t.Errorf("#%d while lint runs: %+v", n, it)
		}
	}

	live := newInboxModel("o/r", "o", "", true) // an open inbox, for the card that turns not ready below
	live.Update(inboxGatheredMsg{items: items})

	// Criteria 2 and 3: dev takes #7, which makes A conflict; B's lint fails
	// and C's goes green. None of A, B or C changes its updated_at.
	m := g.AddPR("feat: other", "Refs #99", "o", "feat/other", strings.Repeat("9", 40), "dev")
	g.Mu.Lock()
	merged := marked.Add(time.Hour)
	m.State, m.Merged, m.MergedAt = "closed", true, merged
	g.Issues[m.Number].State = "closed"
	a.Mergeable, a.MergeableState = &no, "dirty"
	checks(heads["b"], "failure")
	checks(heads["c"], "success")
	g.Requests = nil
	g.Mu.Unlock()
	if items, err = src.gather(t.Context()); err != nil {
		t.Fatal(err)
	}
	g.Mu.Lock()
	read := slices.ContainsFunc(g.Requests, func(r fake.Request) bool {
		return r.Method == "GET" && r.Path == "/repos/o/r/pulls/"+strconv.Itoa(a.Number)
	})
	g.Mu.Unlock()
	if !read {
		t.Errorf("the refresh did not read A's pull request again")
	}
	if it := card(items, a.Number); it.Kind != InboxNotReady || ships(it) || it.short() != "conflicts with dev" ||
		it.Why != "can't merge: it conflicts with dev, which took #7 since ship checked it." || it.Next != "its builder rebases it onto dev; nothing for you yet." {
		t.Errorf("A after dev took #7: %+v", it)
	}
	if it := card(items, b.Number); it.Kind != InboxNotReady || ships(it) || it.Why != "can't merge: its lint check failed on its latest commit." {
		t.Errorf("B after lint failed: %+v", it)
	}
	if it := card(items, c.Number); it.Kind != InboxReady || !ships(it) {
		t.Errorf("C after lint went green: %+v", it)
	}
	// A re-run on the same head turns C's lint red again: the next refresh
	// reads its checks again, though nothing else about C moved.
	g.Mu.Lock()
	checks(heads["c"], "failure")
	g.Mu.Unlock()
	again, err := src.gather(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if it := card(again, c.Number); it.Kind != InboxNotReady || ships(it) {
		t.Errorf("C after a re-run of lint failed on the same head: %+v", it)
	}
	g.Mu.Lock()
	checks(heads["c"], "success")
	g.Mu.Unlock()
	// In the open inbox, A's card turns not ready in place: one row, not a
	// gone Ready card beside a new one.
	live.Update(inboxGatheredMsg{items: items})
	if rows := slices.DeleteFunc(slices.Clone(live.items), func(it InboxItem) bool { return it.Number != a.Number }); len(rows) != 1 || rows[0].Kind != InboxNotReady || live.state[rows[0].key()] != "" {
		t.Errorf("A in the open inbox after the refresh: %d rows %+v, state %q", len(rows), rows, live.state["ready#"+strconv.Itoa(a.Number)])
	}

	// The full-screen inbox (criteria 1, 5, 7): A's card under Not ready
	// yet, counted apart from what needs you, with no ship choice, at 120x34
	// and 72x34, every line the terminal's width; d shows GitHub's facts.
	clock := marked.Add(2 * time.Hour)
	for _, size := range [][2]int{{120, 34}, {72, 34}} {
		tm := newInboxModel("o/r", "o", "", true)
		tm.now = func() time.Time { return clock }
		tm.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		tm.Update(inboxGatheredMsg{items: items, at: clock})
		for tm.item() < 0 || tm.items[tm.item()].Number != a.Number {
			teaKeys(tm, "j")
		}
		view := func() []string {
			tm.dirty = true
			return strings.Split(ansi.Strip(tm.View().Content), "\n")
		}
		// The card's words, read pane by pane: at 120 columns the card is
		// the right pane, so its lines are joined without the list's.
		words := func(lines []string) string {
			var card, all []string
			for _, l := range lines {
				all = append(all, l)
				if _, right, ok := strings.Cut(l, "│"); ok {
					card = append(card, right)
				}
			}
			return strings.Join(strings.Fields(strings.Join(card, " ")), " ") + " | " + strings.Join(strings.Fields(strings.Join(all, " ")), " ")
		}
		lines := view()
		flat := words(lines)
		for _, want := range []string{"aayu inbox · 1 needs you", "Not ready yet (2)", "#2 can't merge: it conflicts with dev, which took #7 since ship checked it.",
			"Next: its builder rebases it onto dev; nothing for you yet.", "Open #2 on GitHub", "R refresh", "read just now"} {
			if !strings.Contains(flat, want) {
				t.Errorf("%dx%d: no %q:\n%s", size[0], size[1], want, strings.Join(lines, "\n"))
			}
		}
		if strings.Contains(flat, "Ship PR #2") || regexp.MustCompile(`\b[0-9a-f]{7,40}\b|https?://|\b(GET|PUT|POST|PATCH)\b|\b[45][0-9][0-9]\b`).MatchString(flat) {
			t.Errorf("%dx%d: a ship choice or a technical word on the card:\n%s", size[0], size[1], strings.Join(lines, "\n"))
		}
		for i, l := range lines {
			if ansi.StringWidth(l) != size[0] {
				t.Errorf("%dx%d: line %d is %d cells: %q", size[0], size[1], i, ansi.StringWidth(l), l)
			}
		}
		if len(lines) != size[1] {
			t.Errorf("%dx%d: %d lines", size[0], size[1], len(lines))
		}
		if teaKeys(tm, "x"); !strings.Contains(tm.status, "#2 is not ready yet; only housekeeping can be selected.") {
			t.Errorf("%dx%d: x on a not-ready card: %q", size[0], size[1], tm.status)
		}
		teaKeys(tm, "d")
		flat = words(view())
		for _, want := range []string{"Details", "GitHub: mergeable false, mergeable state dirty", "Head commit aaaaaaa", "Merged into dev since: #7", "d hide details",
			// each required check by name and result, anything not green first (criterion 7)
			"Checks on aaaaaaa: protected-paths: failure, which only your pass merges past · merge-gate, inspect, pack, build, lint, docs: success"} {
			if !strings.Contains(flat, want) {
				t.Errorf("%dx%d after d: no %q:\n%s", size[0], size[1], want, flat)
			}
		}
		teaKeys(tm, "d")
		if flat = strings.Join(view(), "\n"); strings.Contains(flat, "mergeable state dirty") {
			t.Errorf("%dx%d: d again kept the details:\n%s", size[0], size[1], flat)
		}
	}

	// --list (criterion 6), as Markdown and as JSON: the same words, no ship.
	env := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "AAYU_AGENT": "1", "CLAUDECODE": "1"}
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, Assets: aayudha.Assets}
	code, out, errOut := run(t, env, opts, "inbox", "--list", "--repo", "o/r")
	if code != 0 || !strings.Contains(out, "1 needs you, 2 not ready yet") ||
		!strings.Contains(out, "**⏳ Not ready yet** (nothing for you yet; no action offered):") ||
		!strings.Contains(out, "[#2](https://github.com/o/r/pull/2) feat: a · can't merge: it conflicts with dev, which took \\#7 since ship checked it. Next: its builder rebases it onto dev; nothing for you yet.") ||
		!strings.Contains(out, "[#4](https://github.com/o/r/pull/4) feat: b · can't merge: its lint check failed on its latest commit. Next: its builder fixes it; nothing for you yet.") ||
		strings.Contains(out, "Ship PR #2") || strings.Contains(out, "Ship PR #4") {
		t.Errorf("--list: exit %d:\n%s\n%s", code, out, errOut)
	}
	tty := func(any) bool { return true }
	plain := Options{GitHubAPI: g.URL(), GitHubGap: -1, IsTerminal: tty, Assets: aayudha.Assets}
	if code, out, errOut := runIn(t, map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "XDG_STATE_HOME": t.TempDir()}, plain, "q", "inbox", "--plain", "--repo", "o/r"); code != 0 ||
		!strings.Contains(out, "Not ready yet (nothing for you):") || !strings.Contains(out, "#2 can't merge: it conflicts with dev, which took #7 since ship checked it.") ||
		!strings.Contains(out, "Next: its builder fixes it; nothing for you yet.") {
		t.Errorf("--plain: exit %d:\n%s\n%s", code, out, errOut)
	}
	var l inboxList
	if code, out, _ := run(t, env, opts, "inbox", "--list", "--json", "--repo", "o/r"); code != 0 || json.Unmarshal([]byte(out), &l) != nil || l.NeedsYou != 1 || l.NotReady != 2 {
		t.Errorf("--list --json: exit %d: %s", code, out)
	} else if it := card(l.Items, a.Number); it.Kind != InboxNotReady || it.Next == "" || len(it.Details) == 0 || len(it.Options) != 0 {
		t.Errorf("--list --json card for A: %+v", it)
	}
	// A Not ready yet card waits on its builder: the home screen (and so
	// aayu next) counts in the inbox, and as waiting for a person, only what
	// needs one (C), never A or B, so it never suggests shipping them.
	dir := gitInit(t, "dev", map[string]string{".aayudha/config.toml": "version = 1\n"})
	var home journey.State
	if code, out, _ := run(t, map[string]string{"AAYU_GITHUB_TOKEN": "human-token"}, Options{Dir: dir, GitHubAPI: g.URL(), GitHubGap: -1}, "--json", "--repo", "o/r"); code != 0 ||
		json.Unmarshal([]byte(out), &home) != nil || len(home.Inbox) != 1 || home.Inbox[0].Number != c.Number ||
		len(home.AwaitingHuman) != 1 || home.AwaitingHuman[0].Number != c.Number {
		t.Errorf("home with two not-ready cards: exit %d: %s", code, out)
	}
	// --list stays about a screen: past six not-ready cards it counts the rest.
	for i := range 7 {
		story := g.AddIssue("Story x", "x", "type:story", "status:in-progress")
		head := fmt.Sprintf("%040x", 0xd0+i) // a hex head, as the Ready marker records
		pr := g.AddPR("feat: x", fmt.Sprintf("Refs #%d", story.Number), "o", fmt.Sprintf("feat/x%d", i), head, "dev")
		g.Mu.Lock()
		pr.Mergeable, pr.MergeableState = &no, "dirty"
		mark(pr, head, marked)
		checks(head, "success")
		g.Mu.Unlock()
	}
	if code, out, _ := run(t, env, opts, "inbox", "--list", "--repo", "o/r"); code != 0 || !strings.Contains(out, "- and 3 more, in `aayu inbox`") || strings.Count(out, "Next: its builder") != inboxListLines {
		t.Errorf("--list with nine not-ready cards: exit %d:\n%s", code, out)
	}

	// GitHub still computing whether C can merge (null, unknown), as it is
	// on the first read after dev moves: the same refresh asks again, and
	// uses GitHub's answer once it has one (a conflict here).
	inboxMergeableWait = 0
	t.Cleanup(func() { inboxMergeableWait = time.Second })
	g.Mu.Lock()
	c.Mergeable, c.MergeableState, c.Computing = &no, "dirty", 2
	g.Mu.Unlock()
	if items, err = src.gather(t.Context()); err != nil {
		t.Fatal(err)
	}
	if it := card(items, c.Number); it.Kind != InboxNotReady || ships(it) || it.short() != "conflicts with dev" {
		t.Errorf("C after GitHub computed a conflict on the third read: %+v", it)
	}
	// Still unknown after three reads: an unknown state alone never blocks
	// (Story #796's assumption); its green checks keep it Ready, and ship's
	// own checks still apply.
	g.Mu.Lock()
	c.Mergeable, c.MergeableState, c.Computing = &yes, "blocked", 3
	g.Mu.Unlock()
	if items, err = src.gather(t.Context()); err != nil {
		t.Fatal(err)
	}
	if it := card(items, c.Number); it.Kind != InboxReady || !ships(it) {
		t.Errorf("C while GitHub still computes after three reads: %+v", it)
	}
	g.Mu.Lock()
	c.Computing = 0
	g.Mu.Unlock()

	// Behind dev: GitHub merges it while dev's rules do not require a PR up
	// to date; once they do, it is not ready.
	g.Mu.Lock()
	c.MergeableState = "behind"
	g.Mu.Unlock()
	if items, err = src.gather(t.Context()); err != nil {
		t.Fatal(err)
	}
	if it := card(items, c.Number); it.Kind != InboxReady || !ships(it) {
		t.Errorf("C behind dev, which does not require it up to date: %+v", it)
	}
	g.Mu.Lock()
	g.Strict = true
	g.Mu.Unlock()
	if items, err = src.gather(t.Context()); err != nil {
		t.Fatal(err)
	}
	if it := card(items, c.Number); it.Kind != InboxNotReady || ships(it) || it.short() != "behind dev" ||
		it.Why != "can't merge yet: it is behind dev, and dev only takes pull requests that are up to date; dev took #7 since ship checked it." ||
		it.Next != "its builder updates it from dev; nothing for you yet." {
		t.Errorf("C behind dev: %+v", it)
	}

	// Criterion 4, then 2: A is rebased onto dev. Until ship says Ready on the
	// new head, the card stays and says ship has to check it; once it has, A
	// is Ready to ship again.
	g.Mu.Lock()
	a.Head.SHA, a.Mergeable, a.MergeableState = heads["a2"], &yes, "blocked"
	g.Issues[a.Number].UpdatedAt = "2026-10-12T11:00:00Z" // a push changes it
	checks(heads["a2"], "success")
	g.Mu.Unlock()
	if items, err = src.gather(t.Context()); err != nil {
		t.Fatal(err)
	}
	if it := card(items, a.Number); it.Kind != InboxNotReady || ships(it) || it.Why != "can't ship yet: new commits were pushed since ship checked it, so ship has to check them first." {
		t.Errorf("A after a push: %+v", it)
	}
	g.Mu.Lock()
	mark(a, heads["a2"], marked.Add(3*time.Hour))
	g.Issues[a.Number].UpdatedAt = "2026-10-12T12:00:00Z" // a comment changes it
	g.Mu.Unlock()
	if items, err = src.gather(t.Context()); err != nil {
		t.Fatal(err)
	}
	if it := card(items, a.Number); it.Kind != InboxReady || !ships(it) || it.f.head != heads["a2"] {
		t.Errorf("A once ship said Ready on its new head: %+v", it)
	}

	// A gate check that never ran on C's head, then dev takes three more
	// and C conflicts: the card names two of them and counts the rest.
	g.Mu.Lock()
	c.MergeableState = "blocked"
	g.CheckRuns[heads["c"]] = slices.DeleteFunc(g.CheckRuns[heads["c"]], func(r fake.CheckRun) bool { return r.Name == "aayudha/pack" })
	g.Mu.Unlock()
	if items, err = src.gather(t.Context()); err != nil {
		t.Fatal(err)
	}
	if it := card(items, c.Number); it.Kind != InboxNotReady || ships(it) || it.short() != "pack not run" ||
		it.Why != "can't merge yet: its pack check has not run on its latest commit." || it.Next != "its builder re-runs its checks; nothing for you yet." {
		t.Errorf("C without a pack run: %+v", it)
	}
	for i := range 3 {
		m := g.AddPR("feat: more", "x", "o", "feat/more"+strconv.Itoa(i), strings.Repeat(strconv.Itoa(i+3), 40), "dev")
		g.Mu.Lock()
		m.State, m.Merged, m.MergedAt = "closed", true, merged.Add(time.Duration(i+1)*time.Minute)
		g.Issues[m.Number].State, g.Issues[m.Number].UpdatedAt = "closed", "2026-10-12T13:0"+strconv.Itoa(i)+":00Z" // its merge changes it
		g.Mu.Unlock()
	}
	g.Mu.Lock()
	c.Mergeable, c.MergeableState = &no, "dirty"
	g.Mu.Unlock()
	if items, err = src.gather(t.Context()); err != nil {
		t.Fatal(err)
	}
	if it := card(items, c.Number); !strings.HasPrefix(it.Why, "can't merge: it conflicts with dev, which took #7, #") || !strings.HasSuffix(it.Why, " and 2 more since ship checked it.") {
		t.Errorf("C after dev took four: %q", it.Why)
	}

	// protected-paths re-running is not ready like any other check: only its
	// finished failure is the person's to pass.
	g.Mu.Lock()
	for i, r := range g.CheckRuns[heads["a2"]] {
		if r.Name == "protected-paths" {
			g.CheckRuns[heads["a2"]][i].Status, g.CheckRuns[heads["a2"]][i].Conclusion = "queued", ""
		}
	}
	g.Mu.Unlock()
	if items, err = src.gather(t.Context()); err != nil {
		t.Fatal(err)
	}
	if it := card(items, a.Number); it.Kind != InboxNotReady || ships(it) || it.short() != "checks still running" {
		t.Errorf("A while protected-paths runs again: %+v", it)
	}
	g.Mu.Lock()
	checks(heads["a2"], "success")
	g.Mu.Unlock()

	// A check dev starts to require while the inbox is open counts from the
	// next refresh: A, Ready until now, has never run it.
	g.Mu.Lock()
	g.RequiredChecks = append(g.RequiredChecks, "security")
	g.Mu.Unlock()
	if items, err = src.gather(t.Context()); err != nil {
		t.Fatal(err)
	}
	if it := card(items, a.Number); it.Kind != InboxNotReady || it.short() != "security not run" {
		t.Errorf("A once dev requires security: %+v", it)
	}

	// A read that fails says so and makes no card from half of it.
	for _, path := range []string{"GET /repos/o/r/pulls/" + strconv.Itoa(a.Number), "GET /repos/o/r/commits/" + heads["a2"] + "/check-runs",
		"GET /repos/o/r/commits/" + heads["a2"] + "/statuses", "GET /repos/o/r/rules/branches/dev", "GET /repos/o/r/branches/dev", "GET /repos/o/r/collaborators/o/permission"} {
		once, err := github.New(github.Options{Token: "t", BaseURL: g.URL(), WriteGap: -1, ServerRetries: -1})
		if err != nil {
			t.Fatal(err)
		}
		g.Mu.Lock()
		g.Fail, g.FailAfter = path, 0
		g.Mu.Unlock()
		if items, err := newInboxSource(once, "o/r", "dev", config.Identity{}).gather(t.Context()); err == nil {
			t.Errorf("%s failed, yet the read made %d cards", path, len(items))
		}
		g.Mu.Lock()
		g.Fail = ""
		g.Mu.Unlock()
	}
}
