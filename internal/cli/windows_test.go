package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
)

// Story #1254, criterion 1: in a repository whose config sets no [time]
// windows, `aayu config show` lists the two default windows, 10:30-11:30 and
// 18:30-19:30, with the zone they are in.
func TestConfigShowListsPersonWindows(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		".git/HEAD":            "ref: refs/heads/dev\n",
		".aayudha/config.toml": "version = 1\n[time]\ntz = \"Asia/Kolkata\"\n",
	} {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	code, out, errOut := run(t, map[string]string{windowsKey: defaultWindows}, Options{Dir: dir}, "config", "show")
	want := "[time]\nbusiness_hours = 'Mon-Fri 09:00-18:00'\ntz = 'Asia/Kolkata'\nwindows = ['10:30-11:30', '18:30-19:30']\n"
	if code != ExitOK || !strings.Contains(out, want) {
		t.Fatalf("config show: exit %d, want %q in:\n%s%s", code, want, out, errOut)
	}
}

// Story #1254, criterion 5: outside a window, nobody is asked to act: a
// mission's time card and an agent's comment approval batch are not posted,
// and no message is sent. At the next window's start, the hourly tick sends
// one message that lists both queued requests; the card is shown in the
// window, and the Decision waits in the inbox.
func TestRequestsWaitForAWindow(t *testing.T) {
	m := startedTwo(t, 25)
	m.issue(5, "Choose the rules engine", "decision", 0, "")
	m.edit(func() { m.g.Issues[5].Body = decisionBody("true", "") })
	sink := m.windows()
	t0 := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	for _, n := range []int{20, 21} {
		m.status(n, "in-progress", "")
		m.delegation(n, t0, 0)
	}
	m.status(25, "ready", "")
	m.opts.Now = func() time.Time { return t0.Add(48*time.Hour + time.Minute) } // 01:01 on 22 October, London
	if code, out, errOut := m.next(889); code != 0 || out != "start #25\n" {
		t.Fatalf("mission next at 01:01: exit %d: %q%s", code, out, errOut)
	}
	code, out, errOut := run(t, m.agent, m.opts, "decide", "5", "--accept", "1", "--repo", "o/r")
	if code != ExitBlocked || !strings.Contains(errOut, "outside the person's windows ([time] windows), nobody is asked to act: no batch was posted, and the request waits for the window at Thu 22 Oct 10:30 BST") {
		t.Fatalf("an agent's batch at 01:01: exit %d: %s%s", code, out, errOut)
	}
	batches := func() int {
		m.g.Mu.Lock()
		defer m.g.Mu.Unlock()
		return len(slices.DeleteFunc(slices.Clone(m.g.Issues[5].Comments), func(c *fake.Comment) bool { return !strings.Contains(c.Body, batchMarker) }))
	}
	if batches() != 0 || m.timeComments(889) != 0 || len(sink.Pages) != 0 {
		t.Fatalf("outside a window: %d batches, %d time cards, pages %+v", batches(), m.timeComments(889), sink.Pages)
	}
	m.opts.Now = func() time.Time { return london(10, 30) }
	if code, out, errOut := run(t, m.lane, m.opts, "mission", "tick", "--repo", "o/r"); code != 0 {
		t.Fatalf("the tick at 10:30: exit %d: %s%s", code, out, errOut)
	}
	if len(sink.Pages) != 1 || sink.Pages[0].Subject != "aayudha: 2 requests for your window" ||
		!strings.Contains(sink.Pages[0].Body, "1. #889: mission 1's time card (48 h, limit)") || !strings.Contains(sink.Pages[0].Body, "2. #5 waits for your approval\n   1. Accepts option 1 (A) of Decision #5") {
		t.Fatalf("the window's one message: %+v", sink.Pages)
	}
	if code, _, errOut := m.next(889); code != 0 || m.timeComments(889) != 1 || len(sink.Pages) != 1 {
		t.Fatalf("mission next in the window: exit %d, %d time cards, %d messages: %s", code, m.timeComments(889), len(sink.Pages), errOut)
	}
	if keys, _ := m.listed(m.person); !slices.Contains(keys, "decision#5") || !slices.Contains(keys, "mission-time#889") {
		t.Fatalf("the window's inbox: %v", keys)
	}
}
