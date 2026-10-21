package agentenv

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v4"
)

// GateWorkflows are the workflows whose pull-request jobs the local gate
// mirrors: the repository's CI workflow, and aayudha's protected-path gate
// where it is installed (Story #399).
var GateWorkflows = []string{".github/workflows/ci.yml", ".github/workflows/ci.yaml", ".github/workflows/protected-paths.yml"}

// inputsOnly matches an `if:` that compares only workflow_dispatch inputs,
// which are empty on a pull request, so the job never runs there.
var inputsOnly = regexp.MustCompile(`inputs\.[A-Za-z0-9_-]+\s*[!=]=\s*'[^']*'|[()|&\s]`)

// gate records every job of GateWorkflows that runs on a pull request, in the
// workflow's order. The commands come from the spec already committed
// (KeepPins): the workflows do not say how to run a job here.
func (d *discoverer) gate() {
	for _, wf := range GateWorkflows {
		jobs, err := PullRequestJobs(d.read(wf))
		if err != nil {
			d.spec.Notes = append(d.spec.Notes, fmt.Sprintf("%s: %v; the local gate does not mirror it", wf, err))
			continue
		}
		for _, j := range jobs {
			d.spec.Gate = append(d.spec.Gate, GateJob{Job: j, Workflow: wf})
		}
	}
}

// PullRequestJobs are the check names of a workflow's jobs that run on a pull
// request: none unless it triggers on pull_request or pull_request_target,
// and not a job whose `if` compares only workflow_dispatch inputs.
func PullRequestJobs(src []byte) ([]string, error) {
	if len(src) == 0 {
		return nil, nil
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, nil
	}
	top := doc.Content[0]
	on, jobs := value(top, "on"), value(top, "jobs")
	if on == nil || jobs == nil || (!mentions(on, "pull_request") && !mentions(on, "pull_request_target")) {
		return nil, nil
	}
	var out []string
	for i := 0; i+1 < len(jobs.Content); i += 2 {
		id, job := jobs.Content[i].Value, jobs.Content[i+1]
		if cond := value(job, "if"); cond != nil && strings.Contains(cond.Value, "inputs.") && inputsOnly.ReplaceAllString(cond.Value, "") == "" {
			continue
		}
		name := id
		if n := value(job, "name"); n != nil && n.Value != "" && !strings.Contains(n.Value, "${{") {
			name = n.Value
		}
		out = append(out, name)
	}
	return out, nil
}

func value(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// mentions reports whether the `on:` node names the event: as a scalar, a
// list item, or a key.
func mentions(on *yaml.Node, event string) bool {
	switch on.Kind {
	case yaml.ScalarNode:
		return on.Value == event
	case yaml.SequenceNode:
		for _, c := range on.Content {
			if c.Value == event {
				return true
			}
		}
	case yaml.MappingNode:
		return value(on, event) != nil
	}
	return false
}

// GateGaps are the jobs the local gate has neither commands nor a reason for.
func GateGaps(s Spec) []string {
	var out []string
	for _, g := range s.Gate {
		if len(g.Run) == 0 && strings.TrimSpace(g.Remote) == "" {
			out = append(out, g.Job+" ("+g.Workflow+")")
		}
	}
	return out
}

// keepGate carries the commands and reasons of the committed spec's gate jobs
// over to the jobs discovered again, by job name.
func keepGate(s *Spec, old Spec) {
	for i := range s.Gate {
		for _, o := range old.Gate {
			if o.Job == s.Gate[i].Job {
				s.Gate[i].Run, s.Gate[i].Remote = o.Run, o.Remote
			}
		}
	}
}

// gateScript is the `--full` part of the check script: each required CI job in
// turn, named as CI names it. A job's commands stop at the first failure; the
// gate runs every job, then lists the failed ones and exits 1. What runs only
// in CI is printed, never skipped silently.
func gateScript(s Spec) string {
	if len(s.Gate) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`if [ "${1:-}" = --full ]; then
  # The local gate (Story #399): the CI jobs a pull request must pass, from ` + strings.Join(gateFiles(s), " and ") + `.
  # git push runs it through the pre-push hook that ` + "`aayu install --env --apply`" + ` and .agents/setup install.
  # A git hook exports GIT_DIR and the like for its repository; tests that run git in a temporary directory would
  # then change this clone instead. The gate finds its repository from the working directory.
  unset $(git rev-parse --local-env-vars 2>/dev/null)
  GATE_TMP=$(mktemp -d)
  export GATE_TMP
  trap 'rm -rf "$GATE_TMP"' EXIT
  have() { command -v "$1" >/dev/null 2>&1 || { echo "aayu-gate: NOT RUN LOCALLY ($1 is not installed; CI runs it)"; return 1; }; }
  export -f have
  failed=()
  job() { # name commands...
    local name=$1 c
    shift
    for c in "$@"; do
      step "[ci: $name] $c"
      if ! bash -eo pipefail -c "$c"; then
        echo "aayu-gate: FAIL: CI job \"$name\" fails here: $c" >&2
        failed+=("$name")
        return 0
      fi
    done
  }
`)
	for _, g := range s.Gate {
		if len(g.Run) > 0 {
			args := make([]string, len(g.Run))
			for i, r := range g.Run {
				args[i] = shellQuote(r)
			}
			fmt.Fprintf(&b, "  job %s %s\n", shellQuote(g.Job), strings.Join(args, " "))
		}
		switch {
		case strings.TrimSpace(g.Remote) != "" && len(g.Run) > 0:
			fmt.Fprintf(&b, "  echo %s\n", shellQuote("aayu-gate: NOT RUN LOCALLY: part of CI job \""+g.Job+"\": "+g.Remote))
		case strings.TrimSpace(g.Remote) != "":
			fmt.Fprintf(&b, "  echo %s\n", shellQuote("aayu-gate: NOT RUN LOCALLY: CI job \""+g.Job+"\" runs only in CI: "+g.Remote))
		case len(g.Run) == 0:
			fmt.Fprintf(&b, "  echo %s >&2\n  failed+=(%s)\n", shellQuote("aayu-gate: FAIL: CI job \""+g.Job+"\" ("+g.Workflow+") has no local step: add its commands, or why it runs only in CI, under [[gate]] in "+SpecPath), shellQuote(g.Job))
		}
	}
	b.WriteString(`  if [ ${#failed[@]} -gt 0 ]; then
    echo "aayu-gate: failed CI jobs: ${failed[*]}" >&2
    exit 1
  fi
  echo "aayu-gate ok: every required CI job that can run here passed"
  exit 0
fi
`)
	return b.String()
}

func gateFiles(s Spec) []string {
	var out []string
	for _, g := range s.Gate {
		if !contains(out, g.Workflow) {
			out = append(out, g.Workflow)
		}
	}
	return out
}

// PrePushHook is the git pre-push hook that runs the local gate. It is not a
// committed file: git keeps hooks per clone, so `aayu install --env --apply`
// and .agents/setup install it (InstallPrePush). It replaces git-lfs's stock
// pre-push hook, so it does that hook's work after the gate. The gate reads
// nothing from stdin, which carries the refs git-lfs needs. A global
// core.hooksPath shares the hook with every clone, so it runs the gate only
// where .agents/check has one.
const PrePushHook = `#!/bin/sh
` + GeneratedMarker + `: the local gate before every push (Story #399).
# It runs the checks of the required CI jobs here (.agents/check --full). Skip it once with git push --no-verify.
top=$(git rev-parse --show-toplevel) || exit 1
if grep -qs -- '"${1:-}" = --full' "$top/.agents/check"; then bash "$top/.agents/check" --full </dev/null || exit 1; fi
if command -v git-lfs >/dev/null 2>&1; then exec git lfs pre-push "$@"; fi
if grep -qs 'filter=lfs' "$top/.gitattributes"; then echo >&2 "This repository uses Git LFS, but git-lfs is not on your PATH."; exit 2; fi
`

// otherHookLine matches a line of a pre-push hook that is neither aayu's nor
// part of git-lfs's stock hook: a hook with one is hand-written and kept.
var otherHookLine = regexp.MustCompile(`(?m)^[^#\n](?:.*)$`)

func replaceableHook(old string) bool {
	if strings.Contains(old, GeneratedMarker) {
		return true
	}
	for _, l := range otherHookLine.FindAllString(old, -1) {
		if !strings.HasPrefix(l, "command -v git-lfs ") && !strings.HasPrefix(l, "git lfs pre-push ") {
			return false
		}
	}
	return true
}

// InstallPrePush writes PrePushHook into the clone's hooks directory, unless a
// hand-written pre-push hook is there; it says which it did.
func InstallPrePush(gitHooksDir string) (string, error) {
	p := filepath.Join(gitHooksDir, "pre-push")
	if old, err := os.ReadFile(p); err == nil && !replaceableHook(string(old)) {
		return "", fmt.Errorf("%s is hand-written: add `bash .agents/check --full </dev/null` to it to run the local gate before a push", p)
	}
	if err := os.MkdirAll(gitHooksDir, 0o755); err != nil {
		return "", err
	}
	return p, os.WriteFile(p, []byte(PrePushHook), 0o755)
}

// setupHook is the setup script's step that installs the pre-push hook in a
// cloud host's clone, the way InstallPrePush does.
func setupHook(s Spec) string {
	if len(s.Gate) == 0 {
		return ""
	}
	return `hooks=$(git rev-parse --git-path hooks 2>/dev/null || true)
if [ -n "$hooks" ] && { [ ! -e "$hooks/pre-push" ] || grep -qF ` + shellQuote(GeneratedMarker) + ` "$hooks/pre-push" || ! grep -vqE '^(#|$|command -v git-lfs |git lfs pre-push )' "$hooks/pre-push"; }; then
  step "git pre-push hook: the local gate (.agents/check --full)"
  mkdir -p "$hooks"
  printf '%s' ` + shellQuote(PrePushHook) + ` > "$hooks/pre-push"
  chmod +x "$hooks/pre-push"
fi

`
}
