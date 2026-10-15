package verify

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var hunk = regexp.MustCompile(`^@@ -[0-9]+(?:,[0-9]+)? \+([0-9]+)(?:,([0-9]+))? @@`)

// ParseDiff reads a unified diff with zero context (git diff -U0) and returns
// the added or modified line numbers of each file on the new side. Deleted
// files are left out.
func ParseDiff(diff []byte) map[string][]int {
	out := map[string][]int{}
	file := ""
	sc := bufio.NewScanner(bytes.NewReader(diff))
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "+++ "):
			file = ""
			if name, ok := strings.CutPrefix(line, "+++ b/"); ok {
				file = unquote(name)
				if _, seen := out[file]; !seen {
					out[file] = nil
				}
			}
		case file != "" && strings.HasPrefix(line, "@@"):
			m := hunk.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			start, _ := strconv.Atoi(m[1])
			n := 1
			if m[2] != "" {
				n, _ = strconv.Atoi(m[2])
			}
			for i := range n {
				out[file] = append(out[file], start+i)
			}
		}
	}
	return out
}

func unquote(s string) string {
	if strings.HasPrefix(s, `"`) {
		if u, err := strconv.Unquote(s); err == nil {
			return u
		}
	}
	return strings.TrimSuffix(s, "\t")
}

// Changes reads the diff between base and head in the checkout at root:
// every changed path (including deletions) and the changed lines per file.
func Changes(ctx context.Context, root, base, head string) ([]string, map[string][]int, error) {
	names, err := gitOut(ctx, root, "diff", "--name-only", "--no-renames", base, head)
	if err != nil {
		return nil, nil, err
	}
	diff, err := gitOut(ctx, root, "diff", "--unified=0", "--no-color", "--no-renames", "--no-ext-diff", base, head)
	if err != nil {
		return nil, nil, err
	}
	var paths []string
	for _, p := range strings.Split(strings.TrimSpace(string(names)), "\n") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	slices.Sort(paths)
	return paths, ParseDiff(diff), nil
}

func gitOut(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(cmd.Environ(), "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}
