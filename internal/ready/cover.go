package ready

import (
	"fmt"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// A Story is Ready only while its Feature's approval covers it (Feature #877,
// Story #996, Decision #861): the approval lists the story at its Completion
// Goal, effort, and title as they are now. Other edits to the story need no
// acknowledgment (#903).

// StoryOf is the story as an approval pins it.
func StoryOf(it workitems.Item) ApprovedStory {
	s := ApprovedStory{Number: it.Number, Title: it.Title, CompletionGoalHash: workitems.CompletionGoalHash(it)}
	for _, l := range it.Labels {
		if v, ok := strings.CutPrefix(l, "effort:"); ok {
			s.Effort = v
		}
	}
	return s
}

// StoryChanges says what changed in a story since the approval pinned it:
// "Completion Goal", "effort S to M", "title". It is empty when the approval
// still covers the story.
func StoryChanges(was, now ApprovedStory) []string {
	var out []string
	if was.CompletionGoalHash != now.CompletionGoalHash {
		out = append(out, "Completion Goal")
	}
	if was.Effort != now.Effort {
		out = append(out, fmt.Sprintf("effort %s to %s", or(was.Effort, "none"), or(now.Effort, "none")))
	}
	if was.Title != now.Title {
		out = append(out, "title")
	}
	return out
}

func or(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
