package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
)

// Story #1177: the evening says the day is proven only when every story
// agreed this morning merged, every criterion has its evidence in the pack,
// and he accepted each.

// The proven day's words: three stories with two criteria each.
const (
	dayProvenHead  = "🎉 The day is proven: three stories and six criteria, each with its evidence. Job well done."
	dayProvenVoice = "Every criterion you agreed this morning has its evidence in a pack, and you accepted each story. I looked for a gap and found none."
)

const head1104 = "1104110411041104110411041104110411041104"

// provenWorld is eveningWorld's day with every story proven: #1011 has an
// authoritative pack at its merged head with evidence for each criterion,
// and #1104, no longer stopped, merges with one too.
func provenWorld(t *testing.T, before ...func(*missionWorld)) *missionWorld {
	t.Helper()
	m, _ := eveningWorld(t, before...)
	m.setPlan(889, func(p *MissionPlan) { p.Stopped = nil })
	pr := m.g.AddPR("feat: story 1104", "Refs #1104\n", "agent", "story/1104-x", head1104, "dev")
	m.edit(func() {
		pr.State, pr.Merged, pr.MergedAt, pr.MergeCommitSHA = "closed", true, time.Date(2026, 11, 2, 16, 0, 0, 0, time.UTC), strings.Repeat("e", 39)+"4"
	})
	m.status(1104, "merged", "")
	tracePack(t, m, 1011, head1011, 1, 2)
	tracePack(t, m, 1104, head1104, 1, 2)
	return m
}

// tracePack lands story n's authoritative pack at head, with an API trace on
// the ledger as the evidence of each criterion in crit.
func tracePack(t *testing.T, m *missionWorld, n int, head string, crit ...int) {
	t.Helper()
	p := &proof.Pack{Story: n, PRHeadSHA: head, Authority: proof.Authority{Kind: proof.Authoritative, RunID: int64(n)}}
	files := map[string][]byte{}
	for _, c := range crit {
		rel := fmt.Sprintf("evidence/criterion-%d.jsonl", c)
		b := []byte(fmt.Sprintf(`{"Action":"pass","Test":"Test%dCriterion%d"}`+"\n", n, c))
		p.Criteria = append(p.Criteria, proof.Criterion{ID: c, Evidence: []string{rel}})
		p.Artifacts, files[rel] = append(p.Artifacts, proof.NewArtifact(rel, "api-trace", b)), b
	}
	putPack(t, m.bare, p, files)
}

// milestones are the milestones this checkout recorded, by key.
func (m *missionWorld) milestones() map[string]string {
	m.t.Helper()
	got := map[string]string{}
	for _, p := range []string{filepath.Join(m.dir, ".aayudha", "state", "milestones.json"), filepath.Join(m.person["XDG_STATE_HOME"], "aayudha", "state", "milestones.json")} {
		if b, err := os.ReadFile(p); err == nil {
			_ = json.Unmarshal(b, &got)
		}
	}
	return got
}

// Criterion 1: every story agreed this morning merged, each criterion has its
// evidence in the pack, and he accepts each: the inbox says The day is proven
// with the count of stories and criteria, in the inbox's cheer (with motion,
// its confetti), and records the milestone day-proven for the date. Until the
// last accept lands there is no such line: an accept in an earlier session
// counts from the ledger.
func TestDayIsProven(t *testing.T) {
	t.Run("line by line", func(t *testing.T) {
		m := provenWorld(t)
		code, out, errOut := runIn(t, m.person, m.opts, "1y1yq", "inbox", "--repo", "o/r") // #1010 and #1011, then he stops
		if code != 0 || strings.Count(out, "done: @rahulnakmol accepted") != 2 || strings.Contains(out, "The day is proven") || len(m.milestones()) != 0 {
			t.Fatalf("two of three accepted: exit %d, milestones %v:\n%s%s", code, m.milestones(), out, errOut)
		}
		code, out, errOut = runIn(t, m.person, m.opts, "1y", "inbox", "--repo", "o/r") // #1104, the last
		accepted := strings.Index(out, "done: @rahulnakmol accepted #1104")
		proven := strings.Index(out, "\n"+dayProvenHead+"\n  Quality Architect: "+dayProvenVoice+"\n\ninbox: 1 done, 0 skipped, 0 left\n")
		if code != 0 || accepted < 0 || proven < accepted || strings.Count(out, "The day is proven") != 1 || strings.Contains(out, "Inbox zero") {
			t.Fatalf("the last accept: exit %d:\n%s%s", code, out, errOut)
		}
		if got := m.milestones(); len(got) != 1 || got["day-proven:2026-11-02"] != "2026-11-02T18:00:00Z" {
			t.Fatalf("milestones: %v", got)
		}
	})
	t.Run("full screen", func(t *testing.T) {
		m := provenWorld(t)
		var cheer *inboxCheer
		var pane string
		opts := m.opts
		opts.TUI = func(tm tea.Model) error {
			im := tm.(*inboxModel)
			im.motion = true
			teaDrive(im, im.Init())
			for range 3 { // each Evening card: Enter on its preselected accept, and Enter again
				im.cur = slices.IndexFunc(im.rows, func(i int) bool { return im.items[i].Kind == InboxEvening && im.state[im.items[i].key()] == "" })
				if im.cur < 0 {
					t.Fatalf("an Evening card is missing: %+v", im.items)
				}
				teaKeys(im, "enter", "enter")
			}
			cheer = im.cheer
			im.dirty = true
			pane = ansi.Strip(im.View().Content)
			teaKeys(im, "q")
			return nil
		}
		code, out, errOut := runIn(t, m.person, opts, "", "inbox", "--repo", "o/r")
		if code != 0 || cheer == nil || "🎉 "+cheer.head != dayProvenHead || cheer.voice != dayProvenVoice || len(m.reviews()) != 3 {
			t.Fatalf("full-screen inbox: exit %d, cheer %+v, reviews %+v:\n%s%s", code, cheer, m.reviews(), out, errOut)
		}
		flat := strings.Join(strings.Fields(pane), " ")
		if !strings.Contains(flat, "🎉 Job well done The day is proven: three stories and six criteria, each with its evidence.") || !strings.ContainsAny(pane, "*+•°~✦") {
			t.Fatalf("the cheer, mid-burst:\n%s", pane)
		}
		if !strings.Contains(out, "\n"+dayProvenHead+"\n") || m.milestones()["day-proven:2026-11-02"] == "" {
			t.Fatalf("after the screen closes: milestones %v:\n%s%s", m.milestones(), out, errOut)
		}
	})
}

// Criterion 2: criterion 3 of #1011 has no evidence in its pack: there is no
// proven line, even once he accepts every card, and #1011's card says
// `#1011 criterion 3 has no evidence` and the next step. Evidence that names
// no artifact of the pack is no evidence either.
func TestDayNotProvenWithoutEvidence(t *testing.T) {
	third := func(m *missionWorld) {
		m.edit(func() {
			is := m.g.Issues[1011]
			is.Body = strings.Replace(is.Body, "422 with code cart.empty", "422 with code cart.empty\n3. [api-trace] state: GET /cart after it; expected: 200 with no items", 1)
		})
	}
	for name, pack := range map[string]func(*testing.T, *missionWorld){
		"unmapped": func(*testing.T, *missionWorld) {},
		"no artifact": func(t *testing.T, m *missionWorld) {
			trace := []byte(`{"Action":"pass"}` + "\n")
			putPack(t, m.bare, &proof.Pack{Story: 1011, PRHeadSHA: head1011, Authority: proof.Authority{Kind: proof.Authoritative, RunID: 1011},
				Criteria: []proof.Criterion{{ID: 1, Evidence: []string{"evidence/criterion-1.jsonl"}}, {ID: 2, Evidence: []string{"evidence/criterion-2.jsonl"}},
					{ID: 3, Evidence: []string{"evidence/criterion-3.jsonl"}}}, // the pack holds no such artifact
				Artifacts: []proof.Artifact{proof.NewArtifact("evidence/criterion-1.jsonl", "api-trace", trace), proof.NewArtifact("evidence/criterion-2.jsonl", "api-trace", trace)}},
				map[string][]byte{"evidence/criterion-1.jsonl": trace, "evidence/criterion-2.jsonl": trace})
		},
	} {
		t.Run(name, func(t *testing.T) {
			m := provenWorld(t, third)
			pack(t, m)
			code, out, errOut := runIn(t, m.person, m.opts, "1y1y1y", "inbox", "--repo", "o/r")
			card := "  3. [api-trace] state: GET /cart after it; expected: 200 with no items\n"
			lines := "  #1011 criterion 3 has no evidence.\n" +
				"  Next: say what is missing in a comment on #1011, and the mission's feedback pass (#1014) builds its evidence.\n"
			if code != 0 || strings.Count(out, "done: @rahulnakmol accepted") != 3 || !strings.Contains(out, card) || !strings.Contains(out, lines) ||
				strings.Count(out, "has no evidence") != 1 || strings.Contains(out, "The day is proven") || len(m.milestones()) != 0 {
				t.Fatalf("inbox: exit %d, milestones %v:\n%s%s", code, m.milestones(), out, errOut)
			}
		})
	}
}

// Criterion 3: with `[ui] plain = true`, the inbox is the line-by-line one,
// even where the full-screen one would run: the same words, with no motion
// and no escape codes.
func TestDayProvenPlain(t *testing.T) {
	m := provenWorld(t)
	cfg := filepath.Join(m.dir, ".aayudha", "config.toml")
	b, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, append(b, "[ui]\nplain = true\n"...), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := m.opts
	opts.IsTerminal = func(any) bool { return true }
	opts.TUI = func(tea.Model) error { t.Error("[ui] plain started the full-screen inbox"); return nil }
	code, out, errOut := runIn(t, m.person, opts, "1y1y1y", "inbox", "--repo", "o/r")
	if code != 0 || !strings.Contains(out, "\n"+dayProvenHead+"\n  Quality Architect: "+dayProvenVoice+"\n") || strings.ContainsAny(out+errOut, "\x1b\r") {
		t.Fatalf("plain inbox: exit %d:\n%q%q", code, out, errOut)
	}
}

// Criterion 4: no Feature was started this morning: the evening has nothing
// to prove, so there is no proven line, though yesterday's stories are all
// merged, proven, and accepted.
func TestNoDayToProve(t *testing.T) {
	m := provenWorld(t)
	if code, out, errOut := runIn(t, m.person, m.opts, "1y1y1y", "inbox", "--repo", "o/r"); code != 0 || !strings.Contains(out, dayProvenHead) {
		t.Fatalf("Monday's inbox: exit %d:\n%s%s", code, out, errOut)
	}
	m.mu.Lock()
	m.opts.Now = func() time.Time { return time.Date(2026, 11, 3, 18, 0, 0, 0, time.UTC) } // Tuesday: MS2 waits for MS1's gate
	m.mu.Unlock()
	if cards := eveningList(t, m, m.person); len(cards) != 0 {
		t.Fatalf("Evening cards on Tuesday: %+v", cards)
	}
	code, out, errOut := runIn(t, m.person, m.opts, "", "inbox", "--repo", "o/r")
	if got := m.milestones(); code != 0 || strings.Contains(out, "The day is proven") || got["day-proven:2026-11-03"] != "" {
		t.Fatalf("Tuesday's inbox: exit %d, milestones %v:\n%s%s", code, got, out, errOut)
	}
}
