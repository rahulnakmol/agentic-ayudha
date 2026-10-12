package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/secrets"
	"github.com/rahulnakmol/agentic-ayudha/internal/upgrade"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// Run executes the CLI with args (without the program name) and returns the
// process exit code. It is the only entry point cmd/aayudha calls.
func Run(ctx context.Context, args []string, io IO, opts Options) int {
	e, err := newEnv(io, opts)
	if err != nil {
		fmt.Fprintf(io.Err, "aayu: %v\n", err)
		return ExitInternal
	}
	root := newRoot(e)
	root.SetArgs(args)
	err = root.ExecuteContext(ctx)
	code := CodeOf(err)
	var ce *Error
	if err != nil && !errors.As(err, &ce) {
		code = ExitUsage
	}
	e.endTrace(code, err)
	if err == nil {
		return ExitOK
	}
	if !errors.As(err, &ce) {
		// Errors that reach here without a code come from cobra itself: unknown
		// commands, unknown flags, or malformed flag values.
		ce = &Error{Code: ExitUsage, Err: err}
	}
	if e.JSON {
		enc := json.NewEncoder(e.Out)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(map[string]any{"error": map[string]any{"code": ce.Code, "message": ce.Error()}})
	}
	fmt.Fprintf(e.Err, "aayu: %v\n", ce)
	return ce.Code
}

// globalFlags are the names of the global flags; a registry input with the same
// name reads the global flag instead of defining its own.
var globalFlags = verbs.GlobalFlags

func init() { cobra.EnableCommandSorting = false } // list commands in registry (plan) order

func newRoot(e *Env) *cobra.Command {
	root := &cobra.Command{
		Use:   "aayu",
		Short: "Agentic software delivery pod: verbs from a grilled idea to a measured release, with proof.",
		Long: `aayu (the aayudha binary) runs an agentic software delivery pod: seats, layers, and verbs that take
work from a grilled idea to a measured release, with proof packs and human gates.

Exit codes:
` + exitCodeHelp(),
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.SetIn(e.In)
	root.SetOut(e.Out)
	root.SetErr(e.Err)
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return &Error{Code: ExitUsage, Err: err} })

	hosts := make([]string, len(verbs.Hosts))
	for i, h := range verbs.Hosts {
		hosts[i] = string(h.ID)
	}
	pf := root.PersistentFlags()
	pf.BoolVar(&e.DryRun, "dry-run", false, "plan and log actions without performing any external write")
	pf.BoolVar(&e.JSON, "json", false, "write machine-readable JSON to stdout")
	pf.StringVar(&e.Repo, "repo", "", "GitHub repository as owner/name (default: the current checkout's origin)")
	pf.StringVar(&e.Host, "host", "", "host running aayudha: "+strings.Join(hosts, ", ")+" (default: detected)")
	pf.BoolVar(&e.Yes, "yes", false, "answer yes to confirmations (human only; refused when AAYU_AGENT=1)")
	pf.BoolVarP(&e.Verbose, "verbose", "v", false, "write diagnostics to stderr")
	pf.BoolVar(&e.CI, "ci", false, "running in CI: accept GITHUB_TOKEN as the human token; bot writes need AAYU_BOT_TOKEN")
	pf.StringArrayVar(&e.Set, "set", nil, "override a config value for this run, as key=value (repeatable)")

	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		e.startTrace(cmd)
		if e.Yes && e.Agent() {
			return Errorf(ExitBlocked, "--yes is for humans only and is refused when AAYU_AGENT=1")
		}
		if e.Repo != "" && !repoPattern.MatchString(e.Repo) {
			return Errorf(ExitUsage, "--repo %q: want owner/name", e.Repo)
		}
		c, ok := verbs.Find(cmd.Name())
		if ok && e.Agent() && cmd.GroupID == "verbs" {
			if _, err := os.Stat(e.PauseFile()); err == nil {
				return Errorf(ExitBudget, "agent lanes are paused by the [budget] breaker (%s); a human must clear it", e.PauseFile())
			}
		}
		if ok && cmd.GroupID == "verbs" && !c.ReadOnly {
			if root := config.RepoRoot(e.Dir); root != "" {
				if err := upgrade.Check(root, upgrade.Current); err != nil {
					return Errorf(ExitUsage, "%s: refused: %v (read-only commands still work)", c.Name, err)
				}
			}
		}
		declaresHost := ok && slices.ContainsFunc(c.Inputs, func(in verbs.Input) bool { return in.Name == "host" })
		if e.Host != "" && !declaresHost && !slices.Contains(hosts, e.Host) {
			return Errorf(ExitUsage, "--host %q: want one of %s", e.Host, strings.Join(hosts, ", "))
		}
		return nil
	}

	root.AddGroup(&cobra.Group{ID: "verbs", Title: "Verbs:"}, &cobra.Group{ID: "plumbing", Title: "Plumbing commands:"})
	for _, c := range verbs.Verbs {
		root.AddCommand(newCommand(e, c, "verbs"))
	}
	for _, c := range verbs.Plumbing {
		root.AddCommand(newCommand(e, c, "plumbing"))
	}
	return root
}

var repoPattern = regexp.MustCompile(`^[A-Za-z0-9-]+/[A-Za-z0-9._-]+$`)

func exitCodeHelp() string {
	var b strings.Builder
	for _, x := range ExitCodes {
		fmt.Fprintf(&b, "  %d  %s\n", x.Code, x.Meaning)
	}
	return b.String()
}

// newCommand builds the cobra command for a registry command (ground rule 4):
// positional arguments, flags, and their validation all come from c.Inputs.
func newCommand(e *Env, c verbs.Command, group string) *cobra.Command {
	var positional []verbs.Input
	use := c.Name
	for _, in := range c.Inputs {
		if !in.Positional {
			continue
		}
		positional = append(positional, in)
		name := in.Name
		if in.Type == verbs.TypeEnum {
			name = strings.Join(in.Enum, "|")
		}
		if in.Required {
			use += " <" + name + ">"
		} else {
			use += " [" + name + "]"
		}
	}
	minArgs := 0
	for i, in := range positional {
		if in.Required {
			if i != minArgs {
				panic(fmt.Sprintf("registry: %s: required positional %q follows an optional one", c.Name, in.Name))
			}
			minArgs++
		}
	}

	short := c.Summary
	if _, ok := handlers[c.Name]; !ok {
		short += " (lands in " + c.WP + ")"
	}
	cmd := &cobra.Command{
		Use:     use,
		Short:   short,
		GroupID: group,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) < minArgs || len(args) > len(positional) {
				return Errorf(ExitUsage, "%s: want %s", c.Name, argCount(minArgs, len(positional)))
			}
			return nil
		},
	}

	for _, in := range c.Inputs {
		if in.Positional || slices.Contains(globalFlags, in.Name) {
			continue
		}
		desc := in.Description
		if in.Type == verbs.TypeEnum {
			desc += " One of: " + strings.Join(in.Enum, ", ") + "."
		}
		switch in.Type {
		case verbs.TypeBool:
			cmd.Flags().Bool(in.Name, false, desc)
		case verbs.TypeInt:
			cmd.Flags().Int(in.Name, 0, desc)
		default:
			cmd.Flags().String(in.Name, "", desc)
		}
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		vals := Values{}
		for i, a := range args {
			v, err := parseInput(c.Name, positional[i], a)
			if err != nil {
				return err
			}
			vals[positional[i].Name] = v
		}
		for _, in := range c.Inputs {
			if in.Positional {
				continue
			}
			f := cmd.Flags().Lookup(in.Name)
			if f == nil || !f.Changed {
				if in.Required {
					return Errorf(ExitUsage, "%s: --%s is required", c.Name, in.Name)
				}
				continue
			}
			v, err := parseInput(c.Name, in, f.Value.String())
			if err != nil {
				return err
			}
			vals[in.Name] = v
		}
		h, ok := handlers[c.Name]
		if !ok {
			return Errorf(ExitUsage, "%s: not implemented yet (lands in %s)", c.Name, c.WP)
		}
		return classify(h(cmd.Context(), e, vals))
	}
	return cmd
}

// classify attaches exit codes to the sentinel errors of lower layers.
func classify(err error) error {
	switch {
	case errors.Is(err, models.ErrBudget):
		return Wrap(ExitBudget, err)
	case errors.Is(err, actions.ErrCapability), errors.Is(err, secrets.ErrWriteScoped):
		return Wrap(ExitBlocked, err)
	case errors.Is(err, integrations.ErrNotConfigured), errors.Is(err, integrations.ErrProvider), errors.Is(err, integrations.ErrNoAdapter):
		return Wrap(ExitExternal, err)
	case errors.Is(err, integrations.ErrQuery):
		return Wrap(ExitUsage, err)
	case github.External(err), errors.Is(err, ledger.ErrContended), errors.Is(err, ledger.ErrLocked):
		return Wrap(ExitExternal, err)
	case errors.Is(err, ledger.ErrNoLedger):
		return Wrap(ExitBlocked, err)
	}
	return Wrap(ExitInternal, err)
}

func argCount(lo, hi int) string {
	switch {
	case lo == hi && lo == 0:
		return "no arguments"
	case lo == hi:
		return fmt.Sprintf("%d argument(s)", lo)
	default:
		return fmt.Sprintf("%d to %d arguments", lo, hi)
	}
}

var (
	issueRE = regexp.MustCompile(verbs.IssuePattern)
	prRE    = regexp.MustCompile(verbs.PRPattern)
)

// parseInput validates one raw input value against its registry type.
func parseInput(cmd string, in verbs.Input, raw string) (any, error) {
	bad := func(want string) error {
		return Errorf(ExitUsage, "%s: %s %q: want %s", cmd, in.Name, raw, want)
	}
	switch in.Type {
	case verbs.TypeBool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, bad("true or false")
		}
		return b, nil
	case verbs.TypeInt:
		n, err := strconv.Atoi(raw)
		if err != nil {
			return nil, bad("an integer")
		}
		return n, nil
	case verbs.TypeEnum:
		if !slices.Contains(in.Enum, raw) {
			return nil, bad("one of " + strings.Join(in.Enum, ", "))
		}
	case verbs.TypeIssue:
		if !issueRE.MatchString(raw) {
			return nil, bad("an issue number, #number, or issue URL")
		}
	case verbs.TypePR:
		if !prRE.MatchString(raw) {
			return nil, bad("a pull request number, #number, or PR URL")
		}
	case verbs.TypeString, verbs.TypePath:
		if raw == "" {
			return nil, bad("a non-empty value")
		}
	}
	return raw, nil
}
