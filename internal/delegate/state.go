// Package delegate hands a Ready story to another coding agent (WP5.5b):
// the Codex, Cursor, Claude Code, or Amp cloud agent, or its CLI on this
// machine in an aayu worktree. One brief serves every receiver; adapters
// change only the transport. The receiver never gets merge rights: aayu finds
// or opens the PR, inspect reviews it, and ship merges it after a human pass.
package delegate

import (
	"fmt"

	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// Receivers.
const (
	Codex  = "codex"
	Cursor = "cursor"
	Claude = "claude"
	Amp    = "amp"
	// OpenCode has no hosted cloud agent: its stories and its Kanha run as
	// local sessions of its CLI (Story #1012, #893 story F).
	OpenCode = "opencode"
	// Copilot runs a mission's Kanha as a local session (Story #1012); its
	// story receivers are #917's.
	Copilot = "copilot"
)

// Receivers lists the agents a story can be delegated to.
var Receivers = []string{Codex, Cursor, Claude, Amp, OpenCode}

// Names are the receivers' product names, for messages.
var Names = map[string]string{Codex: "Codex", Cursor: "Cursor", Claude: "Claude Code", Amp: "Amp", OpenCode: "OpenCode", Copilot: "GitHub Copilot"}

// LedgerPath is where a story's delegation state lives on the ledger.
func LedgerPath(story int) string { return fmt.Sprintf("delegate/%d/state.json", story) }

// Family is the model family of a receiver's agent when the receiver fixes
// it: Codex runs OpenAI models and Claude Code Anthropic's. Cursor and Amp
// run the model their user or mode selects, so the family is the configured
// model's, else host-managed (unknown to aayu; inspect's reviewer must then
// name a concrete family, models.CheckFamilies).
func Family(receiver, model string) verbs.Family {
	if f := verbs.FamilyOf(model); f != "" {
		return f
	}
	switch receiver {
	case Codex:
		return verbs.FamilyOpenAI
	case Claude:
		return verbs.FamilyAnthropic
	}
	return verbs.FamilyHostManaged
}

// State is delegate/<story>/state.json: the hand-off, the receiver's task,
// the PR, and every follow-up. inspect reads Receiver and Family to judge the
// PR with reviewers of other families.
type State struct {
	Story     int          `json:"story"`
	Feature   int          `json:"feature,omitempty"`   // the Feature whose mission started it (Story #1004)
	Kanha     string       `json:"kanha,omitempty"`     // that mission's Kanha: its session
	KanhaURL  string       `json:"kanha_url,omitempty"` // and its URL
	Receiver  string       `json:"receiver"`
	Mode      string       `json:"mode"` // cloud or local
	Model     string       `json:"model,omitempty"`
	Family    verbs.Family `json:"family"`
	Task      string       `json:"task,omitempty"` // the receiver's ID: Cursor agent, Amp thread, Codex task
	TaskURL   string       `json:"task_url,omitempty"`
	Env       string       `json:"env,omitempty"` // the cloud environment of the latest hand-off or restart (a follow-up keeps it): Codex's [delegate] codex_env (Bug #1149)
	Branch    string       `json:"branch,omitempty"`
	PR        int          `json:"pr,omitempty"`
	PRURL     string       `json:"pr_url,omitempty"`
	Status    string       `json:"status"` // handed-off, host-handoff, lapsed, working, pr-open, fix-requested, merge-ready, capped, failed
	BriefHash string       `json:"brief_sha256"`
	StartedAt string       `json:"started_at"`
	Handoffs  int          `json:"handoffs,omitempty"` // hand-offs of this story so far; each gets a fresh branch and worktree
	Proved    string       `json:"proved,omitempty"`   // the receiver's own "What I proved" section (untrusted; a preview)
	Reason    string       `json:"reason,omitempty"`   // why this receiver (auto)
	Applied   string       `json:"applied,omitempty"`  // the last task whose returned diff aayu applied (Codex)
	FollowUps []FollowUp   `json:"follow_ups,omitempty"`
	Events    []Event      `json:"events,omitempty"`
}

// FollowUp is one request for fixes sent to the receiver.
type FollowUp struct {
	At       string   `json:"at"`
	Head     string   `json:"head"`
	Cycle    int      `json:"cycle"`
	Findings []string `json:"findings"`         // inspect finding IDs
	Checks   []string `json:"checks,omitempty"` // failed required checks (a CI repair round, P3)
}

// Event is one line of the delegation's history.
type Event struct {
	At     string `json:"at"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
	Until  string `json:"until,omitempty"` // a pause: when the receiver's usage limit resets (RFC 3339)
	Text   string `json:"text,omitempty"`  // a Kanha event: the line its Kanha is told (Story #1012)
}
