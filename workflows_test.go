package aayudha

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
)

// TestEveryWorkflowJobStartsWithHaltCheck holds the WP5.7 rule that `aayu halt` stops every workflow (Bug #173):
// each job with steps, in this repository's workflows and in the templates `init --gates` installs, starts with the
// halt check. The step runs only while vars.AAYUDHA_HALT is set, and its script must fail, so GitHub runs no
// later step of the job.
func TestEveryWorkflowJobStartsWithHaltCheck(t *testing.T) {
	var files []string
	for _, pattern := range []string{".github/workflows/*.yml", ".github/workflows/*.yaml", "internal/initrepo/*.yml", "internal/initrepo/*.yaml"} {
		m, _ := filepath.Glob(pattern)
		files = append(files, m...)
	}
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

// TestHaltedWorkflowRuns holds Bug #173's live runs: floors.yml, inspect-live.yml, and screen-reader.yml started on
// a scratch repository holding this branch, once while AAYUDHA_HALT was set and once after it was cleared, recorded
// from GitHub's jobs API in testdata/workflow-halt (see its README.md). Halted, every job failed at the halt check
// and skipped every later step; resumed, the halt check was skipped and the job ran its steps. The recorded jobs
// must still be these workflows' jobs, each starting with the halt check, so a renamed or reordered job is noticed.
func TestHaltedWorkflowRuns(t *testing.T) {
	type step struct {
		Number     int    `json:"number"`
		Name       string `json:"name"`
		Conclusion string `json:"conclusion"`
	}
	type job struct {
		Name       string `json:"name"`
		Conclusion string `json:"conclusion"`
		Steps      []step `json:"steps"`
	}
	for _, wf := range []string{"floors", "inspect-live", "screen-reader"} {
		b, err := os.ReadFile(filepath.Join(".github", "workflows", wf+".yml"))
		if err != nil {
			t.Fatal(err)
		}
		var def struct {
			Jobs map[string]struct {
				Name  string `yaml:"name"`
				Steps []struct {
					Name string `yaml:"name"`
				} `yaml:"steps"`
			} `yaml:"jobs"`
		}
		if err := yaml.Unmarshal(b, &def); err != nil {
			t.Fatal(err)
		}
		for _, phase := range []string{"halted", "resumed"} {
			var jobs []job
			b, err := os.ReadFile(filepath.Join("testdata", "workflow-halt", phase+"-"+wf+"-jobs.json"))
			if err == nil {
				err = json.Unmarshal(b, &jobs)
			}
			if err != nil || len(jobs) == 0 {
				t.Fatalf("%s %s: %v", phase, wf, err)
			}
			for _, j := range jobs {
				known := false
				for _, d := range def.Jobs {
					base, _, _ := strings.Cut(d.Name, " (${{")
					known = known || strings.HasPrefix(j.Name, base) && strings.HasPrefix(d.Steps[0].Name, "Halt check")
				}
				if !known || len(j.Steps) < 3 || j.Steps[0].Name != "Set up job" || !strings.HasPrefix(j.Steps[1].Name, "Halt check") {
					t.Errorf("%s %s: job %q is not a job of %s.yml that starts with the halt check: %+v", phase, wf, j.Name, wf, j.Steps)
					continue
				}
				work := j.Steps[2 : len(j.Steps)-1] // between the halt check and "Complete job"
				switch phase {
				case "halted":
					ran := slices.ContainsFunc(work, func(s step) bool { return s.Conclusion != "skipped" })
					if j.Conclusion != "failure" || j.Steps[1].Conclusion != "failure" || ran {
						t.Errorf("halted %s: job %q %s, halt check %s, a later step ran %v", wf, j.Name, j.Conclusion, j.Steps[1].Conclusion, ran)
					}
				case "resumed":
					if j.Steps[1].Conclusion != "skipped" || len(work) == 0 || work[0].Conclusion != "success" || j.Conclusion != "success" {
						t.Errorf("resumed %s: job %q %s, halt check %s, next step %+v", wf, j.Name, j.Conclusion, j.Steps[1].Conclusion, work)
					}
				}
			}
		}
	}
}
