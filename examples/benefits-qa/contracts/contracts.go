// Package contracts is the benefits-qa example's API contract (layer L06,
// docs/BUILD-PLAN.md WP8.1): the request and response of POST /api/ask as Go
// types, and ask.schema.json, the JSON Schema generated from them that
// clients integrate against. TestSchemaInSync keeps the two equal.
package contracts

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/invopop/jsonschema"
)

// Statuses of an answer.
const (
	StatusAnswered = "answered" // quoted from a cited plan passage
	StatusReferred = "referred" // not in the plan documents; an HR specialist replies
	StatusRefused  = "refused"  // the example must not answer (another person's records, advice)
)

// AskRequest is the body of POST /api/ask.
type AskRequest struct {
	Question string `json:"question" jsonschema:"minLength=1,maxLength=500" jsonschema_description:"The benefits question, in plain language."`
	UserID   string `json:"user_id,omitempty" jsonschema:"maxLength=64,pattern=^[A-Za-z0-9-]*$" jsonschema_description:"A pseudonymous ID for analytics; never a name or an email address."`
}

// Citation names the plan passage an answer quotes.
type Citation struct {
	PassageID string `json:"passage_id" jsonschema_description:"ID of the plan passage."`
	Plan      string `json:"plan" jsonschema_description:"The plan the passage belongs to."`
	Section   string `json:"section" jsonschema_description:"The section of the plan."`
}

// AskResponse is the body of a 200 answer to POST /api/ask.
type AskResponse struct {
	Status    string     `json:"status" jsonschema:"enum=answered,enum=referred,enum=refused"`
	Answer    string     `json:"answer,omitempty" jsonschema_description:"The quoted plan text; only when answered."`
	Citations []Citation `json:"citations" jsonschema_description:"The passages the answer quotes; empty unless answered."`
	Reason    string     `json:"reason,omitempty" jsonschema_description:"Why the question was referred or refused."`
	Message   string     `json:"message,omitempty" jsonschema_description:"What to tell the asker when the question was referred or refused."`
}

// Error is the body of a 4xx answer.
type Error struct {
	Error string `json:"error"`
}

// Contract is the schema document: both bodies under $defs.
type Contract struct {
	Request  AskRequest  `json:"request"`
	Response AskResponse `json:"response"`
	Error    Error       `json:"error"`
}

//go:embed ask.schema.json
var committed []byte

// Committed is the checked-in ask.schema.json.
func Committed() []byte { return committed }

// Schema generates the JSON Schema of the contract from the Go types.
func Schema() []byte {
	r := &jsonschema.Reflector{RequiredFromJSONSchemaTags: false, ExpandedStruct: true}
	s := r.Reflect(&Contract{})
	s.ID = "https://github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/contracts/ask.schema.json"
	s.Title = "benefits-qa POST /api/ask"
	b, _ := json.MarshalIndent(s, "", "  ")
	return append(b, '\n')
}

// ErrContract is wrapped by every response that breaks the contract.
var ErrContract = errors.New("ask response breaks the contract")

// Check enforces what the schema cannot say: an answered response quotes
// text and cites at least one passage; a referred or refused one has a
// reason and a message and cites nothing.
func (r AskResponse) Check() error {
	switch r.Status {
	case StatusAnswered:
		if r.Answer == "" || len(r.Citations) == 0 || r.Reason != "" {
			return fmt.Errorf("%w: answered needs answer text and a citation, and no reason", ErrContract)
		}
	case StatusReferred, StatusRefused:
		if r.Reason == "" || r.Message == "" || len(r.Citations) != 0 || r.Answer != "" {
			return fmt.Errorf("%w: %s needs a reason and a message, and no answer or citations", ErrContract, r.Status)
		}
	default:
		return fmt.Errorf("%w: unknown status %q", ErrContract, r.Status)
	}
	return nil
}
