// Package fake is an in-memory GitHub for tests of the verbs: the REST
// endpoints M2 uses (users, repositories, labels, issue types, rulesets, git
// refs/trees/commits, issues, comments, sub-issues) with enough fidelity that
// a verb's second run sees what its first run wrote. It records every request.
package fake

import (
	"cmp"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// Label is a repository label.
type Label struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
}

// IssueType is an organization issue type.
type IssueType struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Color       string `json:"color"`
}

// Comment is an issue comment.
type Comment struct {
	ID        int64     `json:"id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at,omitzero"`
	UpdatedAt time.Time `json:"updated_at,omitzero"` // set when the comment is edited (PATCH); zero means never edited
	HTMLURL   string    `json:"html_url,omitempty"`
	User      struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	} `json:"user"`
}

// Issue is an issue with its labels, comments, and sub-issues.
type Issue struct {
	ID     int64  `json:"id"`
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	State  string `json:"state"`
	// StateReason and ClosedBy are set when the issue is closed through the fake.
	StateReason string     `json:"state_reason,omitempty"`
	ClosedBy    *Account   `json:"closed_by,omitempty"`
	Labels      []Label    `json:"labels"`
	Type        *IssueType `json:"type,omitempty"`
	CreatedAt   string     `json:"created_at"`
	// UpdatedAt is advanced by every write to the issue, or to the pull
	// request that shares its number, through the fake; the issue list
	// filters on it with since, as GitHub does.
	UpdatedAt string     `json:"updated_at"`
	URL       string     `json:"html_url"`
	Comments  []*Comment `json:"-"`
	Subs      []int      `json:"-"`
	Parent    int        `json:"-"`
	Assignees []Account  `json:"assignees"`
	// PullRequest is set on the issue that a pull request shares its number with, as GitHub's Issues API does.
	PullRequest *struct {
		URL string `json:"url"`
	} `json:"pull_request,omitempty"`
	User Account `json:"user"` // the author: the viewer
}

// PR is a pull request; it shares its number with an Issue, as on GitHub.
type PR struct {
	Number int     `json:"number"`
	Title  string  `json:"title"`
	Body   string  `json:"body"`
	State  string  `json:"state"`
	Draft  bool    `json:"draft"`
	Merged bool    `json:"merged"`
	URL    string  `json:"html_url"`
	User   Account `json:"user"`
	Head   Branch  `json:"head"`
	Base   Branch  `json:"base"`
	// MergeCommitSHA is set when the PR merges through the fake.
	MergeCommitSHA string    `json:"merge_commit_sha,omitempty"`
	CreatedAt      time.Time `json:"created_at,omitzero"`
	Reviews        []Review  `json:"-"`
	Files          []string  `json:"-"`
	Diff           string    `json:"-"`
}

// Account is a user or bot.
type Account struct {
	Login string `json:"login"`
	Type  string `json:"type"`
}

// Branch is a PR head or base.
type Branch struct {
	Ref string `json:"ref"`
	SHA string `json:"sha"`
}

// Review is a PR review.
type Review struct {
	ID       int64           `json:"id"`
	User     Account         `json:"user"`
	State    string          `json:"state"`
	CommitID string          `json:"commit_id"`
	Body     string          `json:"body,omitempty"`
	Comments []ReviewComment `json:"-"` // the inline comments a posted review carried
}

// ReviewComment is one inline comment of a posted review.
type ReviewComment struct {
	Path string
	Line int
	Body string
}

// CheckRun is a check run on a commit.
type CheckRun struct {
	ID         int64  `json:"id,omitempty"`
	HTMLURL    string `json:"html_url,omitempty"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
}

// WorkflowRun is an Actions run.
type WorkflowRun struct {
	ID         int64  `json:"id"`
	Path       string `json:"path"`
	HeadSHA    string `json:"head_sha"`
	HeadBranch string `json:"head_branch,omitempty"`
	Event      string `json:"event"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	Attempt    int    `json:"run_attempt,omitempty"`
	HTMLURL    string `json:"html_url,omitempty"`
}

// Release is a GitHub Release, keyed by its tag.
type Release struct {
	ID      int64            `json:"id"`
	TagName string           `json:"tag_name"`
	Body    string           `json:"body"`
	HTMLURL string           `json:"html_url"`
	Assets  []map[string]any `json:"assets,omitempty"`
}

// ArtifactZip is a run artifact with its zip bytes.
type ArtifactZip struct {
	ID   int64
	Name string
	Zip  []byte
}

// Request is one recorded request.
type Request struct {
	Method, Path string
	Body         map[string]any
}

// GitHub is the fake. Lock Mu to read or change state from a test.
type GitHub struct {
	Mu        sync.Mutex
	Owner     string // login of the repository owner
	OwnerType string // User or Organization
	Repo      string // name
	Private   bool
	Default   string
	Viewer    string
	// TokenLogins names the account behind a bearer token other than the
	// Viewer's (a bot's installation token): GET /user and the comments it
	// posts answer as that account.
	TokenLogins map[string]Account
	ViewerType  string // GET /user type; default User
	// Permissions is each login's repository permission (GET
	// /repos/{o}/{r}/collaborators/{login}/permission): admin, write,
	// maintain, read, or none. The Viewer defaults to admin, others to read.
	Permissions map[string]string
	Labels      []*Label
	IssueTypes  []*IssueType
	Rulesets    []map[string]any
	Refs        map[string]string           // "heads/ledger" -> sha
	Trees       map[string][]map[string]any // sha -> entries
	Commits     map[string]map[string]any   // sha -> commit
	Issues      map[int]*Issue
	Pulls       map[int]*PR
	CheckRuns   map[string][]CheckRun   // commit SHA -> check runs, newest first
	JobLogs     map[int64]string        // Actions job (check run) ID -> its log, behind a redirect like GitHub's
	RunJobs     map[int64][]CheckRun    // run ID -> its jobs (latest attempt)
	Runs        []WorkflowRun           // Actions workflow runs
	Artifacts   map[int64][]ArtifactZip // run ID -> artifacts
	Releases    map[string]*Release     // tag -> GitHub Release
	Workflows   []string                // workflow file names GET actions/workflows lists
	// OnDispatch, when set, describes the run a workflow_dispatch starts (the
	// fake assigns its ID, path, and event). OnRerun re-runs a run's failed
	// jobs. Both run with Mu held and must not lock it.
	OnDispatch func(workflow, ref string, inputs map[string]any) WorkflowRun
	OnRerun    func(run *WorkflowRun)
	Requests   []Request
	Projects   []map[string]any  // Projects v2: id, title
	Secrets    map[string]string // Actions secret name -> encrypted value
	Variables  map[string]string
	PublicKey  string         // base64 Curve25519 key for Actions secrets
	App        map[string]any // what /app-manifests/{code}/conversions returns
	// Device is GitHub's OAuth device flow (/login/device/code and
	// /login/oauth/access_token, both form-encoded): nil serves no device flow.
	Device *DeviceFlow

	Files map[string]string // repository file path -> content, served by the contents API for any ref
	// RefFiles serves the contents API for one ref (a commit SHA or branch)
	// when a request names it: ref -> path -> content.
	RefFiles map[string]map[string]string
	// Dispatches records workflow_dispatch requests: the workflow file and the request body.
	Dispatches []map[string]any
	// Static answers GET requests the fake does not model (pull requests and
	// their commits, files, and reviews), keyed by path without the query.
	Static map[string]any

	// RulesetsUnavailable answers every rulesets request as GitHub does for a
	// private repository on GitHub Free.
	RulesetsUnavailable bool
	// RequiredChecks are the required status checks the rules for every
	// branch name (GET /rules/branches/{branch}).
	RequiredChecks []string
	// Fail makes the request with this "METHOD /path" fail with a 500 once
	// it has succeeded After times.
	Fail string
	// Hook, when set, runs first on every request with the fake locked: a
	// test changes state between two of aayu's writes.
	Hook      func(method, path string)
	FailAfter int
	seq       int64
	updates   int // writes so far: each advances an issue's UpdatedAt by a second
	failSeen  int
	Server    *httptest.Server
}

// New starts a fake for owner/name and stops it when the test ends.
func New(t testing.TB, repo, ownerType string) *GitHub {
	owner, name, _ := strings.Cut(repo, "/")
	g := &GitHub{Owner: owner, OwnerType: ownerType, Repo: name, Default: "dev", Viewer: owner,
		Refs: map[string]string{}, Trees: map[string][]map[string]any{}, Commits: map[string]map[string]any{}, Issues: map[int]*Issue{}, Pulls: map[int]*PR{}, CheckRuns: map[string][]CheckRun{}, Artifacts: map[int64][]ArtifactZip{}, seq: 1000}
	g.Server = httptest.NewServer(http.HandlerFunc(g.serve))
	t.Cleanup(g.Server.Close)
	return g
}

// URL is the API base URL.
func (g *GitHub) URL() string { return g.Server.URL }

// Writes returns the recorded mutating requests.
func (g *GitHub) Writes() []Request {
	g.Mu.Lock()
	defer g.Mu.Unlock()
	var out []Request
	for _, r := range g.Requests {
		if q, _ := r.Body["query"].(string); r.Method == http.MethodGet || r.Path == "/graphql" && strings.HasPrefix(q, "query") {
			continue
		}
		out = append(out, r)
	}
	return out
}

// Reset forgets recorded requests.
func (g *GitHub) Reset() { g.Mu.Lock(); g.Requests = nil; g.Mu.Unlock() }

// AddIssue adds an issue and returns it.
func (g *GitHub) AddIssue(title, body string, labels ...string) *Issue {
	g.Mu.Lock()
	defer g.Mu.Unlock()
	return g.addIssue(title, body, labels)
}

func (g *GitHub) addIssue(title, body string, labels []string) *Issue {
	g.seq++
	n := len(g.Issues) + 1
	is := &Issue{ID: g.seq, Number: n, Title: title, Body: body, State: "open", CreatedAt: "2026-10-12T09:00:00Z", UpdatedAt: "2026-10-12T09:00:00Z",
		URL: fmt.Sprintf("https://github.com/%s/%s/issues/%d", g.Owner, g.Repo, n), User: Account{Login: g.Viewer, Type: "User"}}
	for _, l := range labels {
		is.Labels = append(is.Labels, g.label(l))
	}
	g.Issues[n] = is
	return is
}

var touched = regexp.MustCompile(`^/repos/[^/]+/[^/]+/(?:issues|pulls)/([0-9]+)(?:/|$)`)

// touch advances the UpdatedAt of the issue a write to path changed.
func (g *GitHub) touch(path string) {
	m := touched.FindStringSubmatch(path)
	if m == nil {
		return
	}
	n, _ := strconv.Atoi(m[1])
	if is := g.Issues[n]; is != nil {
		g.updates++
		is.UpdatedAt = time.Date(2026, 10, 12, 9, 0, g.updates, 0, time.UTC).Format(time.RFC3339)
	}
}

func (g *GitHub) label(name string) Label {
	for _, l := range g.Labels {
		if l.Name == name {
			return *l
		}
	}
	g.seq++
	return Label{ID: g.seq, Name: name}
}

// AddPR adds an open pull request from head (at headSHA) into base, authored by author.
func (g *GitHub) AddPR(title, body, author, head, headSHA, base string) *PR {
	g.Mu.Lock()
	defer g.Mu.Unlock()
	is := g.addIssue(title, body, nil)
	is.PullRequest = &struct {
		URL string `json:"url"`
	}{URL: fmt.Sprintf("https://api.github.com/repos/%s/pulls/%d", g.Repo, is.Number)}
	is.URL = fmt.Sprintf("https://github.com/%s/%s/pull/%d", g.Owner, g.Repo, is.Number)
	pr := &PR{Number: is.Number, Title: title, Body: body, State: "open", URL: is.URL, User: Account{Login: author, Type: "User"},
		Head: Branch{Ref: head, SHA: headSHA}, Base: Branch{Ref: base, SHA: strings.Repeat("b", 40)}}
	g.Pulls[is.Number] = pr
	return pr
}

// LabelNames returns an issue's label names.
func (g *GitHub) LabelNames(n int) []string {
	g.Mu.Lock()
	defer g.Mu.Unlock()
	var out []string
	for _, l := range g.Issues[n].Labels {
		out = append(out, l.Name)
	}
	return out
}

func (g *GitHub) nextSHA(v any) string {
	g.seq++
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(append(b, strconv.FormatInt(g.seq, 10)...))
	return hex.EncodeToString(sum[:20]) // 40 hex characters, like a git object ID
}

var (
	reCheckRuns = regexp.MustCompile(`^/repos/[^/]+/[^/]+/commits/([^/]+)/check-runs$`)
	reWfRuns    = regexp.MustCompile(`^/repos/[^/]+/[^/]+/actions/workflows/([^/]+)/runs$`)
	reRun       = regexp.MustCompile(`^/repos/[^/]+/[^/]+/actions/runs/([0-9]+)(/rerun-failed-jobs)?$`)
	reRelTag    = regexp.MustCompile(`^/repos/[^/]+/[^/]+/releases/tags/(.+)$`)
	reRelease   = regexp.MustCompile(`^/repos/[^/]+/[^/]+/releases/([0-9]+)$`)
	reRunArts   = regexp.MustCompile(`^/repos/[^/]+/[^/]+/actions/runs/([0-9]+)/artifacts$`)
	reJobLogs   = regexp.MustCompile(`^/repos/[^/]+/[^/]+/actions/jobs/([0-9]+)/logs$`)
	reRunJobs   = regexp.MustCompile(`^/repos/[^/]+/[^/]+/actions/runs/([0-9]+)/jobs$`)
	reArtZip    = regexp.MustCompile(`^/repos/[^/]+/[^/]+/actions/artifacts/([0-9]+)/zip$`)
	rePull      = regexp.MustCompile(`^/repos/[^/]+/[^/]+/pulls/([0-9]+)(/reviews|/files|/merge|/comments|/commits)?$`)
	reIssue     = regexp.MustCompile(`^/repos/[^/]+/[^/]+/issues/([0-9]+)$`)
	reLabels    = regexp.MustCompile(`^/repos/[^/]+/[^/]+/issues/([0-9]+)/labels$`)
	reLabel     = regexp.MustCompile(`^/repos/[^/]+/[^/]+/issues/([0-9]+)/labels/(.+)$`)
	reComments  = regexp.MustCompile(`^/repos/[^/]+/[^/]+/issues/([0-9]+)/comments$`)
	reComment   = regexp.MustCompile(`^/repos/[^/]+/[^/]+/issues/comments/([0-9]+)$`)
	rePerm      = regexp.MustCompile(`^/repos/[^/]+/[^/]+/collaborators/([^/]+)/permission$`)
	reSubs      = regexp.MustCompile(`^/repos/[^/]+/[^/]+/issues/([0-9]+)/sub_issues$`)
	reParent    = regexp.MustCompile(`^/repos/[^/]+/[^/]+/issues/([0-9]+)/parent$`)
	reAssignees = regexp.MustCompile(`^/repos/[^/]+/[^/]+/issues/([0-9]+)/assignees$`)
)

func (g *GitHub) serve(w http.ResponseWriter, r *http.Request) {
	g.Mu.Lock()
	defer g.Mu.Unlock()
	if g.Hook != nil {
		g.Hook(r.Method, r.URL.Path)
	}
	if strings.HasPrefix(r.URL.Path, "/login/") {
		g.serveDevice(w, r)
		return
	}
	var body map[string]any
	var list []any
	if r.Body != nil {
		dec := json.NewDecoder(r.Body)
		var raw json.RawMessage
		if dec.Decode(&raw) == nil {
			if json.Unmarshal(raw, &body) != nil {
				_ = json.Unmarshal(raw, &list)
			}
		}
	}
	path, _ := url.PathUnescape(r.URL.EscapedPath())
	g.Requests = append(g.Requests, Request{r.Method, path, body})
	if r.Method != "GET" {
		defer g.touch(path)
	}
	if g.Fail == r.Method+" "+path {
		if g.failSeen >= g.FailAfter {
			g.Fail = ""
			http.Error(w, `{"message":"injected failure"}`, http.StatusInternalServerError)
			return
		}
		g.failSeen++
	}
	repo := "/repos/" + g.Owner + "/" + g.Repo
	reply := func(code int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(v)
	}
	notFound := func() { reply(http.StatusNotFound, map[string]any{"message": "Not Found"}) }
	str := func(k string) string { s, _ := body[k].(string); return s }

	if v, ok := g.Static[path]; ok && r.Method == "GET" {
		reply(200, v)
		return
	}
	switch m, p := r.Method, path; {
	case m == "GET" && p == "/user" && g.Device != nil && r.Header.Get("Authorization") == "Bearer "+DeviceToken:
		typ := g.Device.ApproverType
		if typ == "" {
			typ = "User"
		}
		reply(200, map[string]any{"login": g.Device.Approver, "type": typ})
	case m == "GET" && p == "/user" && g.TokenLogins[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")].Login != "":
		a := g.TokenLogins[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")]
		reply(200, map[string]any{"login": a.Login, "type": a.Type})
	case m == "GET" && p == "/user":
		typ := g.ViewerType
		if typ == "" {
			typ = "User"
		}
		reply(200, map[string]any{"login": g.Viewer, "type": typ})
	case m == "GET" && strings.HasPrefix(p, "/users/"):
		typ := "User"
		if strings.TrimPrefix(p, "/users/") == g.Owner {
			typ = g.OwnerType
		}
		reply(200, map[string]any{"login": strings.TrimPrefix(p, "/users/"), "type": typ})
	case m == "GET" && p == repo:
		vis := "public"
		if g.Private {
			vis = "private"
		}
		reply(200, map[string]any{"name": g.Repo, "full_name": g.Owner + "/" + g.Repo, "private": g.Private, "visibility": vis,
			"default_branch": g.Default, "owner": map[string]any{"login": g.Owner, "type": g.OwnerType}})

	case m == "GET" && p == repo+"/branches":
		names := []string{g.Default}
		for ref := range g.Refs {
			if b, ok := strings.CutPrefix(ref, "heads/"); ok && b != g.Default {
				names = append(names, b)
			}
		}
		slices.Sort(names[1:])
		out := []map[string]any{}
		for _, n := range names {
			out = append(out, map[string]any{"name": n})
		}
		reply(200, out)

	// GraphQL: just Projects v2.
	case m == "POST" && p == "/graphql":
		q, _ := body["query"].(string)
		if strings.Contains(q, "createProjectV2") {
			in, _ := body["variables"].(map[string]any)["input"].(map[string]any)
			g.seq++
			pr := map[string]any{"id": fmt.Sprintf("PVT_%d", g.seq), "title": in["title"], "repository": in["repositoryId"]}
			g.Projects = append(g.Projects, pr)
			reply(200, map[string]any{"data": map[string]any{"createProjectV2": map[string]any{"projectV2": map[string]any{"id": pr["id"]}}}})
			return
		}
		nodes := []any{}
		for _, pr := range g.Projects {
			nodes = append(nodes, map[string]any{"id": pr["id"], "title": pr["title"]})
		}
		reply(200, map[string]any{"data": map[string]any{
			"repositoryOwner": map[string]any{"id": "OWNER_" + g.Owner, "projectsV2": map[string]any{"nodes": nodes}},
			"repository":      map[string]any{"id": "REPO_" + g.Repo},
		}})

	// Apps, Actions secrets and variables.
	case m == "POST" && strings.HasPrefix(p, "/app-manifests/") && strings.HasSuffix(p, "/conversions"):
		if g.App == nil {
			notFound()
			return
		}
		reply(201, g.App)
	case m == "GET" && p == repo+"/actions/secrets/public-key":
		reply(200, map[string]any{"key_id": "k1", "key": g.PublicKey})
	case m == "PUT" && strings.HasPrefix(p, repo+"/actions/secrets/"):
		if g.Secrets == nil {
			g.Secrets = map[string]string{}
		}
		g.Secrets[strings.TrimPrefix(p, repo+"/actions/secrets/")] = str("encrypted_value")
		w.WriteHeader(201)
	case m == "DELETE" && strings.HasPrefix(p, repo+"/actions/secrets/"):
		delete(g.Secrets, strings.TrimPrefix(p, repo+"/actions/secrets/"))
		w.WriteHeader(204)
	case m == "POST" && p == repo+"/actions/variables":
		if g.Variables == nil {
			g.Variables = map[string]string{}
		}
		g.Variables[str("name")] = str("value")
		w.WriteHeader(201)
	case (m == "GET" || m == "PATCH") && strings.HasPrefix(p, repo+"/actions/variables/"):
		name := strings.TrimPrefix(p, repo+"/actions/variables/")
		v, ok := g.Variables[name]
		if !ok {
			notFound()
			return
		}
		if m == "PATCH" {
			v = str("value")
			g.Variables[name] = v
			w.WriteHeader(204)
			return
		}
		reply(200, map[string]any{"name": name, "value": v})
	case m == "DELETE" && strings.HasPrefix(p, repo+"/actions/variables/"):
		delete(g.Variables, strings.TrimPrefix(p, repo+"/actions/variables/"))
		w.WriteHeader(204)
	case m == "GET" && strings.HasPrefix(p, repo+"/rules/branches/"):
		out := []any{}
		if len(g.RequiredChecks) > 0 {
			var cs []any
			for _, c := range g.RequiredChecks {
				cs = append(cs, map[string]any{"context": c})
			}
			out = append(out, map[string]any{"type": "required_status_checks", "ruleset_source_type": "Repository", "ruleset_source": g.Owner + "/" + g.Repo, "ruleset_id": 1,
				"parameters": map[string]any{"required_status_checks": cs, "strict_required_status_checks_policy": false}})
		}
		reply(200, out)
	case m == "PUT" && strings.HasPrefix(p, repo+"/rulesets/"):
		for _, rs := range g.Rulesets {
			if fmt.Sprint(rs["id"]) == strings.TrimPrefix(p, repo+"/rulesets/") {
				for k, v := range body {
					rs[k] = v
				}
				reply(200, rs)
				return
			}
		}
		notFound()

	// Repository contents.
	case m == "GET" && strings.HasPrefix(p, repo+"/contents/"):
		f := strings.TrimPrefix(p, repo+"/contents/")
		content, ok := g.Files[f]
		if files, pinned := g.RefFiles[r.URL.Query().Get("ref")]; pinned {
			content, ok = files[f]
		}
		if !ok {
			// A directory: its direct children, as the contents API lists them.
			files := g.Files
			if pinned, ok := g.RefFiles[r.URL.Query().Get("ref")]; ok {
				files = pinned
			}
			var entries []map[string]any
			prefix := strings.TrimSuffix(f, "/") + "/"
			if f == "" {
				prefix = ""
			}
			for p := range files {
				if rest, ok := strings.CutPrefix(p, prefix); ok && rest != "" && !strings.Contains(rest, "/") {
					entries = append(entries, map[string]any{"type": "file", "path": p, "name": rest})
				}
			}
			if len(entries) == 0 {
				notFound()
				return
			}
			slices.SortFunc(entries, func(a, b map[string]any) int { return strings.Compare(a["path"].(string), b["path"].(string)) })
			reply(200, entries)
			return
		}
		reply(200, map[string]any{"type": "file", "path": f, "encoding": "base64", "content": base64.StdEncoding.EncodeToString([]byte(content))})

	// Workflow dispatch (with return_run_details).
	case m == "POST" && strings.HasPrefix(p, repo+"/actions/workflows/") && strings.HasSuffix(p, "/dispatches"):
		file := strings.TrimSuffix(strings.TrimPrefix(p, repo+"/actions/workflows/"), "/dispatches")
		body["workflow"] = file
		g.Dispatches = append(g.Dispatches, body)
		id := 900 + len(g.Dispatches)
		if g.OnDispatch != nil { // a run the test describes, which GET actions/runs/{id} then serves
			inputs, _ := body["inputs"].(map[string]any)
			run := g.OnDispatch(file, str("ref"), inputs)
			run.ID, run.Path, run.Event = int64(id), ".github/workflows/"+file, "workflow_dispatch"
			run.HTMLURL = fmt.Sprintf("https://github.com/%s/%s/actions/runs/%d", g.Owner, g.Repo, id)
			g.Runs = append(g.Runs, run)
		}
		reply(200, map[string]any{"workflow_run_id": id, "run_url": fmt.Sprintf("%s%s/actions/runs/%d", g.Server.URL, repo, id),
			"html_url": fmt.Sprintf("https://github.com/%s/%s/actions/runs/%d", g.Owner, g.Repo, id)})

	// Labels.
	case m == "GET" && p == repo+"/labels":
		out := []Label{}
		for _, l := range g.Labels {
			out = append(out, *l)
		}
		reply(200, out)
	case m == "POST" && p == repo+"/labels":
		if slices.ContainsFunc(g.Labels, func(l *Label) bool { return strings.EqualFold(l.Name, str("name")) }) {
			reply(422, map[string]any{"message": "Validation Failed", "errors": []any{map[string]any{"code": "already_exists"}}})
			return
		}
		g.seq++
		l := &Label{ID: g.seq, Name: str("name"), Color: str("color"), Description: str("description")}
		g.Labels = append(g.Labels, l)
		reply(201, l)
	case m == "DELETE" && strings.HasPrefix(p, repo+"/labels/"):
		name := strings.TrimPrefix(p, repo+"/labels/")
		n := len(g.Labels)
		g.Labels = slices.DeleteFunc(g.Labels, func(l *Label) bool { return l.Name == name })
		if n == len(g.Labels) {
			notFound()
			return
		}
		w.WriteHeader(204)

	// Issue types.
	case m == "GET" && p == "/orgs/"+g.Owner+"/issue-types":
		reply(200, g.IssueTypes)
	case m == "POST" && p == "/orgs/"+g.Owner+"/issue-types":
		g.seq++
		it := &IssueType{ID: g.seq, Name: str("name"), Description: str("description"), Color: str("color")}
		g.IssueTypes = append(g.IssueTypes, it)
		reply(200, it)
	case m == "DELETE" && strings.HasPrefix(p, "/orgs/"+g.Owner+"/issue-types/"):
		id, _ := strconv.ParseInt(strings.TrimPrefix(p, "/orgs/"+g.Owner+"/issue-types/"), 10, 64)
		g.IssueTypes = slices.DeleteFunc(g.IssueTypes, func(t *IssueType) bool { return t.ID == id })
		w.WriteHeader(204)

	// Rulesets.
	case g.RulesetsUnavailable && strings.HasPrefix(p, repo+"/rulesets"):
		reply(403, map[string]any{"message": "Upgrade to GitHub Pro or make this repository public to enable this feature.", "status": "403"})
	case m == "GET" && p == repo+"/rulesets":
		reply(200, g.Rulesets)
	case m == "GET" && strings.HasPrefix(p, repo+"/rulesets/"):
		for _, rs := range g.Rulesets {
			if fmt.Sprint(rs["id"]) == strings.TrimPrefix(p, repo+"/rulesets/") {
				reply(200, rs)
				return
			}
		}
		notFound()
	case m == "POST" && p == repo+"/rulesets":
		g.seq++
		body["id"] = g.seq
		g.Rulesets = append(g.Rulesets, body)
		reply(201, body)
	case m == "DELETE" && strings.HasPrefix(p, repo+"/rulesets/"):
		id := strings.TrimPrefix(p, repo+"/rulesets/")
		g.Rulesets = slices.DeleteFunc(g.Rulesets, func(rs map[string]any) bool { return fmt.Sprint(rs["id"]) == id })
		w.WriteHeader(204)

	// Git data.
	case m == "GET" && strings.HasPrefix(p, repo+"/git/ref/"):
		ref := strings.TrimPrefix(p, repo+"/git/ref/")
		sha, ok := g.Refs[ref]
		if !ok {
			notFound()
			return
		}
		reply(200, map[string]any{"ref": "refs/" + ref, "object": map[string]any{"sha": sha, "type": "commit"}})
	case m == "POST" && p == repo+"/git/trees":
		entries, _ := body["tree"].([]any)
		var tree []map[string]any
		for _, e := range entries {
			em, _ := e.(map[string]any)
			tree = append(tree, em)
		}
		sha := g.nextSHA(tree)
		g.Trees[sha] = tree
		reply(201, map[string]any{"sha": sha})
	case m == "GET" && strings.HasPrefix(p, repo+"/git/trees/"):
		sha := strings.TrimPrefix(p, repo+"/git/trees/")
		if c, ok := g.Commits[sha]; ok {
			sha, _ = c["tree"].(string)
		}
		tree, ok := g.Trees[sha]
		if !ok {
			notFound()
			return
		}
		reply(200, map[string]any{"sha": sha, "tree": tree})
	case m == "POST" && p == repo+"/git/commits":
		sha := g.nextSHA(body)
		body["sha"] = sha
		g.Commits[sha] = body
		reply(201, commitJSON(body))
	case m == "GET" && strings.HasPrefix(p, repo+"/git/commits/"):
		c, ok := g.Commits[strings.TrimPrefix(p, repo+"/git/commits/")]
		if !ok {
			notFound()
			return
		}
		reply(200, commitJSON(c))
	case m == "POST" && p == repo+"/git/refs":
		ref := strings.TrimPrefix(str("ref"), "refs/")
		if _, ok := g.Refs[ref]; ok {
			reply(422, map[string]any{"message": "Reference already exists"})
			return
		}
		g.Refs[ref] = str("sha")
		reply(201, map[string]any{"ref": "refs/" + ref, "object": map[string]any{"sha": str("sha")}})
	case m == "DELETE" && strings.HasPrefix(p, repo+"/git/refs/"):
		ref := strings.TrimPrefix(p, repo+"/git/refs/")
		if _, ok := g.Refs[ref]; !ok {
			reply(422, map[string]any{"message": "Reference does not exist"})
			return
		}
		delete(g.Refs, ref)
		w.WriteHeader(204)

	// Issues.
	case m == "GET" && p == repo+"/issues":
		out := []*Issue{}
		since := r.URL.Query().Get("since")
		for n := 1; n <= len(g.Issues); n++ {
			want := r.URL.Query().Get("labels")
			if is := g.Issues[n]; is != nil && (r.URL.Query().Get("state") == "all" || is.State == "open") && is.UpdatedAt >= since &&
				(want == "" || slices.ContainsFunc(is.Labels, func(l Label) bool { return l.Name == want })) {
				out = append(out, is)
			}
		}
		reply(200, out)
	case m == "POST" && p == repo+"/issues":
		var labels []string
		ls, _ := body["labels"].([]any) // GitHub creates an issue without labels too
		for _, l := range ls {
			labels = append(labels, l.(string))
		}
		is := g.addIssue(str("title"), str("body"), labels)
		if a := g.TokenLogins[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")]; a.Login != "" {
			is.User = Account{Login: a.Login, Type: a.Type}
		}
		reply(201, is)
	case m == "GET" && reCheckRuns.MatchString(p):
		runs := g.CheckRuns[reCheckRuns.FindStringSubmatch(p)[1]]
		if runs == nil {
			runs = []CheckRun{}
		}
		reply(200, map[string]any{"total_count": len(runs), "check_runs": runs})
	case m == "GET" && reWfRuns.MatchString(p):
		file, q := reWfRuns.FindStringSubmatch(p)[1], r.URL.Query()
		out := []WorkflowRun{}
		for _, run := range g.Runs {
			if strings.HasSuffix(run.Path, "/"+file) && (q.Get("head_sha") == "" || q.Get("head_sha") == run.HeadSHA) && (q.Get("event") == "" || q.Get("event") == run.Event) {
				out = append(out, run)
			}
		}
		slices.SortStableFunc(out, func(a, b WorkflowRun) int { return cmp.Compare(b.ID, a.ID) }) // newest first, as GitHub lists them
		total := len(out)
		if per := atoi(q.Get("per_page")); per > 0 && len(out) > per { // one page, as GitHub serves it
			out = out[:per]
		}
		reply(200, map[string]any{"total_count": total, "workflow_runs": out})
	case m == "GET" && strings.HasPrefix(p, repo+"/actions/workflows/") && !strings.Contains(strings.TrimPrefix(p, repo+"/actions/workflows/"), "/"):
		f := strings.TrimPrefix(p, repo+"/actions/workflows/")
		if !slices.Contains(g.Workflows, f) {
			reply(404, map[string]any{"message": "Not Found"})
			return
		}
		reply(200, map[string]any{"id": slices.Index(g.Workflows, f) + 1, "path": ".github/workflows/" + f, "state": "active"})
	case m == "GET" && p == repo+"/actions/workflows":
		out := []map[string]any{}
		for _, f := range g.Workflows {
			out = append(out, map[string]any{"id": len(out) + 1, "path": ".github/workflows/" + f, "state": "active"})
		}
		reply(200, map[string]any{"total_count": len(out), "workflows": out})
	case reRun.MatchString(p):
		sm := reRun.FindStringSubmatch(p)
		i := slices.IndexFunc(g.Runs, func(r WorkflowRun) bool { return r.ID == int64(atoi(sm[1])) })
		switch {
		case i >= 0 && m == "GET" && sm[2] == "":
			reply(200, g.Runs[i])
		case i >= 0 && m == "POST" && sm[2] != "" && g.OnRerun != nil:
			g.OnRerun(&g.Runs[i])
			w.WriteHeader(http.StatusCreated)
		default:
			notFound()
		}
	case m == "GET" && reRelTag.MatchString(p):
		if rel := g.Releases[reRelTag.FindStringSubmatch(p)[1]]; rel != nil {
			reply(200, rel)
			return
		}
		notFound()
	case m == "PATCH" && reRelease.MatchString(p):
		id := int64(atoi(reRelease.FindStringSubmatch(p)[1]))
		for _, rel := range g.Releases {
			if rel.ID == id {
				if b, ok := body["body"].(string); ok {
					rel.Body = b
				}
				reply(200, rel)
				return
			}
		}
		notFound()
	case m == "GET" && reRunJobs.MatchString(p):
		jobs := g.RunJobs[int64(atoi(reRunJobs.FindStringSubmatch(p)[1]))]
		reply(200, map[string]any{"total_count": len(jobs), "jobs": jobs})
	case m == "GET" && reRunArts.MatchString(p):
		out := []map[string]any{}
		for _, a := range g.Artifacts[int64(atoi(reRunArts.FindStringSubmatch(p)[1]))] {
			out = append(out, map[string]any{"id": a.ID, "name": a.Name, "expired": false})
		}
		reply(200, map[string]any{"total_count": len(out), "artifacts": out})
	case m == "GET" && reJobLogs.MatchString(p):
		id := int64(atoi(reJobLogs.FindStringSubmatch(p)[1]))
		if _, ok := g.JobLogs[id]; !ok {
			notFound()
			return
		}
		w.Header().Set("Location", g.Server.URL+"/job-log/"+reJobLogs.FindStringSubmatch(p)[1])
		w.WriteHeader(http.StatusFound)
	case m == "GET" && strings.HasPrefix(p, "/job-log/"):
		_, _ = w.Write([]byte(g.JobLogs[int64(atoi(strings.TrimPrefix(p, "/job-log/")))]))
	case m == "GET" && reArtZip.MatchString(p):
		w.Header().Set("Location", g.Server.URL+"/blob/"+reArtZip.FindStringSubmatch(p)[1])
		w.WriteHeader(http.StatusFound)
	case m == "GET" && strings.HasPrefix(p, "/blob/"):
		id := int64(atoi(strings.TrimPrefix(p, "/blob/")))
		for _, arts := range g.Artifacts {
			for _, a := range arts {
				if a.ID == id {
					_, _ = w.Write(a.Zip)
					return
				}
			}
		}
		notFound()
	case m == "POST" && p == repo+"/pulls":
		is := g.addIssue(str("title"), str("body"), nil)
		pr := &PR{Number: is.Number, Title: str("title"), Body: str("body"), State: "open", User: Account{Login: g.Viewer, Type: "User"},
			Head: Branch{Ref: str("head")}, Base: Branch{Ref: str("base"), SHA: strings.Repeat("b", 40)}, CreatedAt: time.Now().UTC()}
		g.Pulls[is.Number] = pr
		reply(201, pr)
	case m == "GET" && p == repo+"/pulls":
		out := []*PR{}
		q := r.URL.Query()
		for _, pr := range g.Pulls {
			if st := q.Get("state"); (st == "all" || pr.State == st || (st == "" && pr.State == "open")) && (q.Get("head") == "" || q.Get("head") == g.Owner+":"+pr.Head.Ref) {
				out = append(out, pr)
			}
		}
		slices.SortFunc(out, func(a, b *PR) int { return a.Number - b.Number })
		if q.Get("direction") == "desc" {
			slices.Reverse(out)
		}
		reply(200, out)
	case rePull.MatchString(p):
		sm := rePull.FindStringSubmatch(p)
		pr := g.Pulls[atoi(sm[1])]
		if pr == nil {
			notFound()
			return
		}
		switch {
		case sm[2] == "" && m == "GET":
			if r.Header.Get("Accept") == "application/vnd.github.v3.diff" {
				w.WriteHeader(200)
				_, _ = w.Write([]byte(pr.Diff))
				return
			}
			reply(200, pr)
		case sm[2] == "/reviews" && m == "POST":
			g.seq++
			rv := Review{ID: g.seq, User: Account{Login: g.Viewer, Type: "User"}, State: "COMMENTED", CommitID: str("commit_id"), Body: str("body")}
			if cs, ok := body["comments"].([]any); ok {
				for _, x := range cs {
					if c, ok := x.(map[string]any); ok {
						line, _ := c["line"].(float64)
						rv.Comments = append(rv.Comments, ReviewComment{Path: fmt.Sprint(c["path"]), Line: int(line), Body: fmt.Sprint(c["body"])})
					}
				}
			}
			pr.Reviews = append(pr.Reviews, rv)
			reply(200, rv)
		case sm[2] == "/reviews" && m == "GET":
			out := []Review{}
			out = append(out, pr.Reviews...)
			reply(200, out)
		case sm[2] == "/comments" && m == "GET": // the inline comments of every review, on the review's commit
			out := []map[string]any{}
			for _, rv := range pr.Reviews {
				for i, c := range rv.Comments {
					out = append(out, map[string]any{"id": rv.ID*1000 + int64(i), "user": rv.User, "path": c.Path, "line": c.Line, "body": c.Body,
						"commit_id": rv.CommitID, "original_commit_id": rv.CommitID, "pull_request_review_id": rv.ID})
				}
			}
			reply(200, out)
		case sm[2] == "/commits" && m == "GET": // the head commit alone; Static gives a PR more
			reply(200, []map[string]any{{"sha": pr.Head.SHA, "commit": map[string]any{"message": ""}}})
		case sm[2] == "/files" && m == "GET":
			out := []map[string]any{}
			for _, f := range pr.Files {
				out = append(out, map[string]any{"filename": f, "status": "modified"})
			}
			reply(200, out)
		case sm[2] == "/merge" && m == "PUT":
			if sha, _ := body["sha"].(string); sha != "" && sha != pr.Head.SHA {
				reply(409, map[string]any{"message": "Head branch was modified. Review and try the merge again."})
				return
			}
			pr.Merged, pr.State = true, "closed"
			pr.MergeCommitSHA = g.nextSHA(body)
			if is := g.Issues[pr.Number]; is != nil {
				is.State = "closed"
			}
			reply(200, map[string]any{"merged": true, "sha": pr.MergeCommitSHA, "message": "Pull Request successfully merged"})
		default:
			notFound()
		}
	case reIssue.MatchString(p):
		is := g.Issues[atoi(reIssue.FindStringSubmatch(p)[1])]
		if is == nil {
			notFound()
			return
		}
		if m == "PATCH" {
			if v, ok := body["body"].(string); ok {
				is.Body = v
			}
			if v, ok := body["title"].(string); ok {
				is.Title = v
			}
			if v, ok := body["state"].(string); ok {
				is.State = v
				is.StateReason, _ = body["state_reason"].(string)
				is.ClosedBy = nil
				if v == "closed" {
					is.ClosedBy = &Account{Login: g.Viewer, Type: "User"}
				}
			}
		}
		reply(200, is)
	case reLabels.MatchString(p):
		is := g.Issues[atoi(reLabels.FindStringSubmatch(p)[1])]
		if is == nil {
			notFound()
			return
		}
		var names []any
		if body != nil {
			names, _ = body["labels"].([]any)
		} else {
			names = list
		}
		if m == "PUT" {
			is.Labels = nil
		}
		for _, n := range names {
			if !slices.ContainsFunc(is.Labels, func(l Label) bool { return l.Name == n }) {
				is.Labels = append(is.Labels, g.label(n.(string)))
			}
		}
		reply(200, is.Labels)
	case reAssignees.MatchString(p) && (m == "POST" || m == "DELETE"):
		is := g.Issues[atoi(reAssignees.FindStringSubmatch(p)[1])]
		if is == nil {
			notFound()
			return
		}
		names, _ := body["assignees"].([]any)
		for _, n := range names {
			is.Assignees = slices.DeleteFunc(is.Assignees, func(a Account) bool { return a.Login == n })
			if m == "POST" {
				is.Assignees = append(is.Assignees, Account{Login: n.(string), Type: "User"})
			}
		}
		reply(201, is)
	case reLabel.MatchString(p) && m == "DELETE":
		sm := reLabel.FindStringSubmatch(p)
		is := g.Issues[atoi(sm[1])]
		n := len(is.Labels)
		is.Labels = slices.DeleteFunc(is.Labels, func(l Label) bool { return l.Name == sm[2] })
		if n == len(is.Labels) {
			notFound()
			return
		}
		reply(200, is.Labels)
	case reComments.MatchString(p):
		is := g.Issues[atoi(reComments.FindStringSubmatch(p)[1])]
		if is == nil {
			notFound()
			return
		}
		if m == "POST" {
			g.seq++
			c := &Comment{ID: g.seq, Body: str("body"), CreatedAt: time.Now().UTC()}
			c.HTMLURL = fmt.Sprintf("https://github.com/%s/%s/issues/%d#issuecomment-%d", g.Owner, g.Repo, is.Number, c.ID)
			c.User.Login, c.User.Type = g.Viewer, "User"
			if a := g.TokenLogins[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")]; a.Login != "" {
				c.User.Login, c.User.Type = a.Login, a.Type
			}
			is.Comments = append(is.Comments, c)
			reply(201, c)
			return
		}
		out := []*Comment{}
		out = append(out, is.Comments...)
		reply(200, out)
	case rePerm.MatchString(p) && m == "GET":
		login := rePerm.FindStringSubmatch(p)[1]
		perm := g.Permissions[login]
		switch {
		case perm == "" && login == g.Viewer:
			perm = "admin"
		case perm == "":
			perm = "read"
		}
		role := perm
		if perm == "maintain" {
			perm = "write" // GitHub reports maintain as write, with role_name maintain
		}
		reply(200, map[string]any{"permission": perm, "role_name": role, "user": map[string]any{"login": login}})
	case reComment.MatchString(p) && (m == "PATCH" || m == "GET"):
		id, _ := strconv.ParseInt(reComment.FindStringSubmatch(p)[1], 10, 64)
		for _, is := range g.Issues {
			for _, c := range is.Comments {
				if c.ID == id {
					if m == "PATCH" {
						c.Body, c.UpdatedAt = str("body"), time.Now().UTC().Add(time.Second)
					}
					reply(200, c)
					return
				}
			}
		}
		notFound()
	case reSubs.MatchString(p):
		is := g.Issues[atoi(reSubs.FindStringSubmatch(p)[1])]
		if is == nil {
			notFound()
			return
		}
		if m == "POST" {
			id := int64(body["sub_issue_id"].(float64))
			for _, c := range g.Issues {
				if c.ID == id {
					is.Subs = append(is.Subs, c.Number)
					c.Parent = is.Number
					reply(201, c)
					return
				}
			}
			notFound()
			return
		}
		out := []*Issue{}
		for _, n := range is.Subs {
			out = append(out, g.Issues[n])
		}
		page, per := atoi(r.URL.Query().Get("page")), atoi(r.URL.Query().Get("per_page"))
		if per > 0 {
			page = max(page, 1)
			lo, hi := min((page-1)*per, len(out)), min(page*per, len(out))
			if hi < len(out) {
				w.Header().Set("Link", fmt.Sprintf(`<%s%s?page=%d&per_page=%d>; rel="next"`, g.Server.URL, p, page+1, per))
			}
			out = out[lo:hi]
		}
		reply(200, out)
	case reParent.MatchString(p):
		is := g.Issues[atoi(reParent.FindStringSubmatch(p)[1])]
		if is == nil || is.Parent == 0 {
			notFound()
			return
		}
		reply(200, g.Issues[is.Parent])
	default:
		reply(http.StatusNotFound, map[string]any{"message": "fake: no route for " + r.Method + " " + path})
	}
}

// commitJSON renders a stored commit (whose "tree" is a SHA) as GitHub does.
func commitJSON(c map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range c {
		out[k] = v
	}
	out["tree"] = map[string]any{"sha": c["tree"]}
	return out
}

func atoi(s string) int { n, _ := strconv.Atoi(s); return n }

// DeviceToken is the access token the fake device flow issues on approval.
const DeviceToken = "device-flow-token"

// DeviceFlow scripts GitHub's OAuth device flow.
type DeviceFlow struct {
	ClientID     string // the client ID it accepts
	Approver     string // GET /user login for DeviceToken
	ApproverType string // GET /user type for DeviceToken; default User
	Pending      int    // polls answered authorization_pending before Outcome
	Outcome      string // approve, deny, or "" (pending until the code expires)
	ExpiresIn    int    // seconds; default 900
	Started      int    // device codes issued
	Polls        int    // token polls answered
	// OnPoll, when set, runs at each token poll with the fake locked: a test
	// changes state while a person is entering the code.
	OnPoll func(poll int)
}

func (g *GitHub) serveDevice(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	g.Requests = append(g.Requests, Request{r.Method, r.URL.Path, nil})
	reply := func(v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}
	d := g.Device
	switch {
	case d == nil || r.Method != "POST":
		http.NotFound(w, r)
	case r.Form.Get("client_id") != d.ClientID:
		reply(map[string]any{"error": "incorrect_client_credentials"})
	case r.URL.Path == "/login/device/code":
		d.Started++
		exp := d.ExpiresIn
		if exp == 0 {
			exp = 900
		}
		reply(map[string]any{"device_code": "dc-1", "user_code": "WDJB-MJHT", "verification_uri": "https://github.com/login/device", "expires_in": exp, "interval": 5})
	case r.URL.Path == "/login/oauth/access_token" && r.Form.Get("device_code") == "dc-1" && r.Form.Get("grant_type") == "urn:ietf:params:oauth:grant-type:device_code":
		d.Polls++
		if d.OnPoll != nil {
			d.OnPoll(d.Polls)
		}
		switch {
		case d.Polls <= d.Pending || d.Outcome == "":
			reply(map[string]any{"error": "authorization_pending"})
		case d.Outcome == "approve":
			reply(map[string]any{"access_token": DeviceToken, "token_type": "bearer", "scope": ""})
		default:
			reply(map[string]any{"error": "access_denied"})
		}
	default:
		reply(map[string]any{"error": "bad_verification_code"})
	}
}
