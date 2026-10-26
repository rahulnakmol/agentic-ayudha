package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// earlyBuild is the part of #994's early-build approval
// (specs/<f>/approved-early-<s>.json) that Ready reads.
type earlyBuild struct {
	Schema     string `json:"schema"`
	Feature    int    `json:"feature"`
	Story      int    `json:"story"`
	BodyHash   string `json:"body_hash"`
	ApprovedBy string `json:"approved_by"`
}

const earlyBuildSchema = "aayudha.feature.early-build/v1"

// storyCovered is the Story rule of `aayu ready <story>` (Feature #877, Story
// #996, Decision #861): a Story is Ready only under a Feature whose Approve to
// build covers it as it is now, or under a person's early-build approval of
// that story alone while the Feature has no approval. It reads the ledger,
// never a comment (F2). It returns the snapshot's "Approved to build" row, or
// the refusal. Ready's guard and the build pick never run it (F4).
func (e *Env) storyCovered(ctx context.Context, c *github.Client, w *ledger.Writer, repo string, it workitems.Item, body string) (string, error) {
	s := it.Number
	if it.Parent == nil || it.Parent.Type != "feature" {
		return "", Errorf(ExitBlocked, "#%d has no parent Feature. A Story is Ready only under a Feature approved to build (Decision #861).", s)
	}
	f := it.Parent.Number
	a := &approveRun{e: e, c: c, w: w, n: f}
	prev, _, err := a.last(ctx)
	if err != nil {
		return "", err
	}
	if prev == nil {
		return e.earlyCovered(ctx, w, f, s, body)
	}
	if changed, err := a.featureChanges(ctx, repo, *prev); err != nil {
		return "", err
	} else if len(changed) > 0 {
		return "", WithNext(Errorf(ExitBlocked, "#%d changed after Approve to build: %s. Its stories wait until a person approves it again.", f, strings.Join(changed, "; ")),
			fmt.Sprintf("aayu ready %d --check", f))
	}
	// --ack is not built yet: until it is, the next step that runs is a new spec baseline, then Approve to build again.
	i := slices.IndexFunc(prev.Stories, func(st ready.ApprovedStory) bool { return st.Number == s })
	if i < 0 {
		return "", WithNext(Errorf(ExitBlocked, "#%d was added after Approve to build. A person acknowledges it: aayu ready %d --ack %d", s, f, s), fmt.Sprintf("aayu ready %d --part spec --request", f))
	}
	if changes := ready.StoryChanges(prev.Stories[i], ready.StoryOf(it)); len(changes) > 0 {
		return "", WithNext(Errorf(ExitBlocked, "#%d changed after Approve to build (%s). A person acknowledges it: aayu ready %d --ack %d", s, strings.Join(changes, ", "), f, s), fmt.Sprintf("aayu ready %d --part spec --request", f))
	}
	return fmt.Sprintf("#%d approval %d by @%s", f, prev.Version, prev.By), nil
}

// earlyCovered admits story s of Feature f, which has no Approve to build,
// only on a person's early-build approval of s, at the body it approved.
func (e *Env) earlyCovered(ctx context.Context, w *ledger.Writer, f, s int, body string) (string, error) {
	notApproved := WithNext(Errorf(ExitBlocked, "Feature #%d is not approved to build. A person runs aayu ready %d.", f, f), fmt.Sprintf("aayu ready %d", f))
	path := fmt.Sprintf("specs/%d/approved-early-%d.json", f, s) // the path names the story, so a record covers no other
	files, err := (&partRun{e: e, w: w}).ledgerFiles(ctx, path)
	if err != nil {
		return "", err
	}
	if files[path] == nil {
		return "", notApproved
	}
	var eb earlyBuild
	if err := json.Unmarshal(files[path], &eb); err != nil || eb.Schema != earlyBuildSchema || eb.Feature != f || eb.Story != s || eb.ApprovedBy == "" {
		return "", Errorf(ExitExternal, "ledger %s: not an early-build approval of #%d under #%d; #%d is not Ready", path, s, f, s)
	}
	if now := workitems.BodyHash(body); now != eb.BodyHash {
		return "", WithNext(Errorf(ExitBlocked, "#%d changed after its early build was approved (body hash %.12s to %.12s). A person approves it again: aayu ready %d --early-build %d", s, eb.BodyHash, now, f, s),
			fmt.Sprintf("aayu ready %d", f))
	}
	return fmt.Sprintf("#%d early build of #%d by @%s", f, s, eb.ApprovedBy), nil
}

// featureChanges says how the Feature's approved parts changed after the
// approval: a part approved again ("technical design v1 to v2"), or one that
// drifted from its pins ("intent v1 drifted (#840 body changed)"). The spec
// part's drift is left out: a changed or added story is the story's own
// refusal, and needs only an acknowledgment.
func (a *approveRun) featureChanges(ctx context.Context, repo string, prev ready.Approval) ([]string, error) {
	owner, name, _ := github.SplitRepo(repo)
	is, _, err := a.c.REST.Issues.Get(ctx, owner, name, a.n)
	if err != nil {
		return nil, Wrap(ExitExternal, err)
	}
	it, _ := workitems.Parse(a.n, is.GetTitle(), is.GetBody(), is.GetType().GetName(), github.IssueLabels(is))
	version := func(v int) string {
		if v == 0 {
			return "none"
		}
		return fmt.Sprintf("v%d", v)
	}
	var out []string
	for _, part := range ready.Parts {
		if part == "ux" && !slices.Contains(a.e.footprint(it), string(verbs.L01)) { // as Approve to build: a change of footprint is the intent's drift
			continue
		}
		was := ready.ApprovedBaseline{Part: part}
		for _, b := range prev.Baselines {
			if b.Part == part {
				was = b
			}
		}
		pr := &partRun{e: a.e, c: a.c, w: a.w, repo: repo, n: a.n, part: part}
		base, _, _, err := pr.records(ctx)
		if err != nil {
			return nil, err
		}
		now := ready.ApprovedBaseline{Part: part}
		if base != nil {
			now.Version, now.Hash = base.Version, base.Hash
		}
		if now != was {
			out = append(out, fmt.Sprintf("%s %s to %s", ready.Name(part), version(was.Version), version(now.Version)))
		}
		if now != was || base == nil || part == "spec" {
			continue
		}
		pins, err := pr.pins(ctx, it, is.GetBody())
		if CodeOf(err) == ExitBlocked {
			out = append(out, fmt.Sprintf("%s v%d cannot be pinned now", ready.Name(part), base.Version))
			continue
		} else if err != nil {
			return nil, err
		}
		if drift := ready.Diff(base.Pins, pins); len(drift) > 0 {
			ds := make([]string, len(drift))
			for i, d := range drift {
				ds[i] = d.String()
			}
			out = append(out, fmt.Sprintf("%s v%d drifted (%s)", ready.Name(part), base.Version, strings.Join(ds, " · ")))
		}
	}
	return out, nil
}
