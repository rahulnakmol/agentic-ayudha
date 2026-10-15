package doctor

import (
	"context"
	"fmt"
	"strings"
)

// FleetRow is one repository in `doctor --all`.
type FleetRow struct {
	Repo   string `json:"repo"`
	Status Status `json:"status"`
	Detail string `json:"detail"`
}

// Fleet checks each [fleet] repo remotely through the GitHub API: access,
// issue-type support, and what init would coexist with.
func Fleet(ctx context.Context, repos []string, api func(ctx context.Context, path string, out any) error) []FleetRow {
	rows := make([]FleetRow, 0, len(repos))
	for _, r := range repos {
		var repo struct {
			Visibility    string `json:"visibility"`
			DefaultBranch string `json:"default_branch"`
			Owner         struct {
				Type string `json:"type"`
			} `json:"owner"`
		}
		if api == nil {
			rows = append(rows, FleetRow{r, Warn, "no GitHub token"})
			continue
		}
		if err := api(ctx, "/repos/"+r, &repo); err != nil {
			rows = append(rows, FleetRow{r, Fail, "not accessible: " + err.Error()})
			continue
		}
		var parts []string
		parts = append(parts, repo.Visibility, map[bool]string{true: "org (issue types)", false: "personal (type labels)"}[repo.Owner.Type == "Organization"])
		var files []struct {
			Name string `json:"name"`
		}
		if err := api(ctx, "/repos/"+r+"/contents/.github/workflows", &files); err == nil {
			parts = append(parts, fmt.Sprintf("%d workflow(s)", len(files)))
		} else {
			parts = append(parts, "0 workflows")
		}
		var branches []struct {
			Name string `json:"name"`
		}
		hasDev := false
		if err := api(ctx, "/repos/"+r+"/branches?per_page=100", &branches); err == nil {
			for _, b := range branches {
				hasDev = hasDev || b.Name == "dev"
			}
		}
		if hasDev {
			parts = append(parts, "dev branch present")
		} else {
			parts = append(parts, fmt.Sprintf("default %s, no dev (init maps branches)", repo.DefaultBranch))
		}
		var rulesets []struct {
			Name string `json:"name"`
		}
		if err := api(ctx, "/repos/"+r+"/rulesets", &rulesets); err == nil {
			parts = append(parts, fmt.Sprintf("%d ruleset(s)", len(rulesets)))
		}
		rows = append(rows, FleetRow{r, Pass, strings.Join(parts, "; ")})
	}
	return rows
}
