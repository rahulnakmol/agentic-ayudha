package identity

import (
	"bufio"
	"fmt"
	"slices"
	"strings"
)

// CodeownersRule is one line of .github/CODEOWNERS.
type CodeownersRule struct {
	Pattern string
	Owners  []string // logins without "@"; team entries (org/team) are kept as written
}

// ParseCodeowners parses CODEOWNERS lines; comments and blank lines are skipped.
func ParseCodeowners(text string) []CodeownersRule {
	var rules []CodeownersRule
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		line, _, _ := strings.Cut(sc.Text(), "#")
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		r := CodeownersRule{Pattern: f[0]}
		for _, o := range f[1:] {
			r.Owners = append(r.Owners, strings.TrimPrefix(o, "@"))
		}
		rules = append(rules, r)
	}
	return rules
}

// supported reports whether the gate can evaluate a CODEOWNERS pattern: only
// root-anchored literal paths ("/dir/" for a directory, "/file" for a file or
// directory), which is all this repository uses. Anything else makes the gate
// fail closed rather than silently treat a protected path as unprotected.
func supported(pattern string) bool {
	return strings.HasPrefix(pattern, "/") && len(pattern) > 1 && !strings.ContainsAny(pattern, "*?[\\!")
}

func match(pattern, path string) bool {
	p := strings.TrimPrefix(pattern, "/")
	if strings.HasSuffix(p, "/") {
		return strings.HasPrefix(path, p)
	}
	return path == p || strings.HasPrefix(path, p+"/")
}

// Owners returns the owners of path: the last matching rule wins, as on GitHub.
// ok is false when no rule matches (the path is not protected).
func Owners(rules []CodeownersRule, path string) (owners []string, pattern string, ok bool) {
	for _, r := range rules {
		if match(r.Pattern, path) {
			owners, pattern, ok = r.Owners, r.Pattern, true
		}
	}
	return owners, pattern, ok
}

// GateInput is what the protected-paths workflow fetches from the API.
type GateInput struct {
	Codeowners string
	Changed    []string // every changed path, including the old path of a rename
	Author     string
	HeadSHA    string
	Reviews    []Review
	BotLogins  []string
}

// GateResult is the protected-paths decision and its reasons, one line per path.
type GateResult struct {
	Pass  bool
	Lines []string
}

// ProtectedPathsGate implements the protected-paths check (WP0.1, extended by
// WP1.4 per Appendix B D2): it passes when no changed path is protected, or
// when every protected path has an approval on the head commit from one of its
// human CODEOWNERs who is not the PR author and not a bot.
func ProtectedPathsGate(in GateInput) GateResult {
	rules := ParseCodeowners(in.Codeowners)
	for _, r := range rules {
		if !supported(r.Pattern) {
			return GateResult{Lines: []string{fmt.Sprintf("FAIL CODEOWNERS pattern %q is not a root-anchored literal path; the gate cannot evaluate it and fails closed", r.Pattern)}}
		}
	}
	latest := LatestReviews(in.Reviews)
	res := GateResult{Pass: true}
	paths := slices.Clone(in.Changed)
	slices.Sort(paths)
	for _, path := range slices.Compact(paths) {
		owners, pattern, ok := Owners(rules, path)
		if !ok {
			continue
		}
		var approver string
		var why []string
		for _, r := range latest {
			if !slices.ContainsFunc(owners, func(o string) bool { return strings.EqualFold(o, r.User.Login) }) {
				continue
			}
			if err := HumanPass(r, in.Author, in.HeadSHA, in.BotLogins); err != nil {
				why = append(why, err.Error())
				continue
			}
			approver = r.User.Login
			break
		}
		if approver != "" {
			res.Lines = append(res.Lines, fmt.Sprintf("ok   %s (%s): approved by CODEOWNER %s on the head commit", path, pattern, approver))
			continue
		}
		res.Pass = false
		line := fmt.Sprintf("FAIL %s (%s): needs an approval on the head commit from a human CODEOWNER (%s) who is not the author",
			path, pattern, "@"+strings.Join(owners, ", @"))
		if len(why) > 0 {
			line += "; " + strings.Join(why, "; ")
		}
		res.Lines = append(res.Lines, line)
	}
	if len(res.Lines) == 0 {
		res.Lines = []string{"No protected paths changed."}
	}
	return res
}
