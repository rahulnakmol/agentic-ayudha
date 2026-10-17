package production

import (
	"fmt"
	"regexp"

	"github.com/rahulnakmol/agentic-ayudha/internal/seats"
)

// Postmortem is the blameless postmortem `aayu respond --postmortem` checks,
// written from the incident's timeline and its investigation note.
var Postmortem = Kind{
	Name: "postmortem", Practice: "incident-command", Marker: "<!-- aayudha:postmortem -->",
	Sections: []Section{
		{Title: "Summary", What: "What happened, to whom, and how it ended, in two or three sentences."},
		{Title: "Impact", What: "Who was affected, how badly, and for how long (UTC), cited to the bounded queries or the timeline."},
		{Title: "Timeline", What: "The events in order with UTC times, from the incident timeline (`#<incident>`): detection, the investigation, the proposed and approved catalog actions, recovery.",
			must: []rule{must(`\b\d{1,2}:\d{2}\b`, "give each event its UTC time")}},
		{Title: "Causes", What: "The root cause and the contributing conditions, as systems, changes, and decisions with their evidence (a commit, a `path:line`, a query); never a person.",
			must: []rule{must(`root cause`, "name the root cause"), must(`contribut`, "name the contributing conditions")}},
		{Title: "What went well", What: "What helped: detection, a runbook, a rollback that ran clean, an alert that fired."},
		{Title: "Actions", What: "Each action that removes a class of failure, with its owner (a seat or a person who agreed) and its Bug or Story; the follow-up Bug of the durable fix first.",
			must: []rule{must(`owner`, "give each action an owner"), must(`(#\d+|follow-up bug)`, "link each action to its Bug or Story, or say the follow-up Bug files on close")}},
	},
}

// blame finds language that puts an incident on a person rather than on the
// system, the conditions, and the decisions that let it happen.
var blame = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(blamed?|blames|at fault|fault of|careless(ly|ness)?|negligen(t|ce)|incompeten(t|ce)|sloppy|lazy|should have known|should have caught|human error|operator error|failed to notice|dropped the ball)\b`),
	regexp.MustCompile(`@[A-Za-z0-9-]+\s+(broke|caused|forgot|missed|ignored|failed to|should have)\b`),
}

// CheckPostmortem is every deterministic check on a postmortem: its parts in
// order, the stage's artefact, the evidence rule (known resolves each
// citation against the incident, its timeline, and its investigation note),
// the incident cited, and blameless language.
func CheckPostmortem(md string, incident int, stage string, pr seats.Practice, known func(string) bool) []string {
	out := Postmortem.CheckParts(md)
	out = append(out, checkCommon(md, stage, pr, known)...)
	if tl, ok := Part(md, "Timeline"); ok && !regexp.MustCompile(fmt.Sprintf(`#%d\b`, incident)).MatchString(tl) {
		out = append(out, fmt.Sprintf("part \"Timeline\": cite the incident's timeline (#%d)", incident))
	}
	for _, re := range blame {
		for _, m := range re.FindAllString(md, -1) {
			out = append(out, fmt.Sprintf("%q blames a person; a blameless postmortem names the system, the conditions, and the decisions", m))
		}
	}
	return append(out, checkQueryTemplates(md)...)
}
