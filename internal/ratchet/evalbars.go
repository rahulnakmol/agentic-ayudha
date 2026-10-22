package ratchet

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/evals"
)

// EvalBars checks that the working tree of the repository at repo lowers no
// skill-eval bar against the commit base (evals.CheckBars, Story #400): base's
// evals/ is read from git into a temporary directory. accepted verifies a new
// bar_waiver Decision.
func EvalBars(repo, base string, accepted func(decision int) error) ([]evals.Lowering, []string, error) {
	dir, err := os.MkdirTemp("", "eval-bars-")
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	if err := extract(repo, base, "evals", dir); err != nil {
		return nil, nil, err
	}
	lowered, problems, err := evals.CheckBars(dir, repo, accepted)
	if err != nil {
		return nil, nil, fmt.Errorf("eval-bar ratchet: %w", err)
	}
	return lowered, problems, nil
}

// EvalBarReport runs EvalBars, writes each lowering (or that there is none)
// to w, and returns the problems, each prefixed for CI's log.
func EvalBarReport(w io.Writer, repo, base string, accepted func(decision int) error) ([]string, error) {
	lowered, problems, err := EvalBars(repo, base, accepted)
	if err != nil {
		return nil, err
	}
	for _, l := range lowered {
		_, _ = fmt.Fprintf(w, "eval bar lowered: %s\n", l)
	}
	if len(lowered) == 0 {
		_, _ = fmt.Fprintln(w, "eval bars: none lowered")
	}
	for i, p := range problems {
		problems[i] = "eval-bar ratchet: " + p
	}
	return problems, nil
}

// extract writes the tree at sha:dir of the repository at repo under root.
func extract(repo, sha, dir, root string) error {
	cmd := exec.Command("git", "archive", "--format=tar", sha, dir)
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			err = fmt.Errorf("%w: %s", err, strings.TrimSpace(string(ee.Stderr)))
		}
		return fmt.Errorf("git archive %s %s: %w", sha, dir, err)
	}
	tr := tar.NewReader(bytes.NewReader(out))
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("git archive %s %s: %w", sha, dir, err)
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		name := filepath.Join(root, filepath.FromSlash(h.Name))
		b, err := io.ReadAll(tr)
		if err == nil {
			err = os.MkdirAll(filepath.Dir(name), 0o755)
		}
		if err == nil {
			err = os.WriteFile(name, b, 0o644)
		}
		if err != nil {
			return err
		}
	}
}
