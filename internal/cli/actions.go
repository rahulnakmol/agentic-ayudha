package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
)

func init() { register("actions", runActions) }

// stateDir is where local runtime state lives: <repo>/.aayudha in a checkout,
// else the XDG state directory.
func (e *Env) stateDir() string {
	if root := config.RepoRoot(e.Dir); root != "" {
		return filepath.Join(root, ".aayudha")
	}
	if d := e.Getenv("XDG_STATE_HOME"); d != "" {
		return filepath.Join(d, "aayudha")
	}
	return filepath.Join(e.Getenv("HOME"), ".local", "state", "aayudha")
}

// ActionsPath is the local actions log, telemetry/actions.jsonl (WP1.5).
func (e *Env) ActionsPath() string { return filepath.Join(e.stateDir(), "telemetry", "actions.jsonl") }

// Actions opens the actions log for this invocation. --dry-run makes every
// action "planned". Checkpoint segments go to the ledger branch (WP2.4) when
// run in a checkout whose remote has one, else to a local ledger outbox.
func (e *Env) Actions() (*actions.Log, error) {
	if e.actions != nil {
		return e.actions, nil
	}
	l, err := actions.Open(e.ActionsPath(), actions.Options{
		DryRun: e.DryRun, Host: e.Host, Now: e.Now,
		Sink: e.segmentSink(),
	})
	if err != nil {
		return nil, Wrap(ExitInternal, err)
	}
	e.actions = l
	return l, nil
}

func runActions(_ context.Context, e *Env, in Values) error {
	path := e.ActionsPath()
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return e.Emit([]any{}, func(w io.Writer) error {
			_, err := fmt.Fprintf(w, "no actions logged yet (%s)\n", path)
			return err
		})
	}
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	switch in.String("action") {
	case "verify-chain":
		n, err := actions.VerifyChain(f)
		if err != nil {
			return Errorf(ExitBlocked, "%s: chain broken after %d valid records: %v", path, n, err)
		}
		return e.Emit(map[string]any{"path": path, "records": n, "valid": true}, func(w io.Writer) error {
			_, err := fmt.Fprintf(w, "%s: %d records, chain intact\n", path, n)
			return err
		})
	case "grep":
		pattern := in.String("pattern")
		if pattern == "" {
			return Errorf(ExitUsage, "actions grep: give a pattern")
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return Wrap(ExitUsage, fmt.Errorf("actions grep: %w", err))
		}
		return printLines(e, f, func(line string) bool { return re.MatchString(line) }, 0)
	default: // tail
		if in.String("pattern") != "" {
			return Errorf(ExitUsage, "actions tail takes no pattern")
		}
		return printLines(e, f, func(string) bool { return true }, 20)
	}
}

// printLines writes matching log lines (the last `last` of them when last > 0)
// as raw JSONL; with --json they are the same lines.
func printLines(e *Env, r io.Reader, keep func(string) bool, last int) error {
	var lines []string
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	for sc.Scan() {
		if l := sc.Text(); keep(l) {
			lines = append(lines, l)
			if last > 0 && len(lines) > last {
				lines = lines[1:]
			}
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	for _, l := range lines {
		fmt.Fprintln(e.Out, l)
	}
	return nil
}

// segmentSink is the ledger when there is a checkout, falling back to the
// local outbox when its remote has no ledger branch yet.
func (e *Env) segmentSink() actions.SegmentSink {
	outbox := actions.DirSink{Root: filepath.Join(e.stateDir(), "ledger-outbox")}
	root := config.RepoRoot(e.Dir)
	if root == "" {
		return outbox
	}
	return fallbackSink{ledger.Sink{W: &ledger.Writer{Dir: root}}, outbox}
}

type fallbackSink struct {
	primary  actions.SegmentSink
	fallback actions.SegmentSink
}

func (f fallbackSink) Append(date, host, run string, lines [][]byte) error {
	err := f.primary.Append(date, host, run, lines)
	if errors.Is(err, ledger.ErrNoLedger) {
		return f.fallback.Append(date, host, run, lines)
	}
	return err
}
