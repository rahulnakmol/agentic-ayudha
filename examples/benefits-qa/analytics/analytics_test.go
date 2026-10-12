package analytics

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/ask"
	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/store"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/exports"
)

// The recorder writes one Log Analytics JSON line per event, which aayudha's
// own Application Insights reader (the one `aayu measure` uses) accepts with
// nothing skipped; an event without a user is not exported.
func TestRecorderExportReadable(t *testing.T) {
	var b bytes.Buffer
	a := NewAppInsights(&b)
	at := time.Date(2026, 9, 14, 9, 30, 0, 0, time.FixedZone("IST", 19800))
	for _, e := range []ask.Event{{Name: ask.EventVisit, User: "emp-0001", Time: at}, {Name: ask.EventAsked, User: "emp-0001", Time: at}, {Name: ask.EventGrounded, User: "", Time: at}} {
		if err := a.Record(e); err != nil {
			t.Fatal(err)
		}
	}
	if want := `{"TimeGenerated":"2026-09-14T04:00:00Z","Name":"benefits_visit","UserAuthenticatedId":"emp-0001"}`; !strings.HasPrefix(b.String(), want+"\n") {
		t.Errorf("first line: %q", b.String())
	}
	p := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(p, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := exports.Read("appinsights", p)
	if err != nil || f.Rows != 2 || f.Skipped != 0 || f.Events[0].User != "emp-0001" || f.Events[1].Name != ask.EventAsked || !f.Events[0].TS.Equal(at) {
		t.Fatalf("%v %+v %+v", err, f, f.Events)
	}
}

// The committed exports the Goal is measured on are exactly what the
// simulation gives for their seeds, so the baseline and the Result can be
// re-derived from the code.
func TestExportsReproducible(t *testing.T) {
	s, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for file, cfg := range map[string]Config{
		"hr-ticket-seed1-2026-08-10.jsonl":   {Channel: ChannelHRTicket, Seed: 1, From: day("2026-08-10"), Days: 28, Users: 120},
		"benefits-qa-seed2-2026-09-14.jsonl": {Channel: ChannelBenefitsQA, Seed: 2, From: day("2026-09-14"), Days: 28, Users: 120},
	} {
		want, err := os.ReadFile(filepath.Join("testdata", file))
		if err != nil {
			t.Fatal(err)
		}
		var got bytes.Buffer
		if _, err := Simulate(cfg, s, &got); err != nil {
			t.Fatal(err)
		}
		if sha256.Sum256(got.Bytes()) != sha256.Sum256(want) {
			t.Errorf("%s does not match the simulation of %+v", file, cfg)
		}
	}
}

// Through the product, every question runs the real ask workflow: each
// question_asked is followed at once by exactly one outcome event for the same
// user and instant, refusals and referrals included, and the only other
// grounded answers are HR's, days later. Before the product (HR tickets)
// there is no workflow event, only visits and late HR answers.
func TestSimulateUsesWorkflow(t *testing.T) {
	s, _ := store.Load()
	cfg := Config{Channel: ChannelBenefitsQA, Seed: 7, From: day("2026-09-14"), Days: 28, Users: 60}
	r, err := simulate(cfg, s)
	if err != nil {
		t.Fatal(err)
	}
	outcomes := map[string]int{}
	asked := 0
	for i, e := range r.workflow {
		switch e.Name {
		case ask.EventVisit:
		case ask.EventAsked:
			asked++
			if i+1 >= len(r.workflow) {
				t.Fatalf("question %d has no outcome", asked)
			}
			o := r.workflow[i+1]
			if o.User != e.User || !o.Time.Equal(e.Time) || (o.Name != ask.EventGrounded && o.Name != ask.EventReferred && o.Name != ask.EventRefused) {
				t.Fatalf("question_asked %+v is followed by %+v, not by its outcome", e, o)
			}
			outcomes[o.Name]++
		case ask.EventGrounded, ask.EventReferred, ask.EventRefused:
			if i == 0 || r.workflow[i-1].Name != ask.EventAsked {
				t.Fatalf("outcome %+v without its question", e)
			}
		default:
			t.Fatalf("unexpected workflow event %+v", e)
		}
	}
	if asked != r.st.Questions || outcomes[ask.EventGrounded] == 0 || outcomes[ask.EventReferred] == 0 || outcomes[ask.EventRefused] == 0 {
		t.Errorf("%d questions, %d asked, outcomes %v", r.st.Questions, asked, outcomes)
	}
	for _, e := range r.hr {
		if e.Name != ask.EventGrounded {
			t.Errorf("HR event %+v", e)
		}
	}
	if len(r.hr) == 0 || len(r.hr) > outcomes[ask.EventReferred] {
		t.Errorf("%d HR answers for %d referred questions", len(r.hr), outcomes[ask.EventReferred])
	}

	cfg.Channel = ChannelHRTicket
	hr, err := simulate(cfg, s)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range hr.workflow {
		if e.Name != ask.EventVisit {
			t.Errorf("hr-ticket channel recorded a workflow event: %+v", e)
		}
	}
	if len(hr.workflow) != 60 || len(hr.hr) == 0 || len(hr.hr) >= hr.st.Questions {
		t.Errorf("hr-ticket: %d visits, %d HR answers for %d questions", len(hr.workflow), len(hr.hr), hr.st.Questions)
	}
	if _, err := Simulate(Config{Channel: "email", Days: 28, Users: 1}, s, &bytes.Buffer{}); err == nil {
		t.Error("unknown channel accepted")
	}
	if _, err := Simulate(Config{Channel: ChannelHRTicket, Days: Horizon, Users: 1}, s, &bytes.Buffer{}); err == nil {
		t.Error("a window no longer than the horizon accepted")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

// A write error reaches the caller, so a truncated export is never reported as written.
func TestSimulateWriteError(t *testing.T) {
	s, _ := store.Load()
	if _, err := Simulate(Config{Channel: ChannelHRTicket, Seed: 1, From: day("2026-08-10"), Days: 28, Users: 2}, s, failingWriter{}); err == nil || err.Error() != "disk full" {
		t.Errorf("write error: %v", err)
	}
	if err := NewAppInsights(failingWriter{}).Record(ask.Event{Name: ask.EventVisit, User: "emp-1"}); err == nil {
		t.Error("recorder write error lost")
	}
}

func day(s string) time.Time {
	t, _ := time.Parse(time.DateOnly, s)
	return t
}
