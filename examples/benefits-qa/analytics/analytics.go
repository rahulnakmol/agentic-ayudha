// Package analytics turns the benefits-qa example's product events into an
// Application Insights export (layer L08, docs/BUILD-PLAN.md WP8.1): JSON
// lines in the Log Analytics shape (TimeGenerated, Name, UserAuthenticatedId)
// that `aayu measure --appinsights` reads. Simulate drives a synthetic,
// seeded population of employees through a channel and writes the export the
// example's Goal is measured on; no real person's data is ever involved (V3).
package analytics

import (
	"encoding/json"
	"io"
	"sync"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/ask"
)

// Line is one exported event.
type Line struct {
	TimeGenerated       string `json:"TimeGenerated"`
	Name                string `json:"Name"`
	UserAuthenticatedId string `json:"UserAuthenticatedId"` // the Application Insights column name
}

// AppInsights writes each event as one JSON line. It is safe for concurrent use.
type AppInsights struct {
	mu sync.Mutex
	w  io.Writer
}

// NewAppInsights writes events to w.
func NewAppInsights(w io.Writer) *AppInsights { return &AppInsights{w: w} }

// Record implements ask.Recorder. An event without a user is not exported:
// measure keys every metric on the user.
func (a *AppInsights) Record(e ask.Event) error {
	if e.User == "" {
		return nil
	}
	b, _ := json.Marshal(Line{TimeGenerated: e.Time.UTC().Format(time.RFC3339), Name: e.Name, UserAuthenticatedId: e.User}) // three strings always marshal
	a.mu.Lock()
	defer a.mu.Unlock()
	_, err := a.w.Write(append(b, '\n'))
	return err
}
