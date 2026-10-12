package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/ask"
	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/contracts"
	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/store"
)

func server(t *testing.T) *httptest.Server {
	t.Helper()
	s, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	Register(mux, ask.New(s, nil, nil))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func post(t *testing.T, srv *httptest.Server, ct, body string) (int, string, []byte) {
	t.Helper()
	resp, err := http.Post(srv.URL+"/api/ask", ct, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var b bytes.Buffer
	_, _ = b.ReadFrom(resp.Body)
	return resp.StatusCode, resp.Header.Get("Content-Type"), b.Bytes()
}

func status(t *testing.T, method, url string) (int, http.Header) {
	t.Helper()
	req, _ := http.NewRequest(method, url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	return resp.StatusCode, resp.Header
}

// POST /api/ask answers a plan question with the quoted passage and its
// citation, in a body that decodes strictly as the contract's AskResponse
// and passes its Check.
func TestAskAnswered(t *testing.T) {
	srv := server(t)
	code, ct, body := post(t, srv, "application/json", `{"question":"What is the dental annual deductible?","user_id":"emp-0001"}`)
	var r contracts.AskResponse
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if code != http.StatusOK || ct != "application/json" || dec.Decode(&r) != nil {
		t.Fatalf("%d %s %s", code, ct, body)
	}
	if r.Status != contracts.StatusAnswered || r.Check() != nil || r.Citations[0].PassageID != "dental-deductible" || !strings.Contains(r.Answer, "50 dollars per person") {
		t.Errorf("answer: %+v", r)
	}
	_, _, body = post(t, srv, "application/json", `{"question":"What is my colleague's claim status?"}`)
	if !strings.Contains(string(body), `"status":"refused"`) || !strings.Contains(string(body), `"citations":[]`) {
		t.Errorf("refused: %s", body)
	}
	if code, _ := status(t, http.MethodGet, srv.URL+"/healthz"); code != http.StatusOK {
		t.Errorf("healthz: %d", code)
	}
}

// Malformed, empty, oversized, unknown-field, wrong-type, and non-POST
// requests are rejected with a 4xx and a contracts.Error body.
func TestAskRejects(t *testing.T) {
	srv := server(t)
	for name, tc := range map[string]struct {
		ct, body string
		code     int
	}{
		"malformed":       {"application/json", `{"question":`, http.StatusBadRequest},
		"empty":           {"application/json", `{"question":"  "}`, http.StatusBadRequest},
		"unknown field":   {"application/json", `{"question":"dental","name":"Asha"}`, http.StatusBadRequest},
		"email user id":   {"application/json", `{"question":"dental","user_id":"asha@example.com"}`, http.StatusBadRequest},
		"form":            {"application/x-www-form-urlencoded", `question=dental`, http.StatusUnsupportedMediaType},
		"too large":       {"application/json", `{"question":"` + strings.Repeat("a", MaxBody) + `"}`, http.StatusRequestEntityTooLarge},
		"too long":        {"application/json", `{"question":"` + strings.Repeat("é", 501) + `"}`, http.StatusBadRequest},
		"json look-alike": {"application/jsonx", `{"question":"dental"}`, http.StatusUnsupportedMediaType},
		"second value":    {"application/json", `{"question":"dental"} {"question":"vision"}`, http.StatusBadRequest},
		"stray bracket":   {"application/json", `{"question":"dental"}]`, http.StatusBadRequest},
		"trailing text":   {"application/json", `{"question":"dental"} hello`, http.StatusBadRequest},
		"other case key":  {"application/json", `{"Question":"dental"}`, http.StatusBadRequest},
		"duplicate key":   {"application/json", `{"question":"dental","question":"vision"}`, http.StatusBadRequest},
		"not a string":    {"application/json", `{"question":42}`, http.StatusBadRequest},
		"not an object":   {"application/json", `["dental"]`, http.StatusBadRequest},
		"null question":   {"application/json", `{"question":null}`, http.StatusBadRequest},
		"null user id":    {"application/json", `{"question":"dental","user_id":null}`, http.StatusBadRequest},
		"empty body":      {"application/json", ``, http.StatusBadRequest},
	} {
		code, _, body := post(t, srv, tc.ct, tc.body)
		var e contracts.Error
		if code != tc.code || json.Unmarshal(body, &e) != nil || e.Error == "" {
			t.Errorf("%s: %d %s, want %d", name, code, body, tc.code)
		}
	}
	if code, h := status(t, http.MethodGet, srv.URL+"/api/ask"); code != http.StatusMethodNotAllowed || h.Get("Allow") != "POST" {
		t.Errorf("GET: %d %v", code, h)
	}
}
