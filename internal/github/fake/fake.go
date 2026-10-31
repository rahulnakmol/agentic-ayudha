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
	"maps"
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

// Milestone is a repository milestone.
type Milestone struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Description string `json:"description"`
	State       string `json:"state"`
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
	Milestone   *Milestone `json:"milestone"`
	Type        *IssueType `json:"type,omitempty"`
	CreatedAt   string     `json:"created_at"`
	// UpdatedAt is advanced by every write to the issue, or to the pull
	// request that shares its number, through the fake; the issue list
	// filters on it with since, as GitHub does.
	UpdatedAt string     `json:"updated_at"`
	ClosedAt  string     `json:"closed_at,omitempty"` // set by a test that needs when it closed
	URL       string     `json:"html_url"`
	Comments  []*Comment `json:"-"`
	Subs      []int      `json:"-"`
	Parent    int        `json:"-"`
	ParentURL string     `json:"parent_issue_url,omitempty"` // set from Parent when the issue list replies
	Assignees []Account  `json:"assignees"`
	// PullRequest is set on the issue that a pull request shares its number with, as GitHub's Issues API does.
	PullRequest *struct {
		URL string `json:"url"`
	} `json:"pull_request,omitempty"`
	User Account `json:"user"` // the author: the viewer
	// EditedBy and EditedAt are who last edited its body, and when: GitHub
	// serves them only through GraphQL (editor, lastEditedAt).
	EditedBy string    `json:"-"`
	EditedAt time.Time `json:"-"`
}

// PR is a pull request; it shares its number with an Issue, as on GitHub.
type PR struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	State  string `json:"state"`
	Draft  bool   `json:"draft"`
	Merged bool   `json:"merged"`
	// MergedAt is set when the PR merges through the fake; the list
	// endpoint carries it, not Merged.
	MergedAt time.Time `json:"merged_at,omitzero"`
	URL      string    `json:"html_url"`
	User     Account   `json:"user"`
	Head     Branch    `json:"head"`
	Base     Branch    `json:"base"`
	// Labels and NodeID are served from the PR's issue and number.
	Labels []Label `json:"labels"`
	NodeID string  `json:"node_id"`
	// Mergeable and MergeableState are what GitHub says of merging it
	// (GET /pulls/{n} only): nil is still being computed.
	Mergeable      *bool  `json:"mergeable"`
	MergeableState string `json:"mergeable_state,omitempty"`
	// Computing is how many more GETs answer mergeable null and state
	// unknown before the two above, as GitHub does while it computes them.
	Computing int `json:"-"`
	// MergeCommitSHA is set when the PR merges through the fake.
	MergeCommitSHA string    `json:"merge_commit_sha,omitempty"`
	CreatedAt      time.Time `json:"created_at,omitzero"`
	Reviews        []Review  `json:"-"`
	Files          []string  `json:"-"`
	// Compared, when set, is what the comparison of its base and head SHAs
	// lists instead of Files (GET /compare/{base}...{head}): the live PR's
	// files can differ from the revisions a caller captured.
	Compared []string `json:"-"`
	// Timeline is its timeline's event names (GET /issues/{n}/timeline), oldest first.
	Timeline []string `json:"-"`
	// ChangedFiles is served as len(Files) plus Unlisted, the files GitHub's
	// file listing leaves out (it lists at most 3,000).
	ChangedFiles int    `json:"changed_files"`
	Unlisted     int    `json:"-"`
	Diff         string `json:"-"`
}

// Account is a user or bot.
type Account struct {
	Login string `json:"login"`
	Type  string `json:"type"`
}

// Branch is a PR head or base.
type Branch struct {
	Ref  string    `json:"ref"`
	SHA  string    `json:"sha"`
	Repo *HeadRepo `json:"repo,omitempty"`
}

// HeadRepo is the repository a PR's head branch lives in.
type HeadRepo struct {
	FullName string `json:"full_name"`
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
	ID          int64  `json:"id,omitempty"`
	HTMLURL     string `json:"html_url,omitempty"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	Conclusion  string `json:"conclusion"`
	StartedAt   string `json:"started_at,omitempty"`   // RFC 3339
	CompletedAt string `json:"completed_at,omitempty"` // RFC 3339
	App         *App   `json:"app,omitempty"`          // the GitHub App that made it
	Suite       *Suite `json:"check_suite,omitempty"`
}

// Suite is a check suite: one workflow run's checks, reruns included.
type Suite struct {
	ID int64 `json:"id"`
}

// Status is a legacy commit status.
type Status struct {
	Context string `json:"context"`
	State   string `json:"state"`
}

// App is the GitHub App of a check run.
type App struct {
	ID int64 `json:"id"`
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
	// CheckSuiteID is the check suite that holds the run's check runs.
	CheckSuiteID int64 `json:"check_suite_id,omitempty"`
	// PullRequests are the pull requests the run is for, with their bases,
	// as GitHub lists them for a same-repository pull request.
	PullRequests []RunPR `json:"pull_requests,omitempty"`
	// HeadRepository is the repository of the run's head branch.
	HeadRepository *HeadRepo `json:"head_repository,omitempty"`
	CreatedAt      time.Time `json:"created_at,omitzero"`
	// RunStartedAt is when the run's latest attempt started.
	RunStartedAt time.Time `json:"run_started_at,omitzero"`
}

// RunPR is a pull request a workflow run is for.
type RunPR struct {
	Number int    `json:"number"`
	Base   Branch `json:"base"`
}

// Release is a GitHub Release, keyed by its tag.
type Release struct {
	ID      int64            `json:"id"`
	TagName string           `json:"tag_name"`
	Body    string           `json:"body"`
	HTMLURL string           `json:"html_url"`
	Assets  []map[string]any `json:"assets,omitempty"`
	// PublishedAt is the publication time, as GitHub's release list reports it.
	PublishedAt string `json:"published_at,omitempty"`
}

// ArtifactZip is a run artifact with its zip bytes.
type ArtifactZip struct {
	ID      int64
	Name    string
	Zip     []byte
	Expired bool      // GitHub no longer keeps it
	Created time.Time // when the run uploaded it (created_at)
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
	// InstallationTokens are GitHub App installation tokens, each with the
	// repositories ("owner/name") its installation was granted: GET /user
	// refuses them, as GitHub does, and GET /installation/repositories
	// lists those repositories.
	InstallationTokens map[string][]string
	ViewerType         string // GET /user type; default User
	// Permissions is each login's repository permission (GET
	// /repos/{o}/{r}/collaborators/{login}/permission): admin, write,
	// maintain, read, or none, or "not-found" for an account that is not a
	// collaborator (GitHub answers 404). The Viewer defaults to admin, others to read.
	Permissions map[string]string
	Labels      []*Label
	IssueTypes  []*IssueType
	Rulesets    []map[string]any
	Refs        map[string]string           // "heads/ledger" -> sha
	Trees       map[string][]map[string]any // sha -> entries
	Commits     map[string]map[string]any   // sha -> commit
	Issues      map[int]*Issue
	Milestones  []*Milestone
	// AuthorOnlyEdits makes a comment edit by anyone but its author a 403, as on GitHub.
	AuthorOnlyEdits bool
	Pulls           map[int]*PR
	CheckRuns       map[string][]CheckRun   // commit SHA -> check runs, newest first, CheckRunsPerPage a page (0: all)
	Statuses        map[string][]Status     // commit SHA -> legacy commit statuses, newest first
	JobLogs         map[int64]string        // Actions job (check run) ID -> its log, behind a redirect like GitHub's
	RunJobs         map[int64][]CheckRun    // run ID -> its jobs (latest attempt)
	Runs            []WorkflowRun           // Actions workflow runs
	Artifacts       map[int64][]ArtifactZip // run ID -> artifacts
	Releases        map[string]*Release     // tag -> GitHub Release
	Workflows       []string                // workflow file names GET actions/workflows lists
	// OnDispatch, when set, describes the run a workflow_dispatch starts (the
	// fake assigns its ID, path, and event). OnRerun re-runs a run's failed
	// jobs. Both run with Mu held and must not lock it.
	OnDispatch func(workflow, ref string, inputs map[string]any) WorkflowRun
	OnRerun    func(run *WorkflowRun)
	// OnRerunJob re-runs one Actions job, by its check run ID (POST
	// actions/jobs/{id}/rerun); nil answers 404. It runs with Mu held.
	OnRerunJob func(job int64)
	Requests   []Request
	Projects   []map[string]any  // Projects v2: id, title
	Secrets    map[string]string // Actions secret name -> encrypted value
	Variables  map[string]string
	// Environments are the repository's deployment environments by name.
	Environments map[string]*Environment
	PublicKey    string         // base64 Curve25519 key for Actions secrets
	App          map[string]any // what /app-manifests/{code}/conversions returns
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
	// branch name (GET /rules/branches/{branch}), one rule each, served a page
	// of RulesPerPage at a time (0: all). "name@id" pins a check to the app
	// (integration_id) it must come from.
	RequiredChecks []string
	ClassicChecks  []string // classic branch protection's required checks (GET /branches/{b}), "name@id" pinned
	// Strict is the rules' strict_required_status_checks_policy: a PR must
	// be up to date with its base to merge.
	Strict           bool
	RulesPerPage     int
	CheckRunsPerPage int
	RunsPerPage      int // workflow runs a page, whatever per_page asks (0: per_page)
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
		Head: Branch{Ref: head, SHA: headSHA, Repo: &HeadRepo{FullName: g.Owner + "/" + g.Repo}}, Base: Branch{Ref: base, SHA: strings.Repeat("b", 40)}}
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
	reStatuses  = regexp.MustCompile(`^/repos/[^/]+/[^/]+/commits/([^/]+)/statuses$`)
	reWfRuns    = regexp.MustCompile(`^/repos/[^/]+/[^/]+/actions(?:/workflows/([^/]+))?/runs$`) // one workflow's runs, or every workflow's
	reRun       = regexp.MustCompile(`^/repos/[^/]+/[^/]+/actions/runs/([0-9]+)(/rerun-failed-jobs)?$`)
	reRelTag    = regexp.MustCompile(`^/repos/[^/]+/[^/]+/releases/tags/(.+)$`)
	reRelease   = regexp.MustCompile(`^/repos/[^/]+/[^/]+/releases/([0-9]+)$`)
	reRerunJob  = regexp.MustCompile(`^/repos/[^/]+/[^/]+/actions/jobs/([0-9]+)/rerun$`)
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
	installation, isInstallation := g.InstallationTokens[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")]
	switch m, p := r.Method, path; {
	case m == "GET" && p == "/user" && isInstallation:
		reply(403, map[string]any{"message": "Resource not accessible by integration"})
	case m == "GET" && p == "/installation/repositories" && isInstallation:
		repos := []map[string]any{}
		for _, full := range installation {
			repos = append(repos, map[string]any{"full_name": full})
		}
		reply(200, map[string]any{"total_count": len(repos), "repositories": repos})
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

	case m == "GET" && strings.HasPrefix(p, repo+"/issues/") && strings.HasSuffix(p, "/timeline"):
		out := []map[string]any{}
		if pr := g.Pulls[atoi(strings.TrimSuffix(strings.TrimPrefix(p, repo+"/issues/"), "/timeline"))]; pr != nil {
			for _, ev := range pr.Timeline {
				out = append(out, map[string]any{"event": ev})
			}
		}
		reply(200, out)
	case m == "GET" && strings.HasPrefix(p, repo+"/compare/"):
		basehead := strings.TrimPrefix(p, repo+"/compare/")
		for _, pr := range g.Pulls {
			if pr.Base.SHA+"..."+pr.Head.SHA != basehead {
				continue
			}
			list := pr.Files
			if pr.Compared != nil {
				list = pr.Compared
			}
			files := []map[string]any{}
			for _, f := range list {
				files = append(files, map[string]any{"filename": f, "status": "modified"})
			}
			reply(200, map[string]any{"status": "ahead", "files": files})
			return
		}
		reply(404, map[string]any{"message": "Not Found"})
	case m == "GET" && strings.HasPrefix(p, repo+"/branches/") && !strings.Contains(strings.TrimPrefix(p, repo+"/branches/"), "/"):
		b := strings.TrimPrefix(p, repo+"/branches/")
		sha, ok := g.Refs["heads/"+b]
		for _, pr := range g.Pulls {
			if !ok && pr.Base.Ref == b && pr.Base.SHA != "" {
				sha, ok = pr.Base.SHA, true
			}
		}
		if !ok && b != g.Default {
			notFound()
			return
		}
		checks := []map[string]any{}
		for _, c := range g.ClassicChecks {
			name, app, pinned := strings.Cut(c, "@")
			ck := map[string]any{"context": name}
			if pinned {
				ck["app_id"] = atoi(app)
			}
			checks = append(checks, ck)
		}
		reply(200, map[string]any{"name": b, "commit": map[string]any{"sha": sha}, "protected": len(checks) > 0,
			"protection": map[string]any{"enabled": len(checks) > 0, "required_status_checks": map[string]any{"checks": checks}}})
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

	// GraphQL: Projects v2, a revert, and who last edited an issue.
	case m == "POST" && p == "/graphql":
		q, _ := body["query"].(string)
		if strings.Contains(q, "revertPullRequest") {
			in, _ := body["variables"].(map[string]any)["input"].(map[string]any)
			id, _ := in["pullRequestId"].(string)
			orig := g.Pulls[atoi(strings.TrimPrefix(id, "PR_"))]
			if orig == nil || !orig.Merged {
				reply(200, map[string]any{"errors": []any{map[string]any{"message": "pull request not found or not merged"}}})
				return
			}
			title, _ := in["title"].(string)
			text, _ := in["body"].(string)
			is := g.addIssue(title, text, nil)
			pr := &PR{Number: is.Number, Title: title, Body: text, State: "open", User: Account{Login: g.Viewer, Type: "User"},
				Head: Branch{Ref: fmt.Sprintf("revert-%d-%s", orig.Number, orig.Head.Ref)}, Base: orig.Base, CreatedAt: time.Now().UTC()}
			url := fmt.Sprintf("https://github.com/%s/%s/pull/%d", g.Owner, g.Repo, pr.Number)
			pr.URL = url
			g.Pulls[is.Number] = pr
			reply(200, map[string]any{"data": map[string]any{"revertPullRequest": map[string]any{"revertPullRequest": map[string]any{"url": url}}}})
			return
		}
		if strings.Contains(q, "lastEditedAt") {
			vars, _ := body["variables"].(map[string]any)
			n, _ := vars["n"].(float64)
			is := g.Issues[int(n)]
			out := map[string]any{"lastEditedAt": nil, "editor": nil}
			if is != nil && !is.EditedAt.IsZero() {
				out["lastEditedAt"] = is.EditedAt.UTC().Format(time.RFC3339)
			}
			if is != nil && is.EditedBy != "" {
				out["editor"] = map[string]any{"login": is.EditedBy}
			}
			reply(200, map[string]any{"data": map[string]any{"repository": map[string]any{"issue": out}}})
			return
		}
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
		for i, c := range g.RequiredChecks {
			name, app, pinned := strings.Cut(c, "@")
			sc := map[string]any{"context": name}
			if pinned {
				sc["integration_id"] = atoi(app)
			}
			out = append(out, map[string]any{"type": "required_status_checks", "ruleset_source_type": "Repository", "ruleset_source": g.Owner + "/" + g.Repo, "ruleset_id": i + 1,
				"parameters": map[string]any{"required_status_checks": []any{sc}, "strict_required_status_checks_policy": g.Strict}})
		}
		if per := g.RulesPerPage; per > 0 {
			page := max(atoi(r.URL.Query().Get("page")), 1)
			lo, hi := min((page-1)*per, len(out)), min(page*per, len(out))
			if hi < len(out) {
				w.Header().Set("Link", fmt.Sprintf(`<%s%s?page=%d&per_page=%d>; rel="next"`, g.Server.URL, p, page+1, per))
			}
			out = out[lo:hi]
		}
		reply(200, out)
	case g.RulesetsUnavailable && strings.HasPrefix(p, repo+"/environments/"): // GitHub Free has neither for a private repository
		reply(403, map[string]any{"message": "Upgrade to GitHub Pro or make this repository public to enable this feature.", "status": "403"})
	case strings.HasPrefix(p, repo+"/environments/"):
		name, rest, _ := strings.Cut(strings.TrimPrefix(p, repo+"/environments/"), "/")
		if g.Environments == nil {
			g.Environments = map[string]*Environment{}
		}
		env := g.Environments[name]
		switch {
		case m == "PUT" && rest == "":
			if env == nil {
				env = &Environment{Variables: map[string]string{}}
				g.Environments[name] = env
			}
			env.Policy, _ = body["deployment_branch_policy"].(map[string]any)
			reply(200, map[string]any{"name": name})
		case env == nil:
			notFound()
		case m == "GET" && rest == "":
			reply(200, map[string]any{"name": name, "deployment_branch_policy": env.Policy})
		case m == "GET" && rest == "deployment-branch-policies":
			ps := []any{}
			for i, b := range env.Branches {
				if b != "" {
					ps = append(ps, map[string]any{"id": i + 1, "name": b, "type": "branch"})
				}
			}
			// GitHub pages this list: per_page (default 30) from page (default 1).
			per, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
			pg, _ := strconv.Atoi(r.URL.Query().Get("page"))
			per, pg = cmp.Or(per, 30), cmp.Or(pg, 1)
			ps = ps[min((pg-1)*per, len(ps)):min(pg*per, len(ps))]
			reply(200, map[string]any{"total_count": len(ps), "branch_policies": ps})
		case m == "DELETE" && strings.HasPrefix(rest, "deployment-branch-policies/"):
			i, err := strconv.Atoi(strings.TrimPrefix(rest, "deployment-branch-policies/"))
			if err != nil || i < 1 || i > len(env.Branches) || env.Branches[i-1] == "" {
				notFound()
				return
			}
			env.Branches[i-1] = "" // keeps the other IDs stable
			w.WriteHeader(204)
		case m == "POST" && rest == "deployment-branch-policies":
			env.Branches = append(env.Branches, str("name"))
			reply(200, map[string]any{"name": str("name")})
		case m == "POST" && rest == "variables":
			env.Variables[str("name")] = str("value")
			w.WriteHeader(201)
		case m == "GET" && strings.HasPrefix(rest, "variables/"):
			v, ok := env.Variables[strings.TrimPrefix(rest, "variables/")]
			if !ok {
				notFound()
				return
			}
			reply(200, map[string]any{"name": strings.TrimPrefix(rest, "variables/"), "value": v})
		default:
			notFound()
		}
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

	// Milestones.
	case m == "GET" && p == repo+"/milestones":
		out := []*Milestone{}
		for _, ms := range g.Milestones {
			if q := r.URL.Query().Get("state"); q == "all" || ms.State == q || q == "" && ms.State == "open" {
				out = append(out, ms)
			}
		}
		reply(200, out)
	case m == "POST" && p == repo+"/milestones":
		if slices.ContainsFunc(g.Milestones, func(ms *Milestone) bool { return ms.Title == str("title") }) {
			reply(422, map[string]any{"message": "Validation Failed", "errors": []any{map[string]any{"code": "already_exists"}}})
			return
		}
		ms := &Milestone{Number: len(g.Milestones) + 1, Title: str("title"), Description: str("description"), State: "open"}
		g.Milestones = append(g.Milestones, ms)
		reply(201, ms)
	case m == "PATCH" && strings.HasPrefix(p, repo+"/milestones/"):
		n, _ := strconv.Atoi(strings.TrimPrefix(p, repo+"/milestones/"))
		i := slices.IndexFunc(g.Milestones, func(ms *Milestone) bool { return ms.Number == n })
		if i < 0 {
			reply(404, map[string]any{"message": "Not Found"})
			return
		}
		if v, ok := body["description"].(string); ok {
			g.Milestones[i].Description = v
		}
		reply(200, g.Milestones[i])

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
		for _, pr := range g.Pulls { // a branch no test set is a PR's base, at its base SHA
			if !ok && "heads/"+pr.Base.Ref == ref && pr.Base.SHA != "" {
				sha, ok = pr.Base.SHA, true
			}
		}
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
		for _, pr := range g.Pulls { // a PR's head or base commit no test gave a tree has an empty one
			if !ok && sha != "" && (pr.Head.SHA == sha || pr.Base.SHA == sha) {
				tree, ok = []map[string]any{}, true
			}
		}
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
		for _, n := range slices.Sorted(maps.Keys(g.Issues)) { // every issue, also one renumbered for a scenario
			want, ms := r.URL.Query().Get("labels"), r.URL.Query().Get("milestone")
			if is := g.Issues[n]; is != nil && (r.URL.Query().Get("state") == "all" || is.State == "open") && is.UpdatedAt >= since &&
				(want == "" || slices.ContainsFunc(is.Labels, func(l Label) bool { return l.Name == want })) &&
				(ms == "" || is.Milestone != nil && strconv.Itoa(is.Milestone.Number) == ms) {
				if is.Parent != 0 {
					is.ParentURL = fmt.Sprintf("https://api.github.com%s/issues/%d", repo, is.Parent)
				}
				out = append(out, is)
			}
		}
		reply(200, out)
	case m == "GET" && p == repo+"/releases":
		out := []*Release{}
		for _, tag := range slices.Sorted(maps.Keys(g.Releases)) {
			out = append(out, g.Releases[tag])
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
		// GitHub lists only the newest run of each name in each check suite
		// unless filter=all.
		type key struct {
			name  string
			suite int64
		}
		suite := func(c CheckRun) int64 {
			if c.Suite == nil {
				return 0
			}
			return c.Suite.ID
		}
		all, newest := g.CheckRuns[reCheckRuns.FindStringSubmatch(p)[1]], map[key]int64{}
		for _, c := range all {
			k := key{c.Name, suite(c)}
			newest[k] = max(newest[k], c.ID)
		}
		runs := []CheckRun{}
		for _, c := range all {
			if r.URL.Query().Get("filter") == "all" || c.ID == newest[key{c.Name, suite(c)}] {
				runs = append(runs, c)
			}
		}
		total := len(runs)
		if per := g.CheckRunsPerPage; per > 0 {
			page := max(atoi(r.URL.Query().Get("page")), 1)
			lo, hi := min((page-1)*per, total), min(page*per, total)
			if hi < total {
				w.Header().Set("Link", fmt.Sprintf(`<%s%s?page=%d&per_page=%d>; rel="next"`, g.Server.URL, p, page+1, per))
			}
			runs = runs[lo:hi]
		}
		reply(200, map[string]any{"total_count": total, "check_runs": runs})
	case m == "GET" && reStatuses.MatchString(p):
		out := g.Statuses[reStatuses.FindStringSubmatch(p)[1]]
		if out == nil {
			out = []Status{}
		}
		reply(200, out)
	case m == "GET" && reWfRuns.MatchString(p):
		file, q := reWfRuns.FindStringSubmatch(p)[1], r.URL.Query()
		out := []WorkflowRun{}
		for _, run := range g.Runs {
			if (file == "" || strings.HasSuffix(run.Path, "/"+file)) && (q.Get("head_sha") == "" || q.Get("head_sha") == run.HeadSHA) && (q.Get("event") == "" || q.Get("event") == run.Event) {
				out = append(out, run)
			}
		}
		slices.SortStableFunc(out, func(a, b WorkflowRun) int { return cmp.Compare(b.ID, a.ID) }) // newest first, as GitHub lists them
		total, per := len(out), atoi(q.Get("per_page"))
		if g.RunsPerPage > 0 {
			per = g.RunsPerPage
		}
		if per > 0 { // a page at a time, as GitHub serves them
			page := max(atoi(q.Get("page")), 1)
			lo, hi := min((page-1)*per, len(out)), min(page*per, len(out))
			if hi < len(out) {
				q.Set("page", strconv.Itoa(page+1))
				w.Header().Set("Link", fmt.Sprintf(`<%s%s?%s>; rel="next"`, g.Server.URL, p, q.Encode()))
			}
			out = out[lo:hi]
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
	case m == "POST" && reRerunJob.MatchString(p):
		if g.OnRerunJob == nil {
			notFound()
			return
		}
		g.OnRerunJob(int64(atoi(reRerunJob.FindStringSubmatch(p)[1])))
		w.WriteHeader(http.StatusCreated)
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
			art := map[string]any{"id": a.ID, "name": a.Name, "expired": a.Expired}
			if !a.Created.IsZero() {
				art["created_at"] = a.Created.UTC().Format(time.RFC3339)
			}
			out = append(out, art)
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
			Head: Branch{Ref: str("head"), Repo: &HeadRepo{FullName: g.Owner + "/" + g.Repo}}, Base: Branch{Ref: str("base"), SHA: strings.Repeat("b", 40)}, CreatedAt: time.Now().UTC()}
		g.Pulls[is.Number] = pr
		reply(201, pr)
	case m == "GET" && p == repo+"/pulls":
		out := []*PR{}
		q := r.URL.Query()
		for _, pr := range g.Pulls {
			g.servePR(pr)
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
		g.servePR(pr)
		switch {
		case sm[2] == "" && m == "GET":
			if r.Header.Get("Accept") == "application/vnd.github.v3.diff" {
				w.WriteHeader(200)
				_, _ = w.Write([]byte(pr.Diff))
				return
			}
			pr.ChangedFiles = len(pr.Files) + pr.Unlisted
			if pr.Computing > 0 {
				pr.Computing--
				still := *pr
				still.Mergeable, still.MergeableState = nil, "unknown"
				reply(200, &still)
				return
			}
			reply(200, pr)
		case sm[2] == "" && m == "PATCH":
			if b, ok := body["body"].(string); ok {
				pr.Body = b // a pull request is an issue: both reads return the new body
				if is := g.Issues[pr.Number]; is != nil {
					is.Body = b
				}
			}
			if st, ok := body["state"].(string); ok { // open or closed, on the PR and its issue
				pr.State = st
				if is := g.Issues[pr.Number]; is != nil {
					is.State = st
				}
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
			if pr.Mergeable != nil && !*pr.Mergeable {
				reply(405, map[string]any{"message": "Pull Request is not mergeable"})
				return
			}
			pr.Merged, pr.State, pr.MergedAt = true, "closed", time.Now().UTC()
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
			if v, ok := body["milestone"]; ok {
				is.Milestone = nil
				for _, ms := range g.Milestones {
					if f, _ := v.(float64); int(f) == ms.Number {
						is.Milestone = ms
					}
				}
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
		case perm == "not-found":
			reply(404, map[string]any{"message": "Not Found"})
			return
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
	case reComment.MatchString(p) && m == "DELETE":
		id, _ := strconv.ParseInt(reComment.FindStringSubmatch(p)[1], 10, 64)
		for _, is := range g.Issues {
			if i := slices.IndexFunc(is.Comments, func(c *Comment) bool { return c.ID == id }); i >= 0 {
				is.Comments = slices.Delete(is.Comments, i, i+1)
				w.WriteHeader(204)
				return
			}
		}
		notFound()
	case reComment.MatchString(p) && (m == "PATCH" || m == "GET"):
		id, _ := strconv.ParseInt(reComment.FindStringSubmatch(p)[1], 10, 64)
		for _, is := range g.Issues {
			for _, c := range is.Comments {
				if c.ID == id {
					caller := g.Viewer
					if a := g.TokenLogins[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")]; a.Login != "" {
						caller = a.Login // a bot's token edits as the bot
					}
					if m == "PATCH" && g.AuthorOnlyEdits && !strings.EqualFold(c.User.Login, caller) {
						reply(403, map[string]any{"message": "Resource not accessible: only the author edits a comment"})
						return
					}
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

// Environment is a deployment environment: its branch policy (as PUT sent
// it), the branches it admits, and its variables.
type Environment struct {
	Policy    map[string]any
	Branches  []string
	Variables map[string]string
}

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

// servePR fills a PR's labels from its issue and its node ID, as GitHub
// serves them.
func (g *GitHub) servePR(pr *PR) {
	pr.NodeID = fmt.Sprintf("PR_%d", pr.Number)
	pr.Labels = nil
	if is := g.Issues[pr.Number]; is != nil {
		pr.Labels = is.Labels
	}
}
