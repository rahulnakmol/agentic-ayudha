// Package fake is an in-memory GitHub for tests of the verbs: the REST
// endpoints M2 uses (users, repositories, labels, issue types, rulesets, git
// refs/trees/commits, issues, comments, sub-issues) with enough fidelity that
// a verb's second run sees what its first run wrote. It records every request.
package fake

import (
	"crypto/sha256"
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
	ID   int64  `json:"id"`
	Body string `json:"body"`
	User struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	} `json:"user"`
}

// Issue is an issue with its labels, comments, and sub-issues.
type Issue struct {
	ID        int64      `json:"id"`
	Number    int        `json:"number"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	State     string     `json:"state"`
	Labels    []Label    `json:"labels"`
	Type      *IssueType `json:"type,omitempty"`
	CreatedAt string     `json:"created_at"`
	Comments  []*Comment `json:"-"`
	Subs      []int      `json:"-"`
	Parent    int        `json:"-"`
}

// PR is a pull request; it shares its number with an Issue, as on GitHub.
type PR struct {
	Number int     `json:"number"`
	Title  string  `json:"title"`
	Body   string  `json:"body"`
	State  string  `json:"state"`
	Draft  bool    `json:"draft"`
	Merged bool    `json:"merged"`
	User   Account `json:"user"`
	Head   Branch  `json:"head"`
	Base   Branch  `json:"base"`
	// MergeCommitSHA is set when the PR merges through the fake.
	MergeCommitSHA string   `json:"merge_commit_sha,omitempty"`
	Reviews        []Review `json:"-"`
	Files          []string `json:"-"`
	Diff           string   `json:"-"`
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
	ID       int64   `json:"id"`
	User     Account `json:"user"`
	State    string  `json:"state"`
	CommitID string  `json:"commit_id"`
}

// Request is one recorded request.
type Request struct {
	Method, Path string
	Body         map[string]any
}

// GitHub is the fake. Lock Mu to read or change state from a test.
type GitHub struct {
	Mu         sync.Mutex
	Owner      string // login of the repository owner
	OwnerType  string // User or Organization
	Repo       string // name
	Private    bool
	Default    string
	Viewer     string
	Labels     []*Label
	IssueTypes []*IssueType
	Rulesets   []map[string]any
	Refs       map[string]string           // "heads/ledger" -> sha
	Trees      map[string][]map[string]any // sha -> entries
	Commits    map[string]map[string]any   // sha -> commit
	Issues     map[int]*Issue
	Pulls      map[int]*PR
	Requests   []Request
	Projects   []map[string]any  // Projects v2: id, title
	Secrets    map[string]string // Actions secret name -> encrypted value
	Variables  map[string]string
	PublicKey  string         // base64 Curve25519 key for Actions secrets
	App        map[string]any // what /app-manifests/{code}/conversions returns
	// Fail makes the request with this "METHOD /path" fail with a 500 once
	// it has succeeded After times.
	Fail      string
	FailAfter int
	seq       int64
	failSeen  int
	Server    *httptest.Server
}

// New starts a fake for owner/name and stops it when the test ends.
func New(t testing.TB, repo, ownerType string) *GitHub {
	owner, name, _ := strings.Cut(repo, "/")
	g := &GitHub{Owner: owner, OwnerType: ownerType, Repo: name, Default: "dev", Viewer: owner,
		Refs: map[string]string{}, Trees: map[string][]map[string]any{}, Commits: map[string]map[string]any{}, Issues: map[int]*Issue{}, Pulls: map[int]*PR{}, seq: 1000}
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
	is := &Issue{ID: g.seq, Number: n, Title: title, Body: body, State: "open", CreatedAt: "2026-10-12T09:00:00Z"}
	for _, l := range labels {
		is.Labels = append(is.Labels, g.label(l))
	}
	g.Issues[n] = is
	return is
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
	pr := &PR{Number: is.Number, Title: title, Body: body, State: "open", User: Account{Login: author, Type: "User"},
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
	rePull     = regexp.MustCompile(`^/repos/[^/]+/[^/]+/pulls/([0-9]+)(/reviews|/files|/merge)?$`)
	reIssue    = regexp.MustCompile(`^/repos/[^/]+/[^/]+/issues/([0-9]+)$`)
	reLabels   = regexp.MustCompile(`^/repos/[^/]+/[^/]+/issues/([0-9]+)/labels$`)
	reLabel    = regexp.MustCompile(`^/repos/[^/]+/[^/]+/issues/([0-9]+)/labels/(.+)$`)
	reComments = regexp.MustCompile(`^/repos/[^/]+/[^/]+/issues/([0-9]+)/comments$`)
	reComment  = regexp.MustCompile(`^/repos/[^/]+/[^/]+/issues/comments/([0-9]+)$`)
	reSubs     = regexp.MustCompile(`^/repos/[^/]+/[^/]+/issues/([0-9]+)/sub_issues$`)
	reParent   = regexp.MustCompile(`^/repos/[^/]+/[^/]+/issues/([0-9]+)/parent$`)
)

func (g *GitHub) serve(w http.ResponseWriter, r *http.Request) {
	g.Mu.Lock()
	defer g.Mu.Unlock()
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

	switch m, p := r.Method, path; {
	case m == "GET" && p == "/user":
		reply(200, map[string]any{"login": g.Viewer, "type": "User"})
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
	case m == "DELETE" && strings.HasPrefix(p, repo+"/actions/variables/"):
		delete(g.Variables, strings.TrimPrefix(p, repo+"/actions/variables/"))
		w.WriteHeader(204)
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
		for n := 1; n <= len(g.Issues); n++ {
			want := r.URL.Query().Get("labels")
			if is := g.Issues[n]; is != nil && (r.URL.Query().Get("state") == "all" || is.State == "open") &&
				(want == "" || slices.ContainsFunc(is.Labels, func(l Label) bool { return l.Name == want })) {
				out = append(out, is)
			}
		}
		reply(200, out)
	case m == "POST" && p == repo+"/issues":
		var labels []string
		for _, l := range body["labels"].([]any) {
			labels = append(labels, l.(string))
		}
		reply(201, g.addIssue(str("title"), str("body"), labels))
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
		case sm[2] == "/reviews" && m == "GET":
			out := []Review{}
			out = append(out, pr.Reviews...)
			reply(200, out)
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
			c := &Comment{ID: g.seq, Body: str("body")}
			c.User.Login, c.User.Type = g.Viewer, "User"
			is.Comments = append(is.Comments, c)
			reply(201, c)
			return
		}
		out := []*Comment{}
		out = append(out, is.Comments...)
		reply(200, out)
	case reComment.MatchString(p) && m == "PATCH":
		id, _ := strconv.ParseInt(reComment.FindStringSubmatch(p)[1], 10, 64)
		for _, is := range g.Issues {
			for _, c := range is.Comments {
				if c.ID == id {
					c.Body = str("body")
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
