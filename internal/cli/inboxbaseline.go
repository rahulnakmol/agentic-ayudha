package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/grill"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// Baseline cards (Story #1172) and Feature cards (Bug #1253): the baseline
// drafts of an open Feature that wait for a person are one card in aayu
// inbox, the Feature card. It shows each waiting part with its version, its
// hash, and each change since the version before, and the Approve to build
// that approving them leads to. Approve and start writes each part's approval
// pinned to the card's hash, then Approve to build, then the mission start,
// under one device code (presenceCheck); a part that moved writes nothing
// for the Feature. Opening a part shows it as its own Baseline card, with
// the counsel of the seat that owns the part, where approving runs `aayu
// ready <f> --part <p>` (partRun.approve) for that part only. An agent lane
// sees the cards and no approve action (listInbox).

// baselineChanged is what a card says when its part's pins moved after the draft.
func baselineChanged(part string, f int) string {
	return fmt.Sprintf("The %s of #%d changed after the draft. An agent drafts it again.", ready.Label(part), f)
}

// baselineCards reads the baseline drafts of features that wait for a person
// on the ledger, each with its part's pins as they are now (partRun.pins, the
// code approve checks them with), as one Feature card per Feature. A UX draft
// of a Feature with no L01 in its footprint is not on it. A Feature with no
// draft waiting and every part approved has its Approve to build card
// (approveCard) instead. It writes nothing.
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
		var unapproved []string // the parts with no approved version: Approve to build waits for them (Story #1173)
		var parts []InboxItem
		var lines []string
		drafts := map[string]*ready.Baseline{} // the drafts whose pins hold: the Approve to build preview counts them approved
		for _, part := range ready.Parts {
			r := &partRun{e: e, c: c, w: w, repo: repo, n: f, part: part}
			base, draft, err := r.recordsIn(ctx, t)
			if err != nil {
				return nil, err
			}
			if base == nil {
				unapproved = append(unapproved, part)
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
			if holds {
				drafts[part] = draft
			}
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
			card := baselineCard(*draft, base, holds, stories, e.footprint(it))
			parts, lines = append(parts, card), append(lines, partLines(card, *draft, base, holds)...)
		}
		build, err := e.approveCard(ctx, c, w, t, repo, f, unapproved, drafts)
		if err != nil {
			return nil, err
		}
		switch {
		case len(parts) > 0:
			out = append(out, featureCard(f, parts, lines, build))
		case build != nil:
			out = append(out, *build)
		}
	}
	return out, nil
}

// partLines are what a Feature card says about its part p, the card of
// draft d after the approved version base (nil for v1): its version, its
// hash, its checks, and each change since base, or that its pins moved.
func partLines(p InboxItem, d ready.Baseline, base *ready.Baseline, holds bool) []string {
	ok := 0
	for _, c := range p.Checklist {
		if c.OK {
			ok++
		}
	}
	lines := []string{fmt.Sprintf("  %s v%d · hash %s · %d of %d checks hold", ready.Title(d.Part), d.Version, d.Hash[:12], ok, len(p.Checklist))}
	if !holds {
		return append(lines, "    "+baselineChanged(d.Part, d.Feature))
	}
	if base != nil {
		lines = append(lines, fmt.Sprintf("    Changes since v%d:", base.Version))
		for _, ch := range d.Changes { // the why and the need too: the hash approves them
			lines = append(lines, "      "+termText(ch.Ref)+": "+termText(ch.What), "        Why: "+termText(ch.Why), "        Need: "+termText(ch.Need))
		}
	}
	return lines
}

// featureCard is Feature f's card: its parts, each the Baseline card it opens
// as, lines that say each part, and build, the Approve to build card that
// approving the parts leads to (nil when none follows).
func featureCard(f int, parts []InboxItem, lines []string, build *InboxItem) InboxItem {
	it := InboxItem{Kind: InboxFeature, Number: f, Seat: verbs.ProductStrategist, CounselFrom: "facts", Parts: parts, Build: build}
	var names, moved []string
	for _, p := range parts {
		names = append(names, fmt.Sprintf("%s v%d", ready.Name(p.Ask), p.Version))
		if !p.Checklist[0].OK { // the first check: nothing it pins changed since the draft
			moved = append(moved, ready.Label(p.Ask))
		}
	}
	what := joinWords(names)
	it.Lines = append([]string{fmt.Sprintf("Parts    %d %s to approve, each at the hash shown", len(parts), plural(len(parts), "draft", "drafts"))}, lines...)
	canBuild := build != nil && build.Hash != ""
	switch {
	case canBuild:
		it.Lines = append(append(it.Lines, "Then Approve to build:"), build.Lines...)
	case build != nil:
		it.Lines = append(it.Lines, "Then     "+build.Why)
	default:
		it.Lines = append(it.Lines, fmt.Sprintf("Then     Approve to build is not on this card: a part it needs has no draft that holds, or #%d has an approval already.", f))
	}
	if len(moved) > 0 {
		it.Why = fmt.Sprintf("The %s of #%d changed after the draft, so the card approves nothing; an agent drafts it again.", joinWords(moved), f)
		it.f.problem = joinWords(moved) + " changed after the draft"
	} else {
		it.Why = fmt.Sprintf("%s of #%d wait for a person; approving reads every part and the stories again after the device code.", upperFirst(what), f)
		it.f.problem = fmt.Sprintf("%d %s to approve", len(parts), plural(len(parts), "part", "parts"))
	}
	n := 0
	add := func(label, command string) {
		n++
		it.Options = append(it.Options, InboxAction{N: n, Label: label, Command: command})
	}
	switch {
	case len(moved) > 0: // all or nothing: a part that moved holds the whole Feature back
	case canBuild:
		add(fmt.Sprintf("approve and start: %s, then Approve to build, then the mission of #%d", what, f), featureStart)
		add(fmt.Sprintf("approve only: %s, then Approve to build; the mission waits", what), featureApprove)
	default:
		add(fmt.Sprintf("approve %s of #%d", what, f), featureApprove)
	}
	for _, p := range parts {
		add(fmt.Sprintf("open the %s v%d on its own", ready.Name(p.Ask), p.Version), inboxPart+p.Ask)
	}
	add("not now: nothing is written", inboxLeave)
	switch {
	case len(moved) > 0:
		it.Counsel = fmt.Sprintf("What moved: the pins of the %s of #%d no longer match the draft. Approving the Feature is all or nothing, so "+
			"the card offers no approval until an agent drafts it again and says what changed and why. You can still open each other part and "+
			"approve it on its own. Nothing is written until you choose. You decide; I only advise.", joinWords(moved), f)
	case canBuild:
		it.Counsel = fmt.Sprintf("What you agree to: %s of #%d at the hashes on the card, then Approve to build as the card shows it, then the "+
			"mission start, which locks its issues. Every part and story is read again after the one device code; if one moved, nothing is "+
			"written. Open a part to read its pins and checklist first. The start is yours, so I make no recommendation.", what, f)
	default:
		it.Counsel = fmt.Sprintf("What you agree to: %s of #%d at the hashes on the card. Approve to build is not on this card yet, so it "+
			"waits for its own card. Every part is read again after the one device code; if one moved, nothing is written. Open a part to read "+
			"its pins and checklist first. You decide; I only advise.", what, f)
	}
	return it
}

// featureMoved says which part of Feature card was moved in now, the card
// as it is now: a part drafted again, approved, or whose pins moved, or the
// stories its Approve to build approves; "" when none did.
func featureMoved(was, now InboxItem) string {
	f := was.Number
	for _, p := range was.Parts {
		i := slices.IndexFunc(now.Parts, func(q InboxItem) bool { return q.Ask == p.Ask })
		switch {
		case i < 0:
			return fmt.Sprintf("%s v%d of #%d no longer waits: it was approved or drafted again after you read it.", ready.Title(p.Ask), p.Version, f)
		case now.Parts[i].Hash != p.Hash:
			return fmt.Sprintf("%s v%d of #%d was drafted again after you read it.", ready.Title(p.Ask), p.Version, f)
		case !now.Parts[i].Checklist[0].OK:
			return baselineChanged(p.Ask, f)
		}
	}
	if len(now.Parts) != len(was.Parts) {
		return fmt.Sprintf("Another part of #%d was drafted after you read it.", f)
	}
	switch {
	case (was.Build == nil) != (now.Build == nil) || was.Build != nil && was.Build.Hash != now.Build.Hash:
		return approveChanged(f)
	case was.Build != nil && !slices.Equal(was.Build.Lines, now.Build.Lines): // its budget, agents, or milestones
		return fmt.Sprintf("The Approve to build of #%d changed while you read it: its budget, agents, or milestones.", f)
	}
	return ""
}

// approveFeature is Approve and start (start) or Approve only on Feature
// card shown (Bug #1253). The ledger, every part, and the Approve to build
// are read again before the one device code and after it, and the start's
// own refusals that need no write are checked before the code: when one
// fails, nothing is written for the Feature. Then every part's approval,
// pinned to the hash the card showed, goes on the ledger in one commit with
// the Approve to build and its mission plan (approveRun.parts), which
// approve checks once more just before it writes; then the mission starts.
// The last check and the commit are not one transaction with GitHub: a pin
// edited on GitHub between them is drift on the approved parts, as with
// `aayu ready --part`.
func (e *Env) approveFeature(ctx context.Context, repo string, shown InboxItem, start bool) error {
	f, build := shown.Number, shown.Build != nil && shown.Build.Hash != ""
	if err := e.approveRefusal(); err != nil {
		return err
	}
	c, tok, err := e.HumanGitHub(ctx)
	if err != nil {
		return err
	}
	w, err := e.Ledger(ctx, tok, "human", "")
	if err != nil {
		return err
	}
	// read is the ledger tip and the drafts the card shows at it, or the refusal that says what is not the card's.
	read := func() (string, []ready.Baseline, error) {
		t, err := w.Read(ctx)
		if err != nil {
			return "", nil, Wrap(ExitExternal, err)
		}
		defer t.Close()
		cards, err := e.baselineCards(ctx, repo, []int{f})
		if err != nil {
			return "", nil, err
		}
		now := InboxItem{Number: f}
		if i := slices.IndexFunc(cards, func(it InboxItem) bool { return it.Kind == InboxFeature }); i >= 0 {
			now = cards[i]
		}
		if moved := featureMoved(shown, now); moved != "" {
			return "", nil, Errorf(ExitBlocked, "%s Nothing was written for #%d.", moved, f)
		}
		var drafts []ready.Baseline
		for _, p := range shown.Parts {
			_, d, err := (&partRun{e: e, c: c, w: w, repo: repo, n: f, part: p.Ask}).recordsIn(ctx, t)
			if err != nil {
				return "", nil, err
			}
			if d == nil || d.Hash != p.Hash { // the ledger moved after the card was read again
				return "", nil, Errorf(ExitBlocked, "%s v%d of #%d was drafted again after you read it. Nothing was written for #%d.", ready.Title(p.Ask), p.Version, f, f)
			}
			drafts = append(drafts, *d)
		}
		return t.Tip, drafts, nil
	}
	if _, _, err := read(); err != nil {
		return err
	}
	tool := ""
	if start { // what would stop the start, known now, stops it all before anything is written
		if !build {
			return Errorf(ExitUsage, "inbox: #%d has no Approve to build on its card, so it cannot start; nothing was written", f)
		}
		if tool, err = e.repoTool(ctx, repo); err != nil {
			return Wrap(ExitExternal, err)
		}
		why := e.kanhaCanStart(tool)
		if tool == "" {
			why = fmt.Sprintf("no entry in [hosts] repos names %s, so Start has no tool to record", repo)
		}
		if why != "" {
			return Errorf(ExitBlocked, "The mission of #%d cannot start here: %s. Nothing was written for #%d; Approve only approves it without the start.", f, why, f)
		}
	}
	var effect []string
	for _, p := range shown.Parts {
		effect = append(effect, fmt.Sprintf("Baselines %s v%d of #%d at hash %s; a later edit shows as drift.", ready.Label(p.Ask), p.Version, f, p.Hash[:12]))
	}
	if build {
		effect = append(effect, fmt.Sprintf("Approves #%d to build, as its card shows, in the same ledger commit as its parts.", f))
	}
	if start {
		effect = append(effect, fmt.Sprintf("Then starts the mission of #%d on %s, which locks its issues until the last milestone.", f, tool))
	}
	what := "approve and start"
	if !start {
		what = "approve only"
	}
	p, err := e.presenceCheck(ctx, Pending{What: what, Effect: effect}, AccessWrite, e.personOnly(what))
	if err != nil {
		return err
	}
	// A part, its pins, or the Approve to build can change while the code waits: read it all again.
	tip, drafts, err := read()
	if err != nil {
		return err
	}
	files, approved := map[string][]byte{}, map[string]*ready.Baseline{}
	for i := range drafts {
		b := &drafts[i] // what the person saw: the hash covers the pins, changes, and fixes
		d := *b
		approved[b.Part] = &d
		b.ApprovedBy, b.ApprovedAt, b.Presence = p.Login, e.Now().UTC().Format(time.RFC3339), p.Via
		data, _ := json.MarshalIndent(b, "", "  ")
		files[ready.BaselinePath(f, b.Part, b.Version, false)] = append(data, '\n')
	}
	switch {
	case build: // the parts, Approve to build, and its plan in one commit, after one more check of every pin and story
		cat, err := e.grillCatalog()
		if err != nil {
			return err
		}
		r := &ready.Ready{Repo: repo, TechnicalOwner: e.technicalOwner(), GH: c, Concrete: func(it workitems.Item) []workitems.Problem { return grill.Check(cat, it) }}
		a := &approveRun{e: e, c: c, w: w, r: r, n: f, shown: shown.Build.Hash, drafts: approved, parts: files, check: func(plan MissionPlan) error {
			if line := agentsLine(plan); !slices.Contains(shown.Build.Lines, line) {
				return Errorf(ExitBlocked, "The Approve to build of #%d changed while you read it: %s. Nothing was written for #%d.", f, strings.TrimSpace(strings.TrimPrefix(line, "Agents")), f)
			}
			for _, m := range plan.Missions {
				if start && m.Estimate.over() {
					return Errorf(ExitBlocked, "The mission of #%d cannot start here: mission %d's estimate is %s working hours, %s, so it starts only from its Mission start card with a reason. Nothing was written for #%d; Approve only approves it without the start.",
						f, m.Number, hrs(m.Estimate.Hours), m.Estimate.Mark, f)
				}
			}
			return nil
		}}
		if err := a.approve(ctx); err != nil {
			return err
		}
	case !c.DryRun(): // no Approve to build follows: the parts alone, in one commit on the tip read after the code
		_, err := w.Write(ctx, ledger.Commit{Message: fmt.Sprintf("specs: %d baselines of #%d, approved by @%s", len(drafts), f, p.Login), Files: files, Base: tip})
		if errors.Is(err, ledger.ErrMoved) {
			return Errorf(ExitBlocked, "The ledger changed while aayu wrote the parts of #%d. Nothing was written for #%d. Run it again.", f, f)
		}
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		if work, ok := ctx.Value(inboxWorkKey{}).(*inboxWork); ok { // a later step that fails says this landed
			work.record(fmt.Sprintf("approved %d %s of #%d on the ledger", len(drafts), plural(len(drafts), "part", "parts"), f))
		}
	}
	for _, b := range drafts {
		r := &partRun{e: e, c: c, w: w, repo: repo, n: f, part: b.Part}
		if err := r.comment(ctx, b, ready.Baselined(b)); err != nil {
			return err
		}
		fmt.Fprintln(e.Out, strings.ReplaceAll(ready.Baselined(b), "`", ""))
	}
	switch {
	case !start:
		return nil
	case c.DryRun(): // the start reads the approval the dry run did not write
		_, err := fmt.Fprintf(e.Out, "  --dry-run: the mission start of #%d is planned, not run\n", f)
		return err
	}
	return handlers["mission"](ctx, e, Values{"action": "start", "feature": strconv.Itoa(f)})
}

// agentsLine is the line an Approve to build card says the agents of plan in (approveItem).
func agentsLine(plan MissionPlan) string {
	return fmt.Sprintf("Agents   up to %d at once (%s)", plan.Agents, plan.AgentsSource)
}

// baselineCard is the card of draft d, whose part's pins now hold (or not),
// after the approved version base (nil for v1). stories are a spec draft's
// stories as they are now, and footprint the Feature's layers.
func baselineCard(d ready.Baseline, base *ready.Baseline, holds bool, stories []workitems.Item, footprint []string) InboxItem {
	f, part, label := d.Feature, d.Part, ready.Label(d.Part)
	it := InboxItem{Kind: InboxBaseline, Number: f, Ask: part, Title: fmt.Sprintf("%s v%d", ready.Title(part), d.Version), Version: d.Version,
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
