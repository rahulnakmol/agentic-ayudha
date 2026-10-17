// Command ratchet checks or tightens this repository's coverage floor
// (internal/ratchet; Appendix B S-4.3r). CI's build job runs
//
//	go run ./internal/ratchet/cmd/ratchet -coverprofile coverage.out [-base <sha>]
//
// and a contributor whose change raised coverage runs it with -update.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/ratchet"
)

func main() {
	profile := flag.String("coverprofile", "coverage.out", "Go coverprofile of `go test ./...`")
	base := flag.String("base", "", "base commit of the PR: its floor may only be loosened by a new accepted waiver (fetched from origin when missing)")
	update := flag.Bool("update", false, "tighten the floor in "+ratchet.File+" to what the coverage supports; never loosens it")
	flag.Parse()
	if err := run(*profile, *base, *update); err != nil {
		fmt.Fprintf(os.Stderr, "::error::coverage ratchet: %v\n", err)
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
		if err := checkWaiver(waiver); err != nil {
			problems = append(problems, fmt.Sprintf("waiver #%d: %v", waiver, err))
		}
	}
	fmt.Printf("coverage %.1f %%; floor %.1f %% (margin %.1f)\n", measured, cur.Coverage.Floor, cur.Coverage.Margin)
	if len(problems) > 0 {
		return fmt.Errorf("%s", strings.Join(problems, "\n::error::coverage ratchet: "))
	}
	return nil
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

// checkWaiver reads the waiver Decision from GitHub with the job's token.
func checkWaiver(n int) error {
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
	return ratchet.AcceptedWaiver(is, "coverage", nil)
}
