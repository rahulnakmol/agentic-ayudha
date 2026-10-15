// Package api is the benefits-qa example's HTTP boundary (layer L02,
// docs/BUILD-PLAN.md WP8.1): POST /api/ask takes a contracts.AskRequest and
// answers a contracts.AskResponse; GET /healthz reports readiness. Requests
// outside the contract (ask.schema.json) are refused with a 4xx.
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/ask"
	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/contracts"
	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/policy"
)

// MaxBody bounds a request body, in bytes.
const MaxBody = 4 << 10

var userRE = regexp.MustCompile(`^[A-Za-z0-9-]{0,64}$`)

// Register adds the API routes to mux.
func Register(mux *http.ServeMux, svc *ask.Service) {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("GET /api/plans", func(w http.ResponseWriter, _ *http.Request) {
		write(w, http.StatusOK, []string{"Dental", "Medical", "Retirement", "Leave"})
	})
	mux.HandleFunc("/api/ask", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			fail(w, http.StatusMethodNotAllowed, "use POST with a JSON body")
			return
		}
		if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
			fail(w, http.StatusUnsupportedMediaType, "send Content-Type: application/json")
			return
		}
		req, err := decodeRequest(http.MaxBytesReader(w, r.Body, MaxBody))
		if err != nil {
			var tooBig *http.MaxBytesError
			if errors.As(err, &tooBig) {
				fail(w, http.StatusRequestEntityTooLarge, "the body is larger than 4 KiB")
				return
			}
			fail(w, http.StatusBadRequest, "the body is not an AskRequest: "+err.Error())
			return
		}
		if strings.TrimSpace(req.Question) == "" {
			fail(w, http.StatusBadRequest, "question is required")
			return
		}
		if utf8.RuneCountInString(req.Question) > policy.MaxQuestion {
			fail(w, http.StatusBadRequest, "question is longer than 500 characters")
			return
		}
		if !userRE.MatchString(req.UserID) {
			fail(w, http.StatusBadRequest, "user_id must be a pseudonymous ID of letters, digits, and hyphens")
			return
		}
		resp, _ := svc.Ask(req.UserID, req.Question) // a recorder failure never fails the answer
		write(w, http.StatusOK, resp)
	})
}

// decodeRequest reads exactly one JSON object whose keys are exactly the
// contract's ("question", "user_id"; case-sensitive, each at most once) with
// string values (null is not a string), and nothing after it. encoding/json
// alone would match keys case-insensitively, keep the last of duplicate keys,
// and read null as an empty string.
func decodeRequest(body io.Reader) (contracts.AskRequest, error) {
	var req contracts.AskRequest
	dec := json.NewDecoder(body)
	if t, err := dec.Token(); err != nil {
		return req, err
	} else if t != json.Delim('{') {
		return req, errors.New("want a JSON object")
	}
	seen := map[string]bool{}
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return req, err
		}
		key, _ := t.(string)
		if key != "question" && key != "user_id" {
			return req, fmt.Errorf("unknown field %q", key)
		}
		if seen[key] {
			return req, fmt.Errorf("duplicate field %q", key)
		}
		seen[key] = true
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return req, err
		}
		var v string
		if len(raw) == 0 || raw[0] != '"' || json.Unmarshal(raw, &v) != nil {
			return req, fmt.Errorf("field %q: want a string", key)
		}
		if key == "question" {
			req.Question = v
		} else {
			req.UserID = v
		}
	}
	if _, err := dec.Token(); err != nil { // the closing brace
		return req, err
	}
	// Exactly one JSON value: anything after it (another value, a stray bracket, text) is refused.
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return req, errors.New("something follows the JSON object")
	}
	return req, nil
}

func fail(w http.ResponseWriter, code int, msg string) {
	write(w, code, contracts.Error{Error: msg})
}

func write(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
