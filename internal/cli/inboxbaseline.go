package cli

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// Baseline cards (Story #1172): each baseline draft of an open Feature that
// waits for a person is a card in aayu inbox, one per part, with the counsel
// of the seat that owns the part. The card shows what approving freezes: the
// part, its version, its pins, its checklist, and each change since the
// version before. Approving runs `aayu ready <f> --part <p>` (partRun.approve)
// pinned to the card's hash, so the hash a person sees is the hash written,
// and one device code covers every card of the session (presenceCheck). An
// agent lane sees the cards and no approve action (listInbox).

// baselineChanged is what a card says when its part's pins moved after the draft.
func baselineChanged(part string, f int) string {
	return fmt.Sprintf("The %s of #%d changed after the draft. An agent drafts it again.", ready.Label(part), f)
}

// baselineCards reads the baseline drafts of features that wait for a person
// on the ledger, each with its part's pins as they are now (partRun.pins, the
// code approve checks them with), as inbox cards. A UX draft of a Feature
// with no L01 in its footprint has no card. It writes nothing.
func (e *Env) baselineCards(ctx context.Context, repo string, features []int) ([]InboxItem, error) {
	if config.RepoRoot(e.Dir) == "" || len(features) == 0 { // no checkout, so no ledger to read
		return nil, nil
	}
	c, tok, err := e.HumanGitHub(ctx)
	if err != nil {
		return nil, err
	}
	w, err := e.Ledger(ctx, tok, "human", "")
	if err != nil {
		return nil, err
	}
	t, err := w.Read(ctx) // one read: each part's records come from the same tree, so a listing and its records agree
	if err != nil {       // no ledger yet: nothing is drafted
		return nil, ignoreMissing(err)
	}
	defer t.Close()
	var out []InboxItem
	for _, f := range features {
		for _, part := range ready.Parts {
			r := &partRun{e: e, c: c, w: w, repo: repo, n: f, part: part}
			base, draft, err := r.recordsIn(ctx, t)
			if err != nil {
				return nil, err
			}
			if draft == nil {
				continue
			}
			it, pins, err := r.feature(ctx)
			switch {
			case part == "ux" && !slices.Contains(e.footprint(it), string(verbs.L01)):
				continue
			case CodeOf(err) == ExitBlocked: // the part has nothing to pin now: it changed
			case err != nil:
				return nil, err
			}
			holds := err == nil && ready.PinsHash(pins) == ready.PinsHash(draft.Pins)
			var stories []workitems.Item
			if part == "spec" {
				subs, err := featureStories(ctx, c, repo, f)
				if err != nil {
					return nil, err
				}
				for _, si := range subs {
					s, _ := workitems.Parse(si.GetNumber(), si.GetTitle(), si.GetBody(), si.GetType().GetName(), github.IssueLabels(si))
					stories = append(stories, s)
				}
			}
			out = append(out, baselineCard(*draft, base, holds, stories, e.footprint(it)))
		}
	}
	return out, nil
}

// baselineCard is the card of draft d, whose part's pins now hold (or not),
// after the approved version base (nil for v1). stories are a spec draft's
// stories as they are now, and footprint the Feature's layers.
func baselineCard(d ready.Baseline, base *ready.Baseline, holds bool, stories []workitems.Item, footprint []string) InboxItem {
	f, part, label := d.Feature, d.Part, ready.Label(d.Part)
	it := InboxItem{Kind: InboxBaseline, Number: f, Ask: part, Title: fmt.Sprintf("%s v%d", ready.Title(part), d.Version),
		Seat: verbs.SeatID(ready.Owner(part)), Hash: d.Hash, CounselFrom: "facts"}
	at, _, _ := strings.Cut(d.DraftedAt, "T")
	var lines []string
	if !holds {
		lines = append(lines, baselineChanged(part, f))
	}
	lines = append(lines, fmt.Sprintf("Part     %s v%d, drafted by %s on %s", label, d.Version, termText(or(d.DraftedBy, "an agent")), termText(at)),
		fmt.Sprintf("Hash     %s is what you approve", d.Hash[:12]))

	checks := []InboxCheck{{Check: "nothing it pins changed since the draft", OK: holds}}
	if !holds {
		checks[0].Detail = "an agent drafts it again"
	}
	var storyPins []ready.StoryPin
	for _, p := range d.Pins {
		if p.Story != nil {
			storyPins = append(storyPins, *p.Story)
		}
	}
	if part == "spec" {
		lines = append(lines, fmt.Sprintf("Stories  %d, each pinned by its title, Completion Goal, effort, layers, and depends-on; and the set of stories", len(storyPins)))
		var none, xl, outside []string
		for _, sp := range storyPins {
			count := "? criteria" // a story read now that is not the one pinned: its criteria are not the pinned ones
			if i := slices.IndexFunc(stories, func(s workitems.Item) bool { return s.Number == sp.Number }); i >= 0 && workitems.CompletionGoalHash(stories[i]) == sp.CompletionGoalHash {
				cs, _ := workitems.ParseCriteria(stories[i].Fields[workitems.FieldCriteria])
				count = fmt.Sprintf("%d %s", len(cs), plural(len(cs), "criterion", "criteria"))
				if len(cs) == 0 {
					none = append(none, fmt.Sprintf("#%d", sp.Number))
				}
			}
			lines = append(lines, fmt.Sprintf("  #%d %s · %s · effort %s", sp.Number, termText(sp.Title), count, termText(or(sp.Effort, "none"))))
			if sp.Effort == "XL" {
				xl = append(xl, fmt.Sprintf("#%d", sp.Number))
			}
			switch out := slices.DeleteFunc(slices.Clone(sp.Layers), func(l string) bool { return slices.Contains(footprint, l) }); {
			case len(sp.Layers) == 0:
				outside = append(outside, fmt.Sprintf("#%d names no layer", sp.Number))
			case len(out) > 0:
				outside = append(outside, fmt.Sprintf("#%d is in %s, outside %s", sp.Number, termText(strings.Join(out, ", ")), or(strings.Join(footprint, ", "), "an empty footprint")))
			}
		}
		checks = append(checks,
			InboxCheck{Check: "each story has criteria", OK: len(none) == 0, Detail: joinFail(none, "no criteria")},
			InboxCheck{Check: "no story is XL", OK: len(xl) == 0, Detail: joinFail(xl, "XL")},
			InboxCheck{Check: "each story is in the Feature's paths", OK: len(outside) == 0, Detail: strings.Join(outside, "; ")})
	} else {
		lines = append(lines, "Pins")
		for _, p := range d.Pins {
			lines = append(lines, fmt.Sprintf("  %s · %s", termText(p.Ref), termText(p.Hash[:min(len(p.Hash), 12)])))
		}
	}
	if base != nil {
		lines = append(lines, fmt.Sprintf("Changes since v%d:", base.Version))
		for _, ch := range d.Changes {
			lines = append(lines, "  "+termText(ch.Ref)+": "+termText(ch.What), "    Why: "+termText(ch.Why), "    Need: "+termText(ch.Need))
		}
		_, missing, _, unknown := ready.Reasons(ready.Diff(base.Pins, d.Pins), d.Changes, 1)
		checks = append(checks, InboxCheck{Check: fmt.Sprintf("each change since v%d says what changed, why, and the need behind it", base.Version),
			OK: len(missing)+len(unknown) == 0, Detail: termText(strings.Join(append(missing, unknown...), "; "))})
	}
	ok := 0
	lines = append(lines, "Checklist")
	for _, c := range checks {
		mark := "✓ "
		if c.OK {
			ok++
		} else {
			mark = "✗ "
		}
		line := "  " + mark + c.Check
		if !c.OK && c.Detail != "" {
			line += ": " + c.Detail
		}
		lines = append(lines, line)
	}
	it.Lines, it.Checklist = lines, checks

	if !holds {
		it.Why, it.f.problem = baselineChanged(part, f), "changed after the draft"
		it.Options = []InboxAction{{N: 1, Label: "leave it for the agent's new draft", Command: inboxLeave}}
		it.Counsel = fmt.Sprintf("What moved: the pins of %s v%d of #%d no longer match the draft, so approving it now would freeze what you have not seen. "+
			"The card offers no approval, and aayu ready refuses one too, before and after the device code. The next move is an agent's: it drafts "+
			"the %s again and says what changed and why. Nothing for you yet; the card comes back with the new draft.", label, d.Version, f, label)
		return it
	}
	it.Why = fmt.Sprintf("%s v%d of #%d waits for a person; approving reads the draft and the pins again after the device code.", ready.Title(part), d.Version, f)
	it.f.problem = fmt.Sprintf("%s v%d to approve", label, d.Version)
	it.Options = []InboxAction{
		{N: 1, Recommended: ok == len(checks), Label: fmt.Sprintf("approve %s v%d of #%d at hash %s", label, d.Version, f, d.Hash[:12]), Command: fmt.Sprintf("aayu ready %d --part %s", f, part)},
		{N: 2, Label: "leave it waiting: nothing is frozen", Command: inboxLeave},
	}
	tally := fmt.Sprintf("%d of %d checks hold", ok, len(checks))
	switch part {
	case "intent":
		it.Counsel = fmt.Sprintf("What you agree to: the problem #%d says it solves, frozen as its body at hash %s. Read it as the person who would read "+
			"the announcement; if they would not care, leave it and ask for a sharper draft. %s. A later edit shows as drift, and the next version "+
			"must say what changed and why. You decide; I only advise.", f, d.Hash[:12], tally)
	case "spec":
		it.Counsel = fmt.Sprintf("What you agree to: %d stories of #%d, each frozen by its title, Completion Goal, effort, layers, and dependencies, "+
			"at hash %s. %s. Approve only stories you could prove done one by one; an XL story or one with no criteria is a reason to leave it. "+
			"A later edit shows as drift, and the next version says why. You decide; I only advise.", len(storyPins), f, d.Hash[:12], tally)
	case "tech":
		it.Counsel = fmt.Sprintf("The contract you freeze: %d %s of #%d, with %s first, at hash %s. %s. The alternative is to leave it and ask for "+
			"a new draft. Approve when these boundaries are the ones you want built against; after that each edit to them is drift, and the next "+
			"version names what changed, why, and the need. You decide; I only advise.", len(d.Pins), plural(len(d.Pins), "pin", "pins"), f, termText(d.Pins[0].Ref), d.Hash[:12], tally)
	default:
		it.Counsel = fmt.Sprintf("What you freeze: %d design %s of #%d, at hash %s. %s. Open the frames before you approve: a person's task should "+
			"read step by step in them, in the person's own words. A later edit to a stage shows as drift, and the next version says what changed "+
			"and why. You decide; I only advise.", len(d.Pins), plural(len(d.Pins), "record", "records"), f, d.Hash[:12], tally)
	}
	return it
}

// joinFail is "#21 and #22: no criteria", or "" when none fail.
func joinFail(refs []string, what string) string {
	if len(refs) == 0 {
		return ""
	}
	return joinWords(refs) + ": " + what
}
