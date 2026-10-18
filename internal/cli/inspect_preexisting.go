package cli

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/inspect"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// preFindingRe is how a filed Bug names each pre-existing finding it holds.
var preFindingRe = regexp.MustCompile("inspect finding `([0-9a-f]{10})`")

// filePreExisting files the gate's pre-existing findings on head as one Bug
// (Decision #544, point 2), under the person's client c, outside CI where a
// checkout exists: `git blame` at head names the commit that last changed
// each line, which the PR left unchanged. A finding an open Bug already holds
// (by finding ID) is not filed again. The Bug is always priority:P1, and a
// protected P0 among them pages. It returns a note for the result, "" when
// there was nothing to file.
func (e *Env) filePreExisting(ctx context.Context, c *github.Client, repo string, n int, head string, res *inspect.Result, page func(subject, body string)) (string, error) {
	if res == nil || res.Head != head {
		return "", nil
	}
	pre := inspect.PreExisting(*res)
	if len(pre) == 0 {
		return "", nil
	}
	owner, name, _ := github.SplitRepo(repo)
	open, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.Issue, *gh.Response, error) {
		return c.REST.Issues.ListByRepo(ctx, owner, name, &gh.IssueListByRepoOptions{State: "open", Labels: []string{"type:bug"}, ListOptions: o})
	})
	if err != nil {
		return "", err
	}
	// Only a Bug this account filed counts: anyone can open a type:bug issue
	// that names a finding's ID, and a decoy must not stop the filing or the
	// page.
	self := ""
	if u, _, err := c.REST.Users.Get(ctx, ""); err == nil {
		self = u.GetLogin()
	}
	filed := map[string]int{}
	for _, is := range open {
		if self == "" || !strings.EqualFold(is.GetUser().GetLogin(), self) {
			continue
		}
		for _, m := range preFindingRe.FindAllStringSubmatch(is.GetBody(), -1) {
			filed[m[1]] = is.GetNumber()
		}
	}
	var fresh []inspect.Finding
	var already []string
	for _, f := range pre {
		if b, ok := filed[f.ID]; ok {
			already = append(already, fmt.Sprintf("`%s` in #%d", f.ID, b))
			continue
		}
		fresh = append(fresh, f)
	}
	if len(fresh) == 0 {
		return "pre-existing findings already filed: " + strings.Join(already, ", "), nil
	}
	root := config.RepoRoot(e.Dir)
	urgent := slices.ContainsFunc(fresh, func(f inspect.Finding) bool { return f.Severity == "P0" && inspect.Protected(f.Category) })
	// The checkout may not hold the PR's head (someone else's PR): fetch it
	// from the PR's ref on origin, so blame can name each line's commit.
	if _, err := gitIn(ctx, root, "cat-file", "-e", head+"^{commit}"); err != nil {
		_, _ = gitIn(ctx, root, "fetch", "--quiet", "--no-tags", "origin", fmt.Sprintf("refs/pull/%d/head", n))
	}
	var list []string
	for _, f := range fresh {
		by := "commit unknown (the head is not in the local checkout and could not be fetched from origin)"
		if out, err := gitIn(ctx, root, "blame", "-L", fmt.Sprintf("%d,%d", f.Line, f.Line), "--porcelain", head, "--", f.File); err == nil {
			if sha, _, ok := strings.Cut(out, " "); ok && len(sha) >= 12 {
				by = "last changed by commit " + sha[:12]
			}
		}
		list = append(list, fmt.Sprintf("%d. inspect finding `%s`: %s %s at `%s:%d`, %s: %s. %s", len(list)+1, f.ID, f.Severity, f.Category, f.File, f.Line, by, clip(oneLine(f.Title), 200), clip(oneLine(f.Detail), 600)))
	}
	t, _ := workitems.FindType("bug")
	title := fmt.Sprintf("Pre-existing defects inspect found while reviewing PR #%d", n)
	body := workitems.Render(t, map[string]string{
		workitems.FieldSummary: fmt.Sprintf("inspect's review of PR #%d at `%s` found these on lines the PR does not change, so they did not block it (Decision #544, point 2).", n, short(head)),
		"reproduce":            strings.Join(list, "\n"),
		"expected":             "Each line behaves as the finding's detail says it should.",
		"actual":               "Each finding's detail says what goes wrong.",
	})
	// Through grill's own check (grillCheck), as `aayu grill bug --create`
	// files one: its labels, and no Create problem. The draft holds no
	// Completion Goal yet, so it is filed to the backlog, not Ready.
	g, err := e.grillCheck(ctx, c, repo, t, &grillDraft{Title: title, Body: body, Labels: "priority:P1"})
	if err != nil {
		return "", err
	}
	if len(g.Create) > 0 {
		return "", fmt.Errorf("grill refuses the pre-existing Bug at Create: %s", g.Create[0].Message)
	}
	labels := g.Labels
	bug := 0
	if _, err := c.Write(ctx, actions.Action{Kind: "github.issue.create", Target: repo, Seat: "product-strategist",
		Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/issues", repo)}, Payload: map[string]any{"title": title, "pre_existing_of": n, "findings": len(fresh)}},
		func(ctx context.Context) (any, error) {
			is, _, err := c.REST.Issues.Create(ctx, owner, name, gh.CreateIssueRequest{Title: title, Body: gh.Ptr(body), Labels: labels})
			bug = is.GetNumber()
			return map[string]any{"number": bug}, err
		}); err != nil {
		return "", err
	}
	if urgent && page != nil {
		page(fmt.Sprintf("inspect: a pre-existing P0 found on PR #%d", n), fmt.Sprintf("%s#%d holds a protected P0 on a line PR #%d does not change; filed as #%d.", repo, bug, n, bug))
	}
	note := fmt.Sprintf("pre-existing findings filed as Bug #%d", bug)
	if len(already) > 0 {
		note += "; already filed: " + strings.Join(already, ", ")
	}
	return note, nil
}
