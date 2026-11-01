package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/agentenv"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/upgrade"
)

type envQuestion struct {
	agentenv.Question
	Answer agentenv.Answer `json:"answer"`
}

type envFile struct {
	Path   string `json:"path"`
	Change string `json:"change"` // create, update, same
}

type envReport struct {
	Readback  []string               `json:"readback"`
	Questions []envQuestion          `json:"questions,omitempty"`
	Files     []envFile              `json:"files"`
	Notes     []string               `json:"notes,omitempty"`
	Manual    map[string][]string    `json:"manual,omitempty"`
	Verified  []agentenv.Result      `json:"verified,omitempty"`
	Cloud     []agentenv.CloudResult `json:"cloud,omitempty"`
	Status    []agentenv.HostStatus  `json:"status"`
	Applied   bool                   `json:"applied"`
	Spec      agentenv.Spec          `json:"spec"`
}

var ampProject = regexp.MustCompile(`(?m)^Project:\s*(\S+)`)

// runInstallEnv is `aayu install --env`: discover the environment from the
// code, ask only what the code cannot say, show every host's generated files
// as a diff (and the UI values for hosts configured in a UI), and, with
// --apply, write them; --verify proves each host's environment in a clean
// container and --cloud in the host's live cloud (Appendix B S-3.2c).
func runInstallEnv(ctx context.Context, e *Env, in Values) error {
	root := config.RepoRoot(e.Dir)
	if root == "" {
		return Errorf(ExitUsage, "run inside a git repository")
	}
	hosts := agentenv.Hosts
	if e.Host != "" && e.Host != "all" {
		if !slices.Contains(agentenv.Hosts, e.Host) {
			return Errorf(ExitUsage, "install --env: aayu sets up no agent environment for %s yet; name one of %s with --host", e.Host, strings.Join(agentenv.Hosts, ", "))
		}
		hosts = []string{e.Host}
	}
	if r := in.String("record"); r != "" {
		return e.recordEnvReply(root, r)
	}
	apply := in.Bool("apply") || in.Bool("verify")
	disc, err := agentenv.Discover(root)
	if err != nil {
		return err
	}
	old, _, err := agentenv.LoadSpec(root)
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	answers := map[string]agentenv.Answer{}
	for k, v := range old.Answers {
		answers[k] = v
	}
	open := agentenv.Open(disc.Questions, answers)
	if len(open) > 0 && !e.JSON && e.isTerminal(e.In) && !e.Agent() {
		e.interviewEnv(open, answers)
	}
	s, err := agentenv.Settle(disc, answers)
	if err != nil {
		return Wrap(ExitUsage, fmt.Errorf("install --env: %w", err))
	}
	agentenv.KeepPins(&s, old)
	if err := agentenv.Resolve(ctx, &s, e.envFetch()); err != nil {
		return WithNext(Wrap(ExitExternal, fmt.Errorf("install --env: %w (pinning needs the vendors' release indexes)", err)), "aayu install --env")
	}
	rep := envReport{Readback: agentenv.Readback(s), Manual: map[string][]string{}, Spec: s, Applied: apply}
	for _, q := range disc.Questions {
		rep.Questions = append(rep.Questions, envQuestion{q, s.Answers[q.ID]})
	}
	files, notes, err := agentenv.Generate(root, s)
	if err != nil {
		return Wrap(ExitBlocked, err)
	}
	rep.Notes = append(append(rep.Notes, s.Notes...), notes...)
	var diff strings.Builder
	for _, f := range files {
		oldB, rerr := os.ReadFile(filepath.Join(root, filepath.FromSlash(f.Path)))
		ch := "same"
		switch {
		case rerr != nil:
			ch = "create"
			fmt.Fprintf(&diff, "create %s (%d lines)\n", f.Path, strings.Count(string(f.Content), "\n"))
		case string(oldB) != string(f.Content):
			ch = "update"
			fmt.Fprintf(&diff, "--- a/%s\n+++ b/%s\n%s", f.Path, f.Path, upgrade.Unified(string(oldB), string(f.Content)))
		}
		rep.Files = append(rep.Files, envFile{f.Path, ch})
	}
	repo := e.Repo
	if repo == "" {
		if out, err := e.exec()(ctx, root, "git", "remote", "get-url", "origin"); err == nil {
			if m := originRepo.FindStringSubmatch(strings.TrimSpace(out)); m != nil {
				repo = m[1]
			}
		}
	}
	for _, h := range hosts {
		if s.HasHost(h) {
			rep.Manual[h] = agentenv.Manual(h, s, repo)
		}
	}
	if apply && !e.DryRun {
		if err := agentenv.Write(root, s); err != nil {
			return err
		}
		if len(s.Gate) > 0 { // git keeps hooks per clone, so the pre-push hook is installed, not committed (Story #399)
			hooks, err := e.exec()(ctx, root, "git", "rev-parse", "--path-format=absolute", "--git-path", "hooks")
			if err == nil {
				var p string
				if p, err = agentenv.InstallPrePush(strings.TrimSpace(hooks)); err == nil {
					rep.Notes = append(rep.Notes, "installed the pre-push hook "+p+": git push runs the local gate (.agents/check --full) first")
				}
			}
			if err != nil {
				rep.Notes = append(rep.Notes, "pre-push hook not installed: "+err.Error())
			}
		}
	}
	var failed []string
	if in.Bool("verify") && !e.DryRun {
		for _, h := range hosts {
			if !s.HasHost(h) {
				continue
			}
			e.talkf("install --env: verifying %s in a clean %s container (setup cold, setup warm, then the check)…", h, agentenv.Profiles[h].Image)
			fixed, res, err := agentenv.Verify(ctx, s, agentenv.VerifyOptions{Root: root, Host: h, Run: e.envRunner(), LogDir: filepath.Join(e.stateDir(), "state", "env"), Now: e.Now})
			if err != nil {
				return WithNext(Wrap(ExitExternal, fmt.Errorf("install --env --verify: %w", err)), "aayu doctor")
			}
			if len(res.Fixes) > 0 {
				s = fixed
				if err := agentenv.Write(root, s); err != nil {
					return err
				}
			}
			rep.Verified = append(rep.Verified, res)
			if res.OK {
				if err := agentenv.RecordContainer(root, h, s, res); err != nil {
					return err
				}
			} else {
				failed = append(failed, h)
			}
		}
		rep.Spec, rep.Readback = s, agentenv.Readback(s)
	}
	if in.Bool("cloud") && !e.DryRun {
		for _, h := range hosts {
			if !s.HasHost(h) {
				continue
			}
			r := e.probeCloud(ctx, root, h, repo, s)
			rep.Cloud = append(rep.Cloud, r)
			if !r.OK {
				failed = append(failed, h+" (cloud)")
			}
		}
	}
	if rep.Status, err = agentenv.Status(root); err != nil {
		return err
	}
	text := func(w io.Writer) error {
		fmt.Fprintln(w, "Here is the agent environment the code describes:")
		for _, l := range rep.Readback {
			fmt.Fprintln(w, "  "+l)
		}
		for _, q := range rep.Questions {
			fmt.Fprintf(w, "  answer %s: %s (%s)\n", q.ID, q.Answer.Value, q.Answer.Source)
		}
		for _, n := range rep.Notes {
			fmt.Fprintln(w, "note:", n)
		}
		if diff.Len() > 0 {
			fmt.Fprintf(w, "# Generated files\n%s", diff.String())
		} else {
			fmt.Fprintln(w, "# Generated files: all match the spec")
		}
		for _, h := range hosts {
			if m := rep.Manual[h]; len(m) > 0 {
				fmt.Fprintf(w, "# %s\n", h)
				for _, l := range m {
					fmt.Fprintln(w, "  "+l)
				}
			}
		}
		for _, v := range rep.Verified {
			word := "PASS"
			if !v.OK {
				word = "FAIL"
			}
			fmt.Fprintf(w, "%s %s in %s: setup cold %ds, warm %ds, check %ds; %d attempt(s); log %s\n", word, v.Host, v.Image, v.Phases["setup-cold"], v.Phases["setup-warm"], v.Phases["check"], v.Attempts, v.Log)
			for _, f := range v.Fixes {
				fmt.Fprintln(w, "  fixed: "+f)
			}
			if !v.OK {
				for _, l := range v.Tail {
					fmt.Fprintln(w, "  | "+l)
				}
			}
		}
		for _, c := range rep.Cloud {
			word := "PASS"
			if !c.OK {
				word = "FAIL"
			}
			fmt.Fprintf(w, "%s %s cloud: %s %s\n", word, c.Host, c.Detail, c.URL)
		}
		for _, st := range rep.Status {
			fmt.Fprintf(w, "%-8s %-8s %s\n", st.Host, st.State, st.Detail)
		}
		if !apply && !e.DryRun {
			fmt.Fprintln(w, "install --env: showing the plan only; `aayu install --env --apply` writes it and `--verify` proves it in clean containers")
		}
		return nil
	}
	if err := e.Emit(rep, text); err != nil {
		return err
	}
	if len(failed) > 0 {
		return WithNext(Errorf(ExitBlocked, "install --env: not verified: %s", strings.Join(failed, ", ")), "aayu install --env --verify")
	}
	return nil
}

// interviewEnv asks the open questions on the terminal: Enter takes the
// default, y confirms it, anything else is the human's own answer.
func (e *Env) interviewEnv(qs []agentenv.Question, answers map[string]agentenv.Answer) {
	r := bufio.NewReader(e.In)
	fmt.Fprintf(e.Err, "%d question(s) the code cannot answer:\n", len(qs))
	for _, q := range qs {
		fmt.Fprintf(e.Err, "\n? %s\n  default: %s, because %s\n> ", q.Ask, q.Default, q.Why)
		line, _ := r.ReadString('\n')
		v := strings.TrimSpace(line)
		switch {
		case v == "":
			answers[q.ID] = agentenv.Answer{Value: q.Default, Source: "default"}
		case strings.EqualFold(v, q.Default), (strings.EqualFold(v, "y") || strings.EqualFold(v, "yes")) && q.Default != "no":
			answers[q.ID] = agentenv.Answer{Value: q.Default, Source: "confirmed"}
		default:
			answers[q.ID] = agentenv.Answer{Value: v, Source: "stated"}
		}
	}
	fmt.Fprintln(e.Err)
}

// probeCloud proves one host's live cloud environment with the probe task,
// where the host's CLI can start one and read its reply.
func (e *Env) probeCloud(ctx context.Context, root, h, repo string, s agentenv.Spec) agentenv.CloudResult {
	want := s.Fingerprint()
	switch h {
	case agentenv.Amp:
		project := repo
		if out, err := e.exec()(ctx, root, "amp", "projects", "status"); err == nil {
			if m := ampProject.FindStringSubmatch(out); m != nil {
				project = m[1]
			}
		}
		if project == "" {
			return agentenv.CloudResult{Host: h, Detail: "no Amp project matches this repository (`amp projects create`)"}
		}
		if repo != "" && !strings.EqualFold(project, repo) { // a probe of another project proves nothing here, and Detect refuses its record
			return agentenv.CloudResult{Host: h, Detail: "amp projects status selects the Amp project " + project + ", not this repository (" + repo + ")"}
		}
		e.talkf("install --env --cloud: probing a new orb in Amp project %s (setup, then the check; several minutes)…", project)
		r := agentenv.ProbeAmp(ctx, e.exec(), project, want)
		if r.OK {
			_ = agentenv.RecordCloud(root, h, agentenv.Record{Fingerprint: want, Method: "cloud", At: e.Now().UTC().Format(time.RFC3339), Evidence: r.URL, Project: project})
		}
		return r
	case agentenv.Codex:
		l, err := e.Config()
		if err != nil || l.Config.Delegate.CodexEnv == "" {
			return agentenv.CloudResult{Host: h, Detail: "no [delegate] codex_env: create the Codex environment (see the codex section), then set it"}
		}
		e.talkf("install --env --cloud: probing Codex environment %s…", l.Config.Delegate.CodexEnv)
		r := agentenv.ProbeCodex(ctx, e.exec(), l.Config.Delegate.CodexEnv, e.devBranch(), want, 20*time.Second)
		if r.OK {
			_ = agentenv.RecordCloud(root, h, agentenv.Record{Fingerprint: want, Method: "cloud", At: e.Now().UTC().Format(time.RFC3339), Evidence: r.URL})
		}
		return r
	}
	return agentenv.CloudResult{Host: h, Detail: "aayu cannot read a " + h + " cloud agent's reply from here: run the probe in " + h + " (see its section) and record the reply with `aayu install --env --host " + h + " --record '<reply>'`"}
}

// recordEnvReply records a probe reply a human copied from a host's UI.
func (e *Env) recordEnvReply(root, reply string) error {
	if e.Host == "" || e.Host == "all" {
		return Errorf(ExitUsage, "install --env --record: name the host with --host")
	}
	s, ok, err := agentenv.LoadSpec(root)
	if err != nil || !ok {
		return Errorf(ExitUsage, "install --env --record: no %s; run `aayu install --env --apply` first", agentenv.SpecPath)
	}
	rec, err := agentenv.RecordReply(e.Host, reply, "", s.Fingerprint(), e.Now())
	if err != nil {
		return Errorf(ExitBlocked, "install --env --record: %v", err)
	}
	if err := agentenv.RecordCloud(root, e.Host, rec); err != nil {
		return err
	}
	return e.Emit(rec, func(w io.Writer) error {
		_, err := fmt.Fprintf(w, "install --env: recorded the live %s environment %s\n", e.Host, rec.Fingerprint)
		return err
	})
}

func (e *Env) envRunner() agentenv.Runner {
	if e.EnvRunner != nil {
		return e.EnvRunner
	}
	return agentenv.ExecRunner
}

func (e *Env) envFetch() agentenv.Fetch {
	if e.EnvFetch != nil {
		return e.EnvFetch
	}
	return agentenv.HTTPFetch(e.HTTP)
}

func (e *Env) talkf(format string, args ...any) {
	if !e.JSON {
		fmt.Fprintf(e.Err, format+"\n", args...)
	}
}
