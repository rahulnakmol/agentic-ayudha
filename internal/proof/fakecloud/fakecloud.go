// Package fakecloud holds in-memory Azure Blob Storage (with the Azure
// Resource Manager management policy) and Google Cloud Storage (JSON API)
// servers for tests of the proof media stores (WP7.2b.2). Every request's
// credential is checked: a request that does not carry it is rejected and
// counted.
package fakecloud

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// Store is what both fakes hold.
type Store struct {
	Mu        sync.Mutex
	Objects   map[string][]byte // "<container or bucket>/<key>"
	Lifecycle []byte            // the management policy (Azure) or the bucket's lifecycle (GCS), as JSON
	Rejected  int
	Server    *httptest.Server
}

func newStore(t testing.TB, h http.HandlerFunc) *Store {
	s := &Store{Objects: map[string][]byte{}}
	s.Server = httptest.NewServer(h)
	t.Cleanup(s.Server.Close)
	return s
}

func (s *Store) reject(w http.ResponseWriter, why string) {
	s.Rejected++
	http.Error(w, why, http.StatusForbidden)
}

// Azure is a storage account: blobs at /<container>/<key>, and its
// management policy at the Azure Resource Manager path.
type Azure struct {
	*Store
	SAS   string // when set, blob requests must carry exactly this query; else the bearer below
	Blob  string // bearer for blob requests
	ARM   string // bearer for management policy requests
	Calls []string
}

var policyPath = regexp.MustCompile(`^/subscriptions/[^/]+/resourceGroups/[^/]+/providers/Microsoft\.Storage/storageAccounts/[^/]+/managementPolicies/default$`)

// NewAzure starts the fake.
func NewAzure(t testing.TB, sas, blobBearer, armBearer string) *Azure {
	a := &Azure{SAS: sas, Blob: blobBearer, ARM: armBearer}
	a.Store = newStore(t, a.serve)
	return a
}

func (a *Azure) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	a.Mu.Lock()
	defer a.Mu.Unlock()
	if policyPath.MatchString(r.URL.Path) {
		if r.Header.Get("Authorization") != "Bearer "+a.ARM || r.URL.Query().Get("api-version") == "" {
			a.reject(w, `{"error":{"code":"AuthenticationFailed"}}`)
			return
		}
		a.Calls = append(a.Calls, "arm "+r.Method)
		switch r.Method {
		case http.MethodGet:
			if a.Lifecycle == nil {
				http.Error(w, `{"error":{"code":"ManagementPolicyNotFound"}}`, http.StatusNotFound)
				return
			}
			_, _ = w.Write(a.Lifecycle)
		case http.MethodPut:
			var p struct {
				Properties struct {
					Policy struct {
						Rules []json.RawMessage `json:"rules"`
					} `json:"policy"`
				} `json:"properties"`
			}
			if json.Unmarshal(body, &p) != nil || len(p.Properties.Policy.Rules) == 0 {
				http.Error(w, `{"error":{"code":"InvalidManagementPolicy"}}`, http.StatusBadRequest)
				return
			}
			a.Lifecycle = body
			_, _ = w.Write(body)
		case http.MethodDelete:
			a.Lifecycle = nil
		}
		return
	}
	if r.Header.Get("x-ms-version") == "" {
		http.Error(w, "MissingRequiredHeader x-ms-version", http.StatusBadRequest)
		return
	}
	if a.SAS != "" {
		want, _ := url.ParseQuery(a.SAS)
		if r.URL.Query().Encode() != want.Encode() || r.Header.Get("Authorization") != "" {
			a.reject(w, "AuthenticationFailed: SAS")
			return
		}
	} else if r.Header.Get("Authorization") != "Bearer "+a.Blob {
		a.reject(w, "AuthenticationFailed: bearer")
		return
	}
	k := strings.TrimPrefix(r.URL.Path, "/")
	a.Calls = append(a.Calls, "blob "+r.Method)
	switch r.Method {
	case http.MethodPut:
		if r.Header.Get("x-ms-blob-type") != "BlockBlob" {
			http.Error(w, "InvalidHeaderValue x-ms-blob-type", http.StatusBadRequest)
			return
		}
		a.Objects[k] = body
		w.WriteHeader(http.StatusCreated)
	case http.MethodGet:
		b, ok := a.Objects[k]
		if !ok {
			http.Error(w, "BlobNotFound", http.StatusNotFound)
			return
		}
		_, _ = w.Write(b)
	case http.MethodDelete:
		if _, ok := a.Objects[k]; !ok {
			http.Error(w, "BlobNotFound", http.StatusNotFound)
			return
		}
		delete(a.Objects, k)
		w.WriteHeader(http.StatusAccepted)
	}
}

// GCS is a Cloud Storage JSON API endpoint.
type GCS struct {
	*Store
	Bearer string
}

// NewGCS starts the fake.
func NewGCS(t testing.TB, bearer string) *GCS {
	g := &GCS{Bearer: bearer}
	g.Store = newStore(t, g.serve)
	return g
}

func (g *GCS) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	g.Mu.Lock()
	defer g.Mu.Unlock()
	if r.Header.Get("Authorization") != "Bearer "+g.Bearer {
		g.reject(w, `{"error":{"code":401}}`)
		return
	}
	p := r.URL.EscapedPath()
	switch {
	case strings.HasPrefix(p, "/upload/storage/v1/b/") && r.Method == http.MethodPost:
		b := strings.TrimSuffix(strings.TrimPrefix(p, "/upload/storage/v1/b/"), "/o")
		if r.URL.Query().Get("uploadType") != "media" || r.URL.Query().Get("name") == "" {
			http.Error(w, `{"error":{"code":400}}`, http.StatusBadRequest)
			return
		}
		g.Objects[b+"/"+r.URL.Query().Get("name")] = body
		_, _ = w.Write([]byte(`{"kind":"storage#object"}`))
	case strings.Contains(p, "/o/"):
		rest := strings.TrimPrefix(p, "/storage/v1/b/")
		b, name, _ := strings.Cut(rest, "/o/")
		name, _ = url.PathUnescape(name)
		k := b + "/" + name
		switch r.Method {
		case http.MethodGet:
			obj, ok := g.Objects[k]
			if !ok || r.URL.Query().Get("alt") != "media" {
				http.Error(w, `{"error":{"code":404}}`, http.StatusNotFound)
				return
			}
			_, _ = w.Write(obj)
		case http.MethodDelete:
			if _, ok := g.Objects[k]; !ok {
				http.Error(w, `{"error":{"code":404}}`, http.StatusNotFound)
				return
			}
			delete(g.Objects, k)
			w.WriteHeader(http.StatusNoContent)
		}
	case strings.HasPrefix(p, "/storage/v1/b/") && r.URL.Query().Get("fields") == "lifecycle":
		switch r.Method {
		case http.MethodGet:
			if g.Lifecycle == nil {
				_, _ = w.Write([]byte(`{}`))
				return
			}
			_, _ = w.Write(g.Lifecycle)
		case http.MethodPatch:
			g.Lifecycle = body
			_, _ = w.Write(body)
		}
	default:
		http.Error(w, `{"error":{"code":404}}`, http.StatusNotFound)
	}
}

// Rewrite sends requests for host to the server s instead (Azure Resource
// Manager in tests), and every other request to next.
func Rewrite(next http.RoundTripper, host string, s *httptest.Server) http.RoundTripper {
	return rewrite{next, host, s}
}

type rewrite struct {
	next http.RoundTripper
	host string
	s    *httptest.Server
}

func (r rewrite) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host != r.host {
		return r.next.RoundTrip(req)
	}
	out := req.Clone(req.Context())
	u, _ := url.Parse(r.s.URL)
	out.URL.Scheme, out.URL.Host, out.Host = u.Scheme, u.Host, u.Host
	return http.DefaultTransport.RoundTrip(out)
}
