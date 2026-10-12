// Package fixture replays recorded provider exchanges for the integration
// adapters' tests (WP5.2): a recording is an ordered list of requests and
// their responses, and the replayer fails the test on any request that is not
// the next one recorded, so a fixture pins the exact queries an adapter sends.
package fixture

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
)

// Interaction is one recorded request and its response.
type Interaction struct {
	Method string          `json:"method"`
	URL    string          `json:"url"`            // full URL, query included
	Body   json.RawMessage `json:"body,omitempty"` // request JSON body, compared semantically when set
	// Auth is the request's Authorization header, when the recording pins it
	// (cloud adapters, whose bearer differs per API).
	Auth string `json:"auth,omitempty"`
	// RequestHeader lists request headers that must have these values
	// (an AWS JSON call's X-Amz-Target).
	RequestHeader map[string]string `json:"request_header,omitempty"`
	Status        int               `json:"status"`
	Header        map[string]string `json:"header,omitempty"`
	Response      json.RawMessage   `json:"response,omitempty"`
	// Text is a non-JSON response body (the XML of AWS query APIs); it
	// replaces Response when set.
	Text string `json:"text,omitempty"`
}

// Recording is a fixture file.
type Recording struct {
	// Source says where the exchange came from: a live capture, or the
	// provider documentation it was written from.
	Source       string        `json:"source"`
	Interactions []Interaction `json:"interactions"`
}

// Load reads a recording.
func Load(t testing.TB, path string) *Recording {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var r Recording
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return &r
}

// Replayer answers requests from a recording in order. Auth is the header
// every request must carry (name and value), so a test also proves the key
// travels in a header and never in the URL.
type Replayer struct {
	T          testing.TB
	Rec        *Recording
	AuthHeader string
	AuthValue  string
	// Check, when set, also inspects every request and its body: SigV4
	// verification for the AWS adapter.
	Check func(req *http.Request, body []byte) error

	mu   sync.Mutex
	next int
}

// Requests is the number of requests the replayer has answered.
func (r *Replayer) Requests() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.next
}

// Done fails the test when recorded requests were never made.
func (r *Replayer) Done() {
	r.T.Helper()
	if n := r.Requests(); n != len(r.Rec.Interactions) {
		r.T.Errorf("replayed %d of %d recorded requests", n, len(r.Rec.Interactions))
	}
}

// RoundTrip implements http.RoundTripper.
func (r *Replayer) RoundTrip(req *http.Request) (*http.Response, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.next >= len(r.Rec.Interactions) {
		r.T.Errorf("unrecorded request %s %s", req.Method, req.URL)
		return nil, fmt.Errorf("fixture: unrecorded request %s %s", req.Method, req.URL)
	}
	in := r.Rec.Interactions[r.next]
	r.next++
	if req.Method != in.Method || req.URL.String() != in.URL {
		r.T.Errorf("request %d: got %s %s\nwant %s %s", r.next, req.Method, req.URL, in.Method, in.URL)
	}
	if r.AuthHeader != "" && req.Header.Get(r.AuthHeader) != r.AuthValue {
		r.T.Errorf("request %d: %s header missing or wrong", r.next, r.AuthHeader)
	}
	if r.AuthValue != "" && strings.Contains(req.URL.String(), r.AuthValue) {
		r.T.Errorf("request %d: the key is in the URL", r.next)
	}
	var got []byte
	if req.Body != nil {
		got, _ = io.ReadAll(req.Body)
	}
	if len(in.Body) > 0 && !sameJSON(got, in.Body) {
		r.T.Errorf("request %d body:\ngot  %s\nwant %s", r.next, got, in.Body)
	}
	if in.Auth != "" && req.Header.Get("Authorization") != in.Auth {
		r.T.Errorf("request %d (%s %s): Authorization %q, want %q", r.next, req.Method, req.URL.Host, req.Header.Get("Authorization"), in.Auth)
	}
	for k, v := range in.RequestHeader {
		if req.Header.Get(k) != v {
			r.T.Errorf("request %d: header %s = %q, want %q", r.next, k, req.Header.Get(k), v)
		}
	}
	if r.Check != nil {
		if err := r.Check(req, got); err != nil {
			r.T.Errorf("request %d (%s %s): %v", r.next, req.Method, req.URL, err)
		}
	}
	h := http.Header{"Content-Type": []string{"application/json"}}
	for k, v := range in.Header {
		h.Set(k, v)
	}
	resp := []byte(in.Response)
	if in.Text != "" {
		resp = []byte(in.Text)
	}
	return &http.Response{
		StatusCode: in.Status, Status: fmt.Sprintf("%d %s", in.Status, http.StatusText(in.Status)),
		Header: h, Body: io.NopCloser(bytes.NewReader(resp)), Request: req,
	}, nil
}

func sameJSON(a, b []byte) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	xa, _ := json.Marshal(x)
	ya, _ := json.Marshal(y)
	return bytes.Equal(xa, ya)
}
