package integrations

import (
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
)

type sent struct{ n int }

func (s *sent) RoundTrip(req *http.Request) (*http.Response, error) {
	s.n++
	return &http.Response{StatusCode: 200, Body: http.NoBody, Request: req}, nil
}

// TestReadOnlyTransport: adapters can read and can POST only to the query
// endpoints they declare; any other request is refused before it is sent.
func TestReadOnlyTransport(t *testing.T) {
	query := Allow{Method: http.MethodPost, Path: regexp.MustCompile(`^/api/projects/1/query/$`)}
	for _, tt := range []struct {
		method, url string
		ok          bool
	}{
		{"GET", "https://x.test/api/projects/1/events/", true},
		{"HEAD", "https://x.test/", true},
		{"POST", "https://x.test/api/projects/1/query/", true},
		{"POST", "https://x.test/api/projects/2/query/", false},
		{"POST", "https://x.test/api/projects/1/query/upgrade/", false},
		{"PATCH", "https://x.test/api/projects/1/query/", false},
		{"DELETE", "https://x.test/api/projects/1/query/abc/", false},
	} {
		s := &sent{}
		req, _ := http.NewRequest(tt.method, tt.url, nil)
		resp, err := ReadOnly(s, query).RoundTrip(req)
		if resp != nil {
			_ = resp.Body.Close()
		}
		if tt.ok != (err == nil) || tt.ok != (s.n == 1) {
			t.Errorf("%s %s: err %v, sent %d; want allowed = %v", tt.method, tt.url, err, s.n, tt.ok)
		}
		if !tt.ok && !errors.Is(err, actions.ErrCapability) {
			t.Errorf("%s %s: err %v, want actions.ErrCapability", tt.method, tt.url, err)
		}
	}
}

// TestDoJSONErrorRedactsCredentials: a provider error that echoes a
// credential does not carry it into the error message.
func TestDoJSONErrorRedactsCredentials(t *testing.T) {
	const key = "ghp_abcdefghijklmnopqrstuvwxyz0123456789"
	hc := &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 401, Status: "401 Unauthorized", Header: http.Header{}, Request: req,
			Body: io.NopCloser(strings.NewReader(`{"detail":"bad key ` + key + `"}`))}, nil
	})}
	req, _ := http.NewRequest(http.MethodGet, "https://x.test/api/0/", nil)
	err := DoJSON(hc, req, &struct{}{})
	if !errors.Is(err, ErrProvider) || !strings.Contains(err.Error(), "401") {
		t.Fatalf("err = %v, want ErrProvider with the status", err)
	}
	if strings.Contains(err.Error(), key) {
		t.Fatalf("error message keeps the credential: %v", err)
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
