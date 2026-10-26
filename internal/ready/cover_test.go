package ready

import (
	"slices"
	"testing"
)

// A story's refusal names each pinned field that changed since the approval,
// in the same order every time, and an effort the approval did not pin as none.
func TestStoryChanges(t *testing.T) {
	was := ApprovedStory{Number: 853, Title: "Story 853", CompletionGoalHash: "a"}
	now := ApprovedStory{Number: 853, Title: "Story 853, renamed", Effort: "S", CompletionGoalHash: "b"}
	if got, want := StoryChanges(was, now), []string{"Completion Goal", "effort none to S", "title"}; !slices.Equal(got, want) {
		t.Errorf("StoryChanges = %q, want %q", got, want)
	}
	if got := StoryChanges(now, now); len(got) != 0 {
		t.Errorf("StoryChanges of an unchanged story = %q", got)
	}
}
