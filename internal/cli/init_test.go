package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// WP2.2 accept: init --dry-run lists the labels and issue types and writes nothing.
func TestInitDryRunListsLabelsAndTypes(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Method+" "+r.URL.Path)
		mu.Unlock()
		switch r.URL.Path {
		case "/users/acme":
			_, _ = w.Write([]byte(`{"login":"acme","type":"Organization"}`))
		case "/users/solo":
			_, _ = w.Write([]byte(`{"login":"solo","type":"User"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	for _, tc := range []struct {
		repo         string
		org          bool
		types, label int
	}{
		{"acme/app", true, 10, len(workitems.Labels()) - 10},
		{"solo/app", false, 0, len(workitems.Labels())},
	} {
		code, out, errOut := run(t, nil, Options{GitHubAPI: srv.URL}, "init", "--dry-run", "--json", "--repo", tc.repo)
		if code != 0 {
			t.Fatalf("%s: exit %d: %s", tc.repo, code, errOut)
		}
		var rep struct {
			Organization bool `json:"organization"`
			Planned      []struct {
				Kind    string         `json:"kind"`
				Status  string         `json:"status"`
				Payload map[string]any `json:"payload"`
			} `json:"planned"`
		}
		if err := json.Unmarshal([]byte(out), &rep); err != nil {
			t.Fatal(err)
		}
		types, labels, typeLabels := 0, 0, 0
		for _, p := range rep.Planned {
			if p.Status != "planned" {
				t.Errorf("%s: %s is %s, want planned", tc.repo, p.Kind, p.Status)
			}
			switch p.Kind {
			case "github.issue_type.create":
				types++
			case "github.label.create":
				labels++
				if strings.HasPrefix(p.Payload["name"].(string), "type:") {
					typeLabels++
				}
			}
		}
		if rep.Organization != tc.org || types != tc.types || labels != tc.label || (typeLabels > 0) == tc.org {
			t.Errorf("%s: org %t, %d types, %d labels (%d type:*), want org %t, %d types, %d labels", tc.repo, rep.Organization, types, labels, typeLabels, tc.org, tc.types, tc.label)
		}
	}
	for _, s := range seen {
		if !strings.HasPrefix(s, "GET ") {
			t.Errorf("dry run sent %s", s)
		}
	}
	if code, _, errOut := run(t, nil, Options{GitHubAPI: srv.URL}, "init", "--repo", "solo/app"); code != ExitUsage || !strings.Contains(errOut, "WP2.5") {
		t.Errorf("init without --dry-run: exit %d, %s", code, errOut)
	}
}
