package contracts

import (
	"bytes"
	"errors"
	"testing"
)

// The committed ask.schema.json is exactly the schema generated from the Go
// contract types, so a type change without a schema change fails here.
func TestSchemaInSync(t *testing.T) {
	if got := Schema(); !bytes.Equal(got, Committed()) {
		t.Errorf("ask.schema.json is stale; regenerate it from Schema():\n%s", got)
	}
	for _, want := range []string{`"enum": [`, `"answered"`, `"maxLength": 500`, `"required": [`} {
		if !bytes.Contains(Committed(), []byte(want)) {
			t.Errorf("schema lacks %s", want)
		}
	}
}

// Check: answered responses quote and cite; referred and refused ones explain and cite nothing.
func TestCheck(t *testing.T) {
	cite := []Citation{{PassageID: "p", Plan: "P", Section: "S"}}
	for name, tc := range map[string]struct {
		r  AskResponse
		ok bool
	}{
		"answered":            {AskResponse{Status: StatusAnswered, Answer: "x", Citations: cite}, true},
		"answered, no cite":   {AskResponse{Status: StatusAnswered, Answer: "x"}, false},
		"answered, no text":   {AskResponse{Status: StatusAnswered, Citations: cite}, false},
		"referred":            {AskResponse{Status: StatusReferred, Reason: "r", Message: "m"}, true},
		"refused with a cite": {AskResponse{Status: StatusRefused, Reason: "r", Message: "m", Citations: cite}, false},
		"refused, no reason":  {AskResponse{Status: StatusRefused, Message: "m"}, false},
		"unknown":             {AskResponse{Status: "maybe"}, false},
	} {
		if err := tc.r.Check(); (err == nil) != tc.ok || (err != nil && !errors.Is(err, ErrContract)) {
			t.Errorf("%s: %v", name, err)
		}
	}
}
