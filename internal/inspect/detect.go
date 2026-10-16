package inspect

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

// Detected is a reviewer app found on a repository, and how.
type Detected struct {
	App    string `json:"app"`
	Source string `json:"source"`
}

// DetectApps finds the reviewer apps installed for repo, read-only, through
// get (a GitHub REST GET with a person's token). A personal access token
// cannot list an account's app installations, so it reads what it can: an
// organisation's installations (with admin:org read), then the apps' own
// replies and check runs on the newest pull requests. An app installed but
// never used on a personal account is not found: config's [inspect] apps
// names it then.
func DetectApps(ctx context.Context, get func(ctx context.Context, path string, out any) error, repo string) []Detected {
	var out []Detected
	add := func(app, src string) {
		if !slices.ContainsFunc(out, func(d Detected) bool { return d.App == app }) {
			out = append(out, Detected{app, src})
		}
	}
	bySlug := func(slug string) string {
		for _, a := range Apps {
			if a.Slug != "" && strings.EqualFold(a.Slug, slug) {
				return a.Name
			}
		}
		return ""
	}
	byLogin := func(login string) string {
		for _, a := range Apps {
			if a.Login != "" && strings.EqualFold(a.Login, login) {
				return a.Name
			}
		}
		return ""
	}
	owner, _, _ := strings.Cut(repo, "/")
	var inst struct {
		Installations []struct {
			AppSlug string `json:"app_slug"`
		} `json:"installations"`
	}
	if get(ctx, "/orgs/"+owner+"/installations?per_page=100", &inst) == nil {
		for _, i := range inst.Installations {
			if a := bySlug(i.AppSlug); a != "" {
				add(a, "installed on the organisation "+owner)
			}
		}
	}
	var pulls []struct {
		Number int `json:"number"`
		Head   struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}
	if get(ctx, "/repos/"+repo+"/pulls?state=all&per_page=10", &pulls) != nil {
		return out
	}
	type post struct {
		User struct {
			Login string `json:"login"`
		} `json:"user"`
	}
	for _, p := range pulls {
		var cs, rs []post
		_ = get(ctx, fmt.Sprintf("/repos/%s/issues/%d/comments?per_page=100", repo, p.Number), &cs)
		_ = get(ctx, fmt.Sprintf("/repos/%s/pulls/%d/reviews?per_page=100", repo, p.Number), &rs)
		for _, x := range append(cs, rs...) {
			if a := byLogin(x.User.Login); a != "" {
				add(a, fmt.Sprintf("%s replied on #%d", x.User.Login, p.Number))
			}
		}
		var checks struct {
			CheckRuns []struct {
				Name string `json:"name"`
				App  struct {
					Slug string `json:"slug"`
				} `json:"app"`
			} `json:"check_runs"`
		}
		if p.Head.SHA != "" && get(ctx, fmt.Sprintf("/repos/%s/commits/%s/check-runs?per_page=100", repo, p.Head.SHA), &checks) == nil {
			for _, c := range checks.CheckRuns {
				if a := bySlug(c.App.Slug); a != "" {
					add(a, fmt.Sprintf("check run %q on #%d", c.Name, p.Number))
				}
			}
		}
	}
	return out
}
