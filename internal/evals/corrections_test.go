package evals

import (
	"strings"
	"testing"
)

// correctionsDraft is a small survey draft whose keys are not in alphabetical
// order, with an item (M1) that holds an object with an id of its own, a
// number written as 1.50, and a string a default encoder would escape.
const correctionsDraft = `{"product": "calc", "request": "Rebuild it in Rust.", "summary": "Old <b> summary.",
 "engineering": [{"id": "E1", "claim": "a", "basis": "observed", "cites": ["a.c:1"]}, {"id": "E2", "claim": "b", "basis": "inferred", "reasoning": "r"}, {"id": "E3", "claim": "c", "basis": "observed"}],
 "questions": [{"id": "Q1", "question": "q?", "why": "w", "status": "open", "affects": ["E2"]}],
 "risks": [],
 "modernisation": {"target": "Rust", "strategy": "Old.", "steps": [{"id": "M1", "title": "t", "claims": ["E1"], "sub": [{"id": "X1", "n": 1.50}]}], "cites": ["test:a"]}}`

// keepAll is the verdicts of a reply that keeps every item but the ones given.
func keepAll(but map[string]string) string {
	var vs []string
	for _, id := range []string{"E1", "E2", "E3", "Q1", "M1"} {
		v, ok := but[id]
		if !ok {
			v = `{"id": "` + id + `", "verdict": "keep"}`
		}
		if v != "" {
			vs = append(vs, v)
		}
	}
	return `"verdicts": [` + strings.Join(vs, ", ") + `]`
}

// D-5.9n: the self-check's corrections apply to the draft exactly, keeping
// its order and text, and change only what they name.
func TestApplyCorrections(t *testing.T) {
	reply := "```json\n{" + keepAll(map[string]string{
		"E1": `{"id": "E1", "verdict": "replace", "why": "same", "item": {"id": "E1", "claim": "a", "basis": "observed", "cites": ["a.c:1"]}}`,
		"E2": `{"id": "E2", "verdict": "replace", "why": "narrowed", "item": {"id": "E2", "claim": "b, for Script only", "basis": "observed", "cites": ["s.c:9"]}}`,
		"E3": `{"id": "E3", "verdict": "remove", "why": "nothing supports it"}`,
	}) + `, "add": [{"list": "risks", "why": "was E3", "item": {"id": "R1", "risk": "c may hold"}}],
 "fields": {"summary": "New <b> summary.", "modernisation.strategy": "Old."}}` + "\n```"
	got, changes, problems := applyCorrections(correctionsDraft, reply)
	want := `{"product":"calc","request":"Rebuild it in Rust.","summary":"New <b> summary.",` +
		`"engineering":[{"id":"E1","claim":"a","basis":"observed","cites":["a.c:1"]},{"id":"E2","claim":"b, for Script only","basis":"observed","cites":["s.c:9"]}],` +
		`"questions":[{"id":"Q1","question":"q?","why":"w","status":"open","affects":["E2"]}],` +
		`"risks":[{"id":"R1","risk":"c may hold"}],` +
		`"modernisation":{"target":"Rust","strategy":"Old.","steps":[{"id":"M1","title":"t","claims":["E1"],"sub":[{"id":"X1","n":1.50}]}],"cites":["test:a"]}}`
	if len(problems) > 0 || got != want {
		t.Fatalf("problems %v\ngot  %s\nwant %s", problems, got, want)
	}
	// E1's replacement equals it and the strategy is unchanged: neither counts.
	if strings.Join(changes, ", ") != "E2 replaced, E3 removed, R1 added, summary set" {
		t.Errorf("changes %v", changes)
	}

	backlog := `{"product": "calc", "items": [{"key": "goal", "type": "goal", "title": "G"}, {"key": "s1", "type": "story", "title": "S"}], "open_questions": ["q"]}`
	got, changes, problems = applyCorrections(backlog, `{"verdicts": [{"id": "goal", "verdict": "keep"}, {"id": "s1", "verdict": "replace", "item": {"key": "s1", "type": "story", "title": "S, narrowed"}}],
 "fields": {"open_questions": ["q", "Who runs it?"]}}`)
	if want := `{"product":"calc","items":[{"key":"goal","type":"goal","title":"G"},{"key":"s1","type":"story","title":"S, narrowed"}],"open_questions":["q","Who runs it?"]}`; len(problems) > 0 || got != want ||
		strings.Join(changes, ", ") != "s1 replaced, open_questions set" {
		t.Fatalf("backlog: %v %v\n%s", problems, changes, got)
	}
}

// D-5.9n: a reply that does not apply cleanly is refused whole, with every
// reason, and no draft: a correction is never half-applied or skipped.
func TestApplyCorrectionsRefuses(t *testing.T) {
	good := `{"id": "E2", "verdict": "replace", "item": {"id": "E2", "claim": "narrowed", "basis": "observed"}}`
	for _, tc := range []struct{ name, reply, want string }{
		{"a key the reply format lacks, beside a valid replacement", "{" + keepAll(map[string]string{"E2": good}) + `, "verdicts2": []}`, `unknown field "verdicts2"`},
		{"a verdict for an unknown item", `{"verdicts": [{"id": "E9", "verdict": "keep"}]}`, `verdict for "E9": the draft has no item with that id`},
		{"an item inside another item", "{" + keepAll(map[string]string{"M1": `{"id": "M1", "verdict": "keep"}, {"id": "X1", "verdict": "remove"}`}) + "}", `verdict for "X1": the draft has no item`},
		{"two verdicts for one item", "{" + keepAll(map[string]string{"E1": `{"id": "E1", "verdict": "keep"}, {"id": "E1", "verdict": "remove"}`}) + "}", "item E1 has more than one verdict"},
		{"an item without a verdict", "{" + keepAll(map[string]string{"Q1": "", "E2": good}) + "}", "every item needs a verdict; none for Q1"},
		{"a replacement that changes the id", "{" + keepAll(map[string]string{"E2": `{"id": "E2", "verdict": "replace", "item": {"id": "E7", "claim": "x"}}`}) + "}", `item E2: the replacement must keep id "E2" (it has id "E7")`},
		{"a keep that carries an item", "{" + keepAll(map[string]string{"E2": `{"id": "E2", "verdict": "keep", "item": {"id": "E2"}}`}) + "}", "item E2: a keep verdict carries no item"},
		{"an unknown verdict", "{" + keepAll(map[string]string{"E2": `{"id": "E2", "verdict": "fix"}`}) + "}", `verdict "fix"; want keep, replace, or remove`},
		{"the whole draft instead", correctionsDraft, `unknown field "product"`},
		{"text, not JSON", "E2 should be narrowed.", "not one JSON object"},
		{"an add to a list the draft lacks", "{" + keepAll(nil) + `, "add": [{"list": "dangers", "item": {"id": "R1"}}]}`, `add to "dangers": the draft has no list`},
		{"an add with a used id", "{" + keepAll(nil) + `, "add": [{"list": "risks", "item": {"id": "E1", "risk": "x"}}]}`, "add to risks: id E1 is already used"},
		{"a change to the request", "{" + keepAll(nil) + `, "fields": {"request": "Rebuild it in Go."}}`, "field request is not the self-check's to change"},
		{"a list of items as a field", "{" + keepAll(nil) + `, "fields": {"engineering": []}}`, `field "engineering": the draft has no field without an id`},
		{"a field the draft lacks", "{" + keepAll(nil) + `, "fields": {"modernisation.goal": "x"}}`, `field "modernisation.goal"`},
		{"an item set as a field", "{" + keepAll(nil) + `, "fields": {"summary": {"id": "E9"}}}`, "field summary: a field's value holds no items"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, changes, problems := applyCorrections(correctionsDraft, tc.reply)
			if got != "" || changes != nil || !strings.Contains(strings.Join(problems, "\n"), tc.want) {
				t.Fatalf("got %q %v; problems %v; want a refusal naming %q", got, changes, problems, tc.want)
			}
		})
	}
}
