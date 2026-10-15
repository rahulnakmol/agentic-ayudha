// Command protected-paths-gate decides the protected-paths check from API
// responses the workflow saved to files (.github/workflows/protected-paths.yml).
// It runs from the base branch's checkout and never sees PR code; all logic is
// identity.ProtectedPathsGate, which is fixture-tested.
//
// Usage:
//
//	protected-paths-gate -codeowners FILE -pr FILE -files FILE -reviews FILE [-repo DIR]
//
// -pr, -files, and -reviews hold `gh api` output for the pull request, its files
// (paginated pages may be concatenated), and its reviews. -repo is the base
// checkout, whose .aayudha/config.toml [identity] bot_logins is honoured.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
)

func main() {
	pass, err := run(os.Args[1:], os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "protected-paths-gate:", err)
		os.Exit(2)
	}
	if !pass {
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) (bool, error) {
	fs := flag.NewFlagSet("protected-paths-gate", flag.ContinueOnError)
	codeowners := fs.String("codeowners", "", "base branch CODEOWNERS file")
	prFile := fs.String("pr", "", "pull request JSON")
	filesFile := fs.String("files", "", "pull request files JSON")
	reviewsFile := fs.String("reviews", "", "pull request reviews JSON")
	repo := fs.String("repo", "", "base branch checkout (for [identity] bot_logins)")
	if err := fs.Parse(args); err != nil {
		return false, err
	}
	co, err := os.ReadFile(*codeowners)
	if err != nil {
		return false, err
	}
	var pr struct {
		User identity.Account `json:"user"`
		Head struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}
	if err := decodeAll(*prFile, func(d *json.Decoder) error { return d.Decode(&pr) }); err != nil {
		return false, err
	}
	var changed []string
	err = decodeAll(*filesFile, func(d *json.Decoder) error {
		var page []struct {
			Filename         string `json:"filename"`
			PreviousFilename string `json:"previous_filename"`
		}
		if err := d.Decode(&page); err != nil {
			return err
		}
		for _, f := range page {
			changed = append(changed, f.Filename)
			if f.PreviousFilename != "" {
				changed = append(changed, f.PreviousFilename)
			}
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	var reviews []identity.Review
	err = decodeAll(*reviewsFile, func(d *json.Decoder) error {
		var page []identity.Review
		err := d.Decode(&page)
		reviews = append(reviews, page...)
		return err
	})
	if err != nil {
		return false, err
	}
	var botLogins []string
	if *repo != "" {
		l, err := config.Load(config.Sources{RepoDir: *repo})
		if err != nil {
			return false, fmt.Errorf("base config: %w", err)
		}
		botLogins = l.Config.Identity.BotLogins
	}
	if pr.Head.SHA == "" || pr.User.Login == "" {
		return false, errors.New("pull request JSON has no head.sha or user.login")
	}
	res := identity.ProtectedPathsGate(identity.GateInput{
		Codeowners: string(co), Changed: changed, Author: pr.User.Login, HeadSHA: pr.Head.SHA,
		Reviews: reviews, BotLogins: botLogins,
	})
	for _, l := range res.Lines {
		fmt.Fprintln(out, l)
	}
	return res.Pass, nil
}

// decodeAll calls fn for each JSON value in the file (gh api --paginate
// concatenates one array per page).
func decodeAll(path string, fn func(*json.Decoder) error) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	d := json.NewDecoder(f)
	for d.More() {
		if err := fn(d); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	return nil
}
