package agentenv

import (
	"fmt"
	"strings"
)

// Open returns the questions the spec's recorded answers do not settle yet.
func Open(qs []Question, answers map[string]Answer) []Question {
	var out []Question
	for _, q := range qs {
		if _, ok := answers[q.ID]; !ok {
			out = append(out, q)
		}
	}
	return out
}

// Settle builds the spec from a discovery and the answers (recorded ones and
// new ones). A question nobody answered takes its default, recorded as such.
func Settle(disc Discovery, answers map[string]Answer) (Spec, error) {
	s := disc.Spec
	s.Answers = map[string]Answer{}
	ids := []string{}
	for _, q := range disc.Questions {
		ids = append(ids, q.ID)
		if a, ok := answers[q.ID]; ok {
			s.Answers[q.ID] = a
		} else {
			s.Answers[q.ID] = Answer{Value: q.Default, Source: "default"}
		}
	}
	// An answer stays settled after its question is no longer asked (the
	// go-version question disappears once the hand-written setup is replaced).
	for _, id := range []string{"go-version", "secrets", "checks", "steps", "browsers", "fix-packages", "fix-steps"} {
		if a, ok := answers[id]; ok && s.Answers[id] == (Answer{}) {
			s.Answers[id], ids = a, append(ids, id)
		}
	}
	for _, id := range ids {
		a := s.Answers[id]
		v := strings.TrimSpace(a.Value)
		switch id {
		case "hosts":
			var hs []string
			for _, h := range strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ' ' }) {
				if !contains(Hosts, h) {
					return Spec{}, fmt.Errorf("hosts: %q is not one of %s", h, strings.Join(Hosts, ", "))
				}
				hs = append(hs, h)
			}
			if len(hs) == 0 {
				return Spec{}, fmt.Errorf("hosts: name at least one of %s", strings.Join(Hosts, ", "))
			}
			s.Hosts = hs
		case "go-version":
			if g := s.Tool("go"); g != nil && v != "" {
				g.Version = strings.TrimPrefix(v, "go")
				g.Evidence += "; version " + g.Version + " (answer go-version, " + a.Source + ")"
			}
		case "browsers":
			if !yes(v) {
				s.Browsers = nil
			}
		case "secrets":
			if v != "" && !strings.EqualFold(v, "none") {
				need := map[string]bool{}
				for _, n := range strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ' ' }) {
					need[n] = true
				}
				for n := range need {
					found := false
					for _, e := range s.Env {
						found = found || e.Name == n
					}
					if !found {
						return Spec{}, fmt.Errorf("secrets: %s is not in %s", n, strings.Split(s.EnvFile, " ")[0])
					}
				}
				s.Notes = append(s.Notes, "the checks need real values for "+v+"; each host's instructions say where to add them")
			}
		case "checks", "steps":
			if v != "" && v != "none" && v != "skip" {
				st := Step{Run: v, Evidence: "answer " + id + " (" + a.Source + ")"}
				if id == "checks" {
					s.Checks = append(s.Checks, st)
				} else {
					s.Steps = append(s.Steps, st)
				}
			}
		case "fix-packages":
			for _, p := range strings.Fields(v) {
				if !contains(s.Packages, p) {
					s.Packages = append(s.Packages, p)
				}
			}
		case "fix-steps":
			if contains(strings.Fields(v), "safe-directory") {
				s.Steps = append(s.Steps, SafeDirectoryStep)
			}
		case "existing-setup":
			if v == "keep" {
				return Spec{}, fmt.Errorf("existing-setup: keep means aayu cannot generate %s; port what it does into [[steps]] in %s, then answer replace", SetupPath, SpecPath)
			}
		}
	}
	return s, nil
}

func yes(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	return v == "y" || v == "yes" || v == "true"
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// Readback is the spec in a few lines, for the human to confirm.
func Readback(s Spec) []string {
	var out []string
	var tools []string
	for _, t := range s.Tools {
		v := t.Version
		if v == "" || t.Name == "rust" || t.Name == "python" {
			v = t.Want
		}
		tools = append(tools, fmt.Sprintf("%s %s (%s)", t.Name, v, t.Evidence))
	}
	if len(tools) > 0 {
		out = append(out, "Toolchains: "+strings.Join(tools, "; "))
	}
	if len(s.Packages) > 0 {
		out = append(out, "System packages: "+strings.Join(s.Packages, ", "))
	}
	for _, b := range s.Browsers {
		out = append(out, "Browser: "+b.Run+" ("+b.Evidence+")")
	}
	for _, sv := range s.Services {
		out = append(out, fmt.Sprintf("Service: %s, started per session by %s (%s)", sv.Name, StartPath, sv.Evidence))
	}
	var deps []string
	for _, d := range s.Deps {
		deps = append(deps, d.Run)
	}
	if len(deps) > 0 {
		out = append(out, "Dependencies: "+strings.Join(deps, "; "))
	}
	for _, st := range s.Steps {
		out = append(out, "Step: "+st.Run+" ("+st.Evidence+")")
	}
	var env []string
	for _, e := range s.Env {
		n := e.Name
		if e.Secret {
			n += " (secret)"
		}
		env = append(env, n)
	}
	if len(env) > 0 {
		out = append(out, "Variables by name ("+s.EnvFile+"): "+strings.Join(env, ", "))
	}
	var checks []string
	for _, c := range s.Checks {
		checks = append(checks, c.Run)
	}
	out = append(out, "Proof that an agent can work here: "+orStr(strings.Join(checks, " && "), "none"))
	if len(s.Gate) > 0 {
		var jobs []string
		for _, g := range s.Gate {
			jobs = append(jobs, g.Job)
		}
		out = append(out, "Local gate before a push (.agents/check --full, pre-push hook): CI jobs "+strings.Join(jobs, ", "))
		if gaps := GateGaps(s); len(gaps) > 0 {
			out = append(out, "Local gate gaps (it fails on them until "+SpecPath+" [[gate]] gives commands or a reason): "+strings.Join(gaps, ", "))
		}
	}
	out = append(out, "Hosts: "+strings.Join(s.Hosts, ", "))
	return out
}
