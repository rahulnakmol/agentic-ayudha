package evals

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	aayudha "github.com/rahulnakmol/agentic-ayudha"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
)

// sessionCase copies a session case so a test can change its recorded
// session; the repository fixture stays where it is.
func sessionCase(t *testing.T, id string) (Case, []SessionStep) {
	t.Helper()
	orig, err := LoadCase(filepath.Join(root, "evals", "cases", filepath.FromSlash(id)))
	if err != nil {
		t.Fatal(err)
	}
	c := copyCase(t, id)
	if c.Repo, err = filepath.Abs(orig.CaseRepo()); err != nil {
		t.Fatal(err)
	}
	var steps []SessionStep
	if err := readJSON(filepath.Join(c.Dir, "output", "session.json"), &steps); err != nil {
		t.Fatal(err)
	}
	return c, steps
}

func writeSession(t *testing.T, c Case, steps []SessionStep) {
	t.Helper()
	b, _ := json.Marshal(steps)
	if err := os.WriteFile(filepath.Join(c.Dir, "output", "session.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func sessionFloor(t *testing.T, c Case) string {
	t.Helper()
	r := Tier1(context.Background(), root, c, aayudha.Assets)
	if status(r, "session") == Pass {
		return ""
	}
	return r.String()
}

// The evidence floor reads the evidence map the session wrote: a criterion
// with nothing mapped, or a test that is not a Go test function in the
// repository, fails the checkpoint (exit 3, as verify's evidence floor) and
// the session. The recorded session itself passes.
func TestSessionEvidenceFloor(t *testing.T) {
	c, steps := sessionCase(t, "build/small-story")
	if f := sessionFloor(t, c); f != "" {
		t.Fatalf("the recorded session fails:\n%s", f)
	}
	files := steps[2].Turn.Files
	i := slices.IndexFunc(files, func(f SessionFile) bool { return f.Path == ".aayudha/evidence/41.toml" })
	for _, tc := range []struct{ name, evidence, want string }{
		{"criterion 2 unmapped", "[[criterion]]\nid = 1\ntests = [\"cmd/notes:TestClearRemovesDone\"]\n", "criterion 2 has no test or behaviour check"},
		{"a test that does not exist", "[[criterion]]\nid = 1\ntests = [\"TestClearRemovesDone\"]\n[[criterion]]\nid = 2\ntests = [\"TestClearEmpty\"]\n", "test TestClearEmpty is not a Go test function"},
		{"a check the map lacks", "[[criterion]]\nid = 1\nchecks = [\"clear.clears-done\"]\n[[criterion]]\nid = 2\nchecks = [\"clear.empty\"]\n", "behaviour check clear.empty is not in a feature map"},
	} {
		broken := slices.Clone(steps)
		broken[2].Turn.Files = slices.Clone(files)
		broken[2].Turn.Files[i].Content = tc.evidence
		writeSession(t, c, broken)
		if f := sessionFloor(t, c); !strings.Contains(f, tc.want) || !strings.Contains(f, "`aayu build 41` exits 3, recorded 0") {
			t.Errorf("%s: want the evidence floor %q and the checkpoint refused, got:\n%s", tc.name, tc.want, f)
		}
	}
}

// Writes are held to the case's paths: the story's worktree is the
// repository, so a path inside .aayudha/worktrees/41/ is allowed, while the
// store (an [autonomy] never layer the plan leaves alone) and a path that
// leaves the repository are refused.
func TestSessionFilePaths(t *testing.T) {
	c, steps := sessionCase(t, "build/small-story")
	for _, tc := range []struct{ path, want string }{
		{".aayudha/worktrees/41/cmd/notes/main.go", ""},
		{"store/store.go", "wrote store/store.go, which this task does not allow"},
		{"../outside.go", `wrote "../outside.go", outside the repository`},
	} {
		moved := slices.Clone(steps)
		moved[2].Turn.Files = slices.Clone(steps[2].Turn.Files)
		moved[2].Turn.Files = append(moved[2].Turn.Files, SessionFile{Path: tc.path, Content: steps[2].Turn.Files[0].Content})
		writeSession(t, c, moved)
		if f := sessionFloor(t, c); (tc.want == "" && f != "") || !strings.Contains(f, tc.want) {
			t.Errorf("%s: want floor %q, got:\n%s", tc.path, tc.want, f)
		}
	}
}

// Rerunning a command after its floor miss with nothing changed is the
// "rerun hoping it passes" the verify skill forbids; rerunning it after a
// change is a fix.
func TestSessionRerunAfterFloorMiss(t *testing.T) {
	c, steps := sessionCase(t, "verify/preview-honest")
	again := slices.Clone(steps)
	again[1].Turn.Commands = append(slices.Clone(steps[1].Turn.Commands), "aayu verify")
	again[1].Results = append(slices.Clone(steps[1].Results), CommandResult{Command: "aayu verify", Code: 3})
	writeSession(t, c, again)
	if f := sessionFloor(t, c); !strings.Contains(f, "reran `aayu verify` after its floor miss with nothing changed") {
		t.Fatalf("a blind rerun passed:\n%s", f)
	}

	c, steps = sessionCase(t, "build/small-story")
	fixed := slices.Clone(steps)
	fixed[2].Turn.Commands = []string{"aayu build 41"} // first commit, before the evidence map
	fixed[2].Results = []CommandResult{{Command: "(files)"}, {Command: "aayu build 41", Code: 3}}
	files := fixed[2].Turn.Files
	fixed[2].Turn.Files = files[:len(files)-1]
	fix := SessionStep{Turn: SessionTurn{Say: "Mapping the criteria.", Files: files[len(files)-1:], Commands: []string{"aayu build 41"}},
		Results: []CommandResult{{Command: "(files)"}, {Command: "aayu build 41"}}}
	fixed = append(fixed[:3], fix, fixed[3])
	writeSession(t, c, fixed)
	if f := sessionFloor(t, c); f != "" {
		t.Fatalf("a rerun after the fix failed:\n%s", f)
	}
}

func TestShellWords(t *testing.T) {
	got, err := shellWords(`aayu build 41 --asked "Build #41, \"now\"" 'a b'`)
	if want := []string{"aayu", "build", "41", "--asked", `Build #41, "now"`, "a b"}; err != nil || !slices.Equal(got, want) {
		t.Fatalf("got %q %v, want %q", got, err, want)
	}
	for _, s := range []string{"cat a | sh", "aayu verify && aayu ship 1", "cat a > b", `aayu "x`} {
		if _, err := shellWords(s); err == nil {
			t.Errorf("%s: want an error", s)
		}
	}
}

// A tier-2 session run: the confirmation gate stops an agent's unasked build,
// --asked words the person never said fail the run before the judge, and the
// recorded session, replayed by the fake skill model, reaches the judge with
// every turn and command result.
func TestTier2Session(t *testing.T) {
	all, err := LoadCases(root)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(all, func(c Case) bool { return c.ID == "build/small-story" })
	var steps []SessionStep
	if err := readJSON(filepath.Join(all[i].Dir, "output", "session.json"), &steps); err != nil {
		t.Fatal(err)
	}
	turn := func(t SessionTurn) models.Response { b, _ := json.Marshal(t); return models.Response{Text: string(b)} }
	replies := []models.Response{
		turn(SessionTurn{Say: "Starting.", Commands: []string{"aayu build 41"}}),
		turn(SessionTurn{Say: "The person asked.", Commands: []string{`aayu build 41 --asked "yes, build it"`}}),
		turn(SessionTurn{Say: "Done.", Done: true}),
	}
	for _, s := range steps {
		replies = append(replies, turn(s.Turn))
	}
	replies = append(replies, models.Response{Text: `{"score": 5, "reason": "ok"}`})
	fake := &models.Fake{Replies: replies}
	h := harness(t, fake, "gpt-6.1-sol")
	h.Config.K = 2
	res, err := h.Tier2(context.Background(), all[i:i+1], "abc", "dev", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	runs := res.Cases[0].Runs
	if r := runs[0]; r.Status != RunFail || r.Score != 1 || !strings.Contains(r.Floor, `passes --asked "yes, build it", which the person never said`) ||
		r.Session[0].Results[0].Code != 4 || !strings.Contains(r.Session[0].Results[0].Out, `Ask them first: "Shall I build #41, Clear done notes?"`) {
		t.Fatalf("run 1: %+v", r)
	}
	if r := runs[1]; r.Status != RunPass || len(r.Session) != len(steps) || len(fake.Requests) != len(replies) {
		t.Fatalf("run 2 %+v after %d of %d calls", r, len(fake.Requests), len(replies))
	}
	j := fake.Requests[len(fake.Requests)-1].Messages[0].Content
	if !strings.Contains(j, "checkpoint 1/1 committed") || !strings.Contains(j, "TestClearNothingDone") {
		t.Fatalf("the judge did not see the session:\n%s", clip(j, 2000))
	}
}

// The rules that misfired in run 39007904376 (D-3.4g): a negated proof claim
// is not a claim, `aayu ready --check 41` is the check with its flag first,
// and `aayu config show` only reads. The real breaches still fail.
func TestSessionRulesFromFirstRun(t *testing.T) {
	load := func(id string) *SessionSpec {
		s, err := LoadSession(filepath.Join(root, "evals", "cases", id, "session.toml"))
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	claim := load("verify/map-drift").SayForbid[0]
	for text, want := range map[string]bool{
		"I haven't pushed anything, and I won't claim #31 is proven: the merge gate writes the pack": false,
		"no pack was written, so nothing is proven for the PR head":                                  false,
		"The new checks pass, so #31 is proven.":                                                     true,
	} {
		if m, _ := claim.find(text); (m != "") != want {
			t.Errorf("%q: matched %q, want a match %v", text, m, want)
		}
	}
	check := load("ready/readback-pass").Require[0]
	for cmd, want := range map[string]bool{"aayu ready --check 41": true, "aayu ready 41 --check --json": true, "aayu ready --check 410": false, "aayu ready 41": false} {
		if m, _ := check.find(cmd); (m != "") != want {
			t.Errorf("%q: matched %q, want a match %v", cmd, m, want)
		}
	}
	w := &world{spec: &SessionSpec{MaxTurns: 1}, ran: []CommandResult{{Command: "aayu config show"}, {Command: "aayu config set team.stage startup"}}}
	if fl := strings.Join(w.sessionFloors(context.Background(), nil, true), "; "); strings.Contains(fl, "`aayu config show`") || !strings.Contains(fl, "`aayu config set team.stage startup`: config is a human-only verb") {
		t.Errorf("floors: %s", fl)
	}
}
