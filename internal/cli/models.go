package cli

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"text/tabwriter"

	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

func init() { register("models", runModels) }

// PauseFile is the marker the [budget] breaker writes to pause agent lanes.
func (e *Env) PauseFile() string { return filepath.Join(e.stateDir(), "state", "paused") }

type modelRow struct {
	models.Resolved
	Route string `json:"route"`
}

func runModels(_ context.Context, e *Env, _ Values) error { // action: show
	l, err := e.Config()
	if err != nil {
		return err
	}
	r := models.Resolver{Models: l.Config.Models, Hosts: l.Config.Hosts, Log: func(s string) { e.Logf("%s", s) }}
	var rows []modelRow
	for _, lane := range verbs.Lanes {
		for _, h := range verbs.Hosts {
			if e.Host != "" && string(h.ID) != e.Host {
				continue
			}
			res, err := r.Resolve(lane.ID, h.ID)
			if err != nil {
				return Wrap(ExitUsage, err)
			}
			route := "host-managed (the host picks the model)"
			if rt, err := models.RouteFor(l.Config.Models, res.Model, res.Family); err == nil {
				route = fmt.Sprintf("%s %s %s (key: %s)", rt.Via, rt.Style, rt.BaseURL, rt.Secret)
			}
			rows = append(rows, modelRow{res, route})
		}
	}
	return e.Emit(rows, func(w io.Writer) error {
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "LANE\tHOST\tMODEL\tFAMILY\tSOURCE\tROUTE")
		for _, row := range rows {
			m := row.Model
			if m == "" {
				m = row.Display
			}
			if row.Effort != "" {
				m += " (effort " + row.Effort + ")"
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", row.Lane, row.Host, m, row.Family, row.Source, row.Route)
		}
		if err := tw.Flush(); err != nil {
			return err
		}
		for _, row := range rows {
			if row.Note != "" {
				fmt.Fprintf(w, "note: %s on %s: %s\n", row.Lane, row.Host, row.Note)
			}
		}
		return nil
	})
}
