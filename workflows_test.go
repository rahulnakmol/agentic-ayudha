package aayudha

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
)

// TestEveryWorkflowJobStartsWithHaltCheck holds the WP5.7 rule that `aayu halt` stops every workflow (Bug #173):
// each job with steps, in this repository's workflows and in the templates `init --gates` installs, starts with the
// halt check. The step runs only while vars.AAYUDHA_HALT is set, and its script must fail, so GitHub runs no
// later step of the job.
func TestEveryWorkflowJobStartsWithHaltCheck(t *testing.T) {
	files, _ := filepath.Glob(".github/workflows/*.yml")
	tmpl, _ := filepath.Glob("internal/initrepo/*.yml")
	files = append(files, tmpl...)
	if len(files) < 10 {
		t.Fatalf("found only %d workflow files", len(files))
	}
	type step struct {
		Name string `yaml:"name"`
		If   string `yaml:"if"`
		Run  string `yaml:"run"`
	}
	var scripts []string
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var wf struct {
			Jobs map[string]struct {
				Steps []step `yaml:"steps"`
			} `yaml:"jobs"`
		}
		if err := yaml.Unmarshal([]byte(strings.ReplaceAll(string(b), "{{VERSION}}", "dev")), &wf); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for name, job := range wf.Jobs {
			if len(job.Steps) == 0 {
				continue // a job that calls a reusable workflow has no steps of its own
			}
			s := job.Steps[0]
			if !strings.HasPrefix(s.Name, "Halt check") || s.If != "vars.AAYUDHA_HALT != ''" || !strings.Contains(s.Run, "exit ") {
				t.Errorf("%s: job %s does not start with the halt check step (aayu halt, WP5.7); its first step is %q", f, name, s.Name)
				continue
			}
			scripts = append(scripts, s.Run)
		}
	}
	if runtime.GOOS == "windows" {
		return
	}
	// The halt check's script, run as GitHub runs it while halted, fails with the halt message.
	for _, run := range scripts {
		cmd := exec.Command("bash", "-c", run)
		cmd.Env = append(os.Environ(), "AAYUDHA_HALT=halted by test at 2026-10-15T00:00:00Z")
		out, err := cmd.CombinedOutput()
		if err == nil || !strings.Contains(string(out), "Halted by aayu halt") {
			t.Errorf("halt check %q: want a failing exit and the halt message, got %v: %s", run, err, out)
		}
	}
}
