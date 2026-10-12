package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	gh "github.com/google/go-github/v90/github"
	"github.com/shurcooL/githubv4"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/github/scratch"
)

// interaction is one recorded request and its response.
type interaction struct {
	Method string            `json:"method"`
	Path   string            `json:"path"` // path and query
	Status int               `json:"status"`
	Header map[string]string `json:"header,omitempty"`
	Body   string            `json:"body"`
}

type recording struct {
	Repo         string        `json:"repo"`
	Interactions []interaction `json:"interactions"`
}

var keptHeaders = []string{"Content-Type", "Etag", "Link", "Retry-After"}

// recorder captures every exchange that reaches GitHub.
type recorder struct {
	next http.RoundTripper
	mu   sync.Mutex
	rec  []interaction
}

func (r *recorder) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := r.next.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	h := map[string]string{}
	for _, k := range keptHeaders {
		if v := resp.Header.Get(k); v != "" {
			h[k] = v
		}
	}
	r.mu.Lock()
	r.rec = append(r.rec, interaction{req.Method, req.URL.RequestURI(), resp.StatusCode, h, string(body)})
	r.mu.Unlock()
	return resp, nil
}

// replayer answers requests from a recording, in order, and fails the test on
// any request the recording does not have next.
type replayer struct {
	t   *testing.T
	mu  sync.Mutex
	rec []interaction
	i   int
}

func (r *replayer) RoundTrip(req *http.Request) (*http.Response, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.i >= len(r.rec) {
		r.t.Errorf("unexpected request %s %s after the recording ended", req.Method, req.URL.RequestURI())
		return nil, errors.New("recording exhausted")
	}
	in := r.rec[r.i]
	if in.Method != req.Method || in.Path != req.URL.RequestURI() {
		r.t.Errorf("request %d: got %s %s, recording has %s %s", r.i, req.Method, req.URL.RequestURI(), in.Method, in.Path)
		return nil, errors.New("recording mismatch")
	}
	r.i++
	h := http.Header{}
	for k, v := range in.Header {
		h.Set(k, v)
	}
	return &http.Response{StatusCode: in.Status, Status: http.StatusText(in.Status), Header: h, Body: io.NopCloser(strings.NewReader(in.Body)), Request: req}, nil
}

func newLog(t *testing.T, dry bool) *actions.Log {
	t.Helper()
	l, err := actions.Open(filepath.Join(t.TempDir(), "actions.jsonl"), actions.Options{DryRun: dry})
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// smoke is the WP2.3 contract: the operations M2's verbs use, against a
// repository that starts with GitHub's default labels. The live test runs it
// against a scratch repository; TestRecordedContract replays the recording.
func smoke(t *testing.T, ctx context.Context, c *Client, repo string) {
	t.Helper()
	owner, name, _ := SplitRepo(repo)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}

	// Writes outside actions never leave the process.
	before := c.Stats().Calls()
	_, _, err := c.REST.Issues.CreateLabel(ctx, owner, name, gh.CreateIssueLabelRequest{Name: "unlogged", Color: gh.Ptr("ffffff")})
	if !errors.Is(err, ErrUnlogged) || c.Stats().Calls() != before {
		t.Fatalf("write outside actions: err %v, calls %d -> %d", err, before, c.Stats().Calls())
	}

	for _, l := range []string{"aayu-a", "aayu-b", "aayu-c"} {
		_, err := c.Write(ctx, actions.Action{Kind: "github.label.create", Target: repo, Request: &actions.Request{Method: "POST", Path: "/repos/" + repo + "/labels"}, Payload: map[string]any{"name": l}},
			func(ctx context.Context) (any, error) {
				lb, _, err := c.REST.Issues.CreateLabel(ctx, owner, name, gh.CreateIssueLabelRequest{Name: l, Color: gh.Ptr("0e8a16")})
				return lb.GetID(), err
			})
		must(err)
	}
	labels, err := AllPer(ctx, 5, func(ctx context.Context, o gh.ListOptions) ([]*gh.Label, *gh.Response, error) {
		return c.REST.Issues.ListLabels(ctx, owner, name, &o)
	})
	must(err)
	var names []string
	for _, l := range labels {
		names = append(names, l.GetName())
	}
	if len(labels) <= 5 || !slices.Contains(names, "aayu-a") || !slices.Contains(names, "aayu-c") {
		t.Fatalf("paginated labels (%d): %v", len(labels), names)
	}
	// The same first page again: answered by a 304 from the ETag cache.
	nc := c.Stats().NotChanged
	again, _, err := c.REST.Issues.ListLabels(ctx, owner, name, &gh.ListOptions{PerPage: 5})
	must(err)
	if c.Stats().NotChanged != nc+1 || len(again) != 5 || again[0].GetName() != labels[0].GetName() {
		t.Fatalf("ETag revalidation: not-modified %d -> %d, %d labels", nc, c.Stats().NotChanged, len(again))
	}

	issue := func(title string) *gh.Issue {
		var out *gh.Issue
		_, err := c.Write(ctx, actions.Action{Kind: "github.issue.create", Target: repo, Request: &actions.Request{Method: "POST", Path: "/repos/" + repo + "/issues"}, Payload: map[string]any{"title": title}},
			func(ctx context.Context) (any, error) {
				i, _, err := c.REST.Issues.Create(ctx, owner, name, gh.CreateIssueRequest{Title: title, Body: gh.Ptr("aayudha WP2.3 smoke")})
				out = i
				return i.GetNumber(), err
			})
		must(err)
		return out
	}
	parent, child := issue("Feature: smoke parent"), issue("Story: smoke child")
	_, err = c.Write(ctx, actions.Action{Kind: "github.sub_issue.add", Target: fmt.Sprintf("%s#%d", repo, parent.GetNumber())},
		func(ctx context.Context) (any, error) {
			_, _, err := c.REST.SubIssue.Add(ctx, owner, name, int64(parent.GetNumber()), gh.SubIssueRequest{SubIssueID: child.GetID()})
			return nil, err
		})
	must(err)
	subs, err := All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.SubIssue, *gh.Response, error) {
		return c.REST.SubIssue.ListByIssue(ctx, owner, name, int64(parent.GetNumber()), &o)
	})
	must(err)
	if len(subs) != 1 || (*gh.Issue)(subs[0]).GetNumber() != child.GetNumber() {
		t.Fatalf("sub-issues of #%d: %d", parent.GetNumber(), len(subs))
	}
	p, _, err := c.REST.SubIssue.GetParentIssue(ctx, owner, name, int64(child.GetNumber()))
	must(err)
	if p.GetNumber() != parent.GetNumber() {
		t.Fatalf("parent of #%d is #%d, want #%d", child.GetNumber(), p.GetNumber(), parent.GetNumber())
	}

	_, err = c.Write(ctx, actions.Action{Kind: "github.issue.labels.add", Target: fmt.Sprintf("%s#%d", repo, child.GetNumber())},
		func(ctx context.Context) (any, error) {
			_, _, err := c.REST.Issues.AddLabelsToIssue(ctx, owner, name, child.GetNumber(), []string{"aayu-a", "aayu-b"})
			return nil, err
		})
	must(err)
	_, err = c.Write(ctx, actions.Action{Kind: "github.issue.labels.remove", Target: fmt.Sprintf("%s#%d", repo, child.GetNumber())},
		func(ctx context.Context) (any, error) {
			_, err := c.REST.Issues.RemoveLabelForIssue(ctx, owner, name, child.GetNumber(), "aayu-a")
			return nil, err
		})
	must(err)
	got, _, err := c.REST.Issues.Get(ctx, owner, name, child.GetNumber())
	must(err)
	if len(got.Labels) != 1 || got.Labels[0].GetName() != "aayu-b" {
		t.Fatalf("labels on #%d: %v", child.GetNumber(), got.Labels)
	}

	var q struct {
		Repository struct {
			ProjectsV2 struct{ TotalCount int } `graphql:"projectsV2(first: 1)"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}
	must(c.GraphQL.Query(ctx, &q, map[string]any{"owner": githubv4.String(owner), "name": githubv4.String(name)}))
	if s := c.Stats(); s.GraphQL != 1 || q.Repository.ProjectsV2.TotalCount != 0 {
		t.Fatalf("GraphQL: %d calls, %d projects", s.GraphQL, q.Repository.ProjectsV2.TotalCount)
	}
}

// TestLive runs the smoke contract against a scratch repository it creates and
// deletes (AAYU_LIVE=1). The repository is created and deleted with
// AAYU_LIVE_ADMIN_TOKEN when set (creating and deleting repositories needs a
// token with those rights); everything else uses the human token. With
// AAYU_RECORD=1 it rewrites testdata/smoke.json for TestRecordedContract.
func TestLive(t *testing.T) {
	if os.Getenv("AAYU_LIVE") != "1" {
		t.Skip("set AAYU_LIVE=1 to run against GitHub")
	}
	ctx := context.Background()
	tok := scratch.Token(t)
	admin := os.Getenv("AAYU_LIVE_ADMIN_TOKEN")
	if admin == "" {
		admin = tok
	}
	repo := scratch.Repo(t, admin)
	rec := &recorder{next: http.DefaultTransport}
	c, err := New(Options{Token: tok, Transport: rec, Log: newLog(t, false), Identity: "human"})
	if err != nil {
		t.Fatal(err)
	}
	smoke(t, ctx, c, repo)
	t.Logf("calls: %+v", c.Stats())
	if os.Getenv("AAYU_RECORD") == "1" {
		b, _ := json.MarshalIndent(recording{repo, rec.rec}, "", "  ")
		if err := os.WriteFile(filepath.Join("testdata", "smoke.json"), append(b, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestRecordedContract replays the recorded live smoke run: same requests in
// the same order, and the client reads every response the way the live run did.
func TestRecordedContract(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "smoke.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rec recording
	if err := json.Unmarshal(b, &rec); err != nil {
		t.Fatal(err)
	}
	rp := &replayer{t: t, rec: rec.Interactions}
	c, err := New(Options{Token: "test", BaseURL: "https://api.github.com", Transport: rp, Log: newLog(t, false), WriteGap: -1})
	if err != nil {
		t.Fatal(err)
	}
	smoke(t, context.Background(), c, rec.Repo)
	if rp.i != len(rp.rec) {
		t.Errorf("replayed %d of %d recorded requests", rp.i, len(rp.rec))
	}
}
