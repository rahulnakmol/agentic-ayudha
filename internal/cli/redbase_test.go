package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/verify"
)

// P5: a Bug fix's proving test is red on the base, a vacuous one green, and
// one that needs the fix's new API inconclusive; green blocks only from the
// configured date.
func TestRedOnBase(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(dir, rel)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git(t, dir, "init", "--quiet", "-b", "dev")
	git(t, dir, "config", "user.email", "t@example.com")
	git(t, dir, "config", "user.name", "t")
	write("go.mod", "module example.com/abs\n\ngo 1.25\n")
	write("abs.go", "package abs\n\n// Abs is wrong for negatives (the bug).\nfunc Abs(x int) int { return x }\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "--quiet", "-m", "base")
	base := git(t, dir, "rev-parse", "HEAD")
	write("abs.go", "package abs\n\n// Abs returns |x|.\nfunc Abs(x int) int {\n\tif x < 0 {\n\t\treturn -x\n\t}\n\treturn x\n}\n\n// Sign is new.\nfunc Sign(x int) int {\n\tif x < 0 {\n\t\treturn -1\n\t}\n\treturn 1\n}\n")
	write("abs_test.go", "package abs\n\nimport \"testing\"\n\nfunc TestAbsNegative(t *testing.T) {\n\tif Abs(-2) != 2 {\n\t\tt.Fatal(\"-2\")\n\t}\n}\n\nfunc TestAbsPositive(t *testing.T) {\n\tif Abs(2) != 2 {\n\t\tt.Fatal(\"2\")\n\t}\n}\n")
	write("sign/sign_test.go", "package sign\n\nimport (\n\t\"testing\"\n\n\t\"example.com/abs\"\n)\n\nfunc TestSign(t *testing.T) {\n\tif abs.Sign(-3) != -1 {\n\t\tt.Fatal(\"-3\")\n\t}\n}\n")
	write(".aayudha/evidence/7.toml", "[[criterion]]\nid = 1\ntests = [\"TestAbsNegative\"]\n\n[[criterion]]\nid = 2\ntests = [\"TestAbsPositive\"]\n\n[[criterion]]\nid = 3\ntests = [\"sign:TestSign\"]\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "--quiet", "-m", "fix: abs of a negative")
	head := git(t, dir, "rev-parse", "HEAD")

	sb := &verify.Sandbox{Home: t.TempDir(), NoNamespaces: true}
	day := time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC)
	check := func(now time.Time, from string) proof.Check {
		t.Helper()
		c, ok, err := redOnBase(t.Context(), sb, dir, base, head, 7, now, from)
		if err != nil || !ok {
			t.Fatalf("redOnBase: %v %v", ok, err)
		}
		return c
	}
	want := "criterion 1: red on base; criterion 2: green on base; criterion 3: inconclusive (base " + short(base) + ")"
	if c := check(day.Add(-time.Second), "2026-11-13"); c.Result != proof.Skipped || c.Detail != "advisory until 2026-11-13: "+want || c.Value != 1 || c.Threshold != 3 {
		t.Fatalf("the day before: %+v", c)
	}
	if c := check(day, "2026-11-13"); c.Result != proof.Fail || c.Detail != want {
		t.Fatalf("on the day: %+v", c)
	}
	if c := check(day.AddDate(1, 0, 0), ""); c.Result != proof.Skipped {
		t.Fatalf("no date stays advisory: %+v", c)
	}
	// Only the real proof mapped: it passes, whatever the date.
	write(".aayudha/evidence/7.toml", "[[criterion]]\nid = 1\ntests = [\"TestAbsNegative\"]\n\n[[criterion]]\nid = 3\ntests = [\"sign:TestSign\"]\n")
	if c := check(day, "2026-11-13"); c.Result != proof.Pass {
		t.Fatalf("red and inconclusive only: %+v", c)
	}
	if out := git(t, dir, "worktree", "list"); strings.Count(out, "\n") != 0 {
		t.Fatalf("worktree left behind:\n%s", out)
	}
}
