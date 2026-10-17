// Command ratchet checks or tightens this repository's coverage floor
// (internal/ratchet; Appendix B S-4.3r) and, given a base, checks that the
// change lowers no skill-eval bar (evals.CheckBars, Story #400). CI's build
// job runs
//
//	go run ./internal/ratchet/cmd/ratchet -coverprofile coverage.out [-base <sha>]
//
// and a contributor whose change raised coverage runs it with -update.
package main

import (
	"archive/tar"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/evals"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/ratchet"
)

func main() {
	profile := flag.String("coverprofile", "coverage.out", "Go coverprofile of `go test ./...`")
	base := flag.String("base", "", "base commit of the PR: its floor may only be loosened by a new accepted waiver (fetched from origin when missing)")
	update := flag.Bool("update", false, "tighten the floor in "+ratchet.File+" to what the coverage supports; never loosens it")
	flag.Parse()
	if err := run(*profile, *base, *update); err != nil {
		fmt.Fprintf(os.Stderr, "::error::%v\n", err)
		os.Exit(1)
	}
}

func run(profile, base string, update bool) error {
	b, err := os.ReadFile(ratchet.File)
	if err != nil {
		return err
	}
	cur, err := ratchet.Parse(b)
	if err != nil {
		return err
	}
	f, err := os.Open(profile)
	if err != nil {
		return err
	}
	measured, err := ratchet.Total(f)
	_ = f.Close()
	if err != nil {
		return err
	}
	if update {
		floor := ratchet.Tighten(cur.Coverage, measured)
		if floor > cur.Coverage.Floor {
			if err := os.WriteFile(ratchet.File, ratchet.SetFloor(b, floor), 0o644); err != nil {
				return err
			}
		}
		fmt.Printf("coverage %.1f %%; floor %.1f %% -> %.1f %%\n", measured, cur.Coverage.Floor, floor)
		return nil
	}
	var prev *ratchet.Ratchet
	if base != "" {
		if prev, err = baseRatchet(base); err != nil {
			return err
		}
	}
	problems, waiver := ratchet.Check(cur.Coverage, prev, measured)
	if waiver != 0 {
		if err := checkWaiver(waiver, "coverage"); err != nil {
			problems = append(problems, fmt.Sprintf("waiver #%d: %v", waiver, err))
		}
	}
	fmt.Printf("coverage %.1f %%; floor %.1f %% (margin %.1f)\n", measured, cur.Coverage.Floor, cur.Coverage.Margin)
	var barProblems []string
	if base != "" {
		if barProblems, err = evalBars(base); err != nil {
			return err
		}
	}
	var all []string
	for _, p := range problems {
		all = append(all, "coverage ratchet: "+p)
	}
	for _, p := range barProblems {
		all = append(all, "eval-bar ratchet: "+p)
	}
	if len(all) > 0 {
		return errors.New(strings.Join(all, "\n::error::"))
	}
	return nil
}

// evalBars checks that the change from base lowers no skill-eval bar
// (evals.CheckBars): base's evals/ is read from git, the change's from the
// working tree.
func evalBars(base string) ([]string, error) {
	dir, err := os.MkdirTemp("", "eval-bars-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	if err := extract(base, "evals", dir); err != nil {
		return nil, err
	}
	lowered, problems, err := evals.CheckBars(dir, ".", func(n int) error { return checkWaiver(n, evals.BarFloor) })
	if err != nil {
		return nil, fmt.Errorf("eval-bar ratchet: %w", err)
	}
	for _, l := range lowered {
		fmt.Printf("eval bar lowered: %s\n", l)
	}
	if len(lowered) == 0 {
		fmt.Println("eval bars: none lowered")
	}
	return problems, nil
}

// extract writes the tree at sha:dir under root, as `git archive` gives it.
func extract(sha, dir, root string) error {
	cmd := exec.Command("git", "archive", "--format=tar", sha, dir)
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	tr := tar.NewReader(out)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			_ = cmd.Wait()
			return fmt.Errorf("git archive %s %s: %w", sha, dir, err)
		}
		name := filepath.Join(root, filepath.FromSlash(h.Name))
		if !strings.HasPrefix(name, filepath.Clean(root)+string(os.PathSeparator)) {
			continue
		}
		switch h.Typeflag {
		case tar.TypeDir:
			err = os.MkdirAll(name, 0o755)
		case tar.TypeReg:
			if err = os.MkdirAll(filepath.Dir(name), 0o755); err == nil {
				err = writeFrom(name, tr)
			}
		}
		if err != nil {
			_ = cmd.Wait()
			return err
		}
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("git archive %s %s: %w: %s", sha, dir, err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func writeFrom(name string, r io.Reader) error {
	f, err := os.Create(name)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, r)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// baseRatchet reads the base commit's ratchet; the floor before the ratchet
// existed is 0, so any floor is a tightening.
func baseRatchet(sha string) (*ratchet.Ratchet, error) {
	show := func() ([]byte, error) { return exec.Command("git", "show", sha+":"+ratchet.File).Output() }
	b, err := show()
	if err != nil {
		if out, ferr := exec.Command("git", "fetch", "--no-tags", "--quiet", "--depth=1", "origin", sha).CombinedOutput(); ferr != nil {
			return nil, fmt.Errorf("fetch base %s: %w: %s", sha, ferr, out)
		}
		if exec.Command("git", "cat-file", "-e", sha+":"+ratchet.File).Run() != nil {
			return &ratchet.Ratchet{}, nil // the base predates the ratchet
		}
		if b, err = show(); err != nil {
			return nil, err
		}
	}
	r, err := ratchet.Parse(b)
	if err != nil {
		return nil, fmt.Errorf("base %s: %w", sha, err)
	}
	return &r.Coverage, nil
}

// checkWaiver reads a floor-waiver Decision for floor from GitHub with the job's token.
func checkWaiver(n int, floor string) error {
	repo := os.Getenv("GITHUB_REPOSITORY")
	owner, name, err := github.SplitRepo(repo)
	if err != nil {
		return fmt.Errorf("GITHUB_REPOSITORY %q: %w", repo, err)
	}
	c, err := github.New(github.Options{Token: os.Getenv("GH_TOKEN")})
	if err != nil {
		return err
	}
	is, _, err := c.REST.Issues.Get(context.Background(), owner, name, n)
	if err != nil {
		return err
	}
	return ratchet.AcceptedWaiver(is, floor, nil)
}
