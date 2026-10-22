package hosts

import (
	"regexp"
	"slices"
	"strings"
)

// Amp says which model ran a thread and in which agent mode only through its
// CLI, which has no JSON form for either (amp CLI as run in an orb on
// 2026-10-18):
//
//   - `amp threads usage <thread> --details` prints Markdown with a
//     "## Model Routing" table whose Agent rows are the thread's own models
//     (Task, Thread Title, Read Thread, and View Media rows are helpers);
//   - `amp threads markdown <thread>` starts with YAML frontmatter holding
//     agentMode (low, medium, high, ...), which is the thread's effort.
//
// The parsers below read that output defensively and return nothing rather
// than a guess when its shape changes.

// ampToken is a value aayu writes as a one-token trailer: no spaces, no
// Markdown.
var ampToken = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// AmpAgentModels reads the output of `amp threads usage <thread> --details`
// into the model IDs of its Model Routing table's Agent rows, each once, in
// table order, without the provider prefix (anthropic/claude-opus-5-5 is
// claude-opus-5-5). The columns are found by their header names. It returns
// nil when the table or its Model ID and Purpose columns are missing.
func AmpAgentModels(usage string) []string {
	var models []string
	in, model, purpose := false, -1, -1
	for _, l := range strings.Split(strings.ReplaceAll(usage, "\r\n", "\n"), "\n") {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "#") {
			if in {
				break // the next section
			}
			in = strings.TrimSpace(strings.TrimLeft(l, "#")) == "Model Routing"
			continue
		}
		if !in || !strings.HasPrefix(l, "|") {
			continue
		}
		// An escaped pipe (in a routing link's name) is not a column break.
		cells := strings.Split(strings.Trim(strings.ReplaceAll(l, `\|`, "\x00"), "|"), "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		if model < 0 || purpose < 0 {
			// The first row is the header.
			model, purpose = slices.Index(cells, "Model ID"), slices.Index(cells, "Purpose")
			if model < 0 || purpose < 0 {
				return nil
			}
			continue
		}
		if max(model, purpose) >= len(cells) || cells[purpose] != "Agent" {
			continue
		}
		id := cells[model][strings.LastIndex(cells[model], "/")+1:]
		if ampToken.MatchString(id) && !slices.Contains(models, id) {
			models = append(models, id)
		}
	}
	return models
}

// AmpAgentMode reads the agentMode key from the YAML frontmatter that starts
// the output of `amp threads markdown <thread>`, or "" without one. Only the
// frontmatter counts, so a line in the conversation is never read as it.
func AmpAgentMode(markdown string) string {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return ""
	}
	for _, l := range lines[1:] {
		if strings.TrimSpace(l) == "---" {
			break
		}
		if v, ok := strings.CutPrefix(l, "agentMode:"); ok {
			v = strings.Trim(strings.TrimSpace(v), `"'`)
			if ampToken.MatchString(v) {
				return v
			}
			return ""
		}
	}
	return ""
}
