// Package scratch creates and deletes the aayu-scratch-* repositories that
// live tests (AAYU_LIVE=1) run against.
package scratch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"
)

// Token returns the human token for live tests: AAYU_GITHUB_TOKEN, GH_TOKEN,
// GITHUB_TOKEN, then `gh auth token`.
func Token(t testing.TB) string {
	for _, k := range []string{"AAYU_GITHUB_TOKEN", "GH_TOKEN", "GITHUB_TOKEN"} {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil || len(bytes.TrimSpace(out)) == 0 {
		t.Fatal("no GitHub token: set AAYU_GITHUB_TOKEN or log in with gh")
	}
	return string(bytes.TrimSpace(out))
}

// Repo creates a private aayu-scratch-* repository for the token's user
// with a first commit, and deletes it when the test ends. Test scaffolding,
// not a verb write, so it calls the API directly.
func Repo(t testing.TB, token string) string {
	t.Helper()
	call := func(method, path string, body any) (int, []byte) {
		var rd io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			rd = bytes.NewReader(b)
		}
		req, _ := http.NewRequest(method, "https://api.github.com"+path, rd)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github+json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, b
	}
	name := fmt.Sprintf("aayu-scratch-%d", time.Now().UnixNano())
	code, b := call("POST", "/user/repos", map[string]any{"name": name, "private": true, "auto_init": true, "description": "aayudha live test scratch repository; deleted by the test"})
	if code != http.StatusCreated {
		t.Fatalf("create scratch repository: %d %s", code, b)
	}
	var r struct {
		FullName string `json:"full_name"`
	}
	_ = json.Unmarshal(b, &r)
	t.Cleanup(func() {
		if code, b := call("DELETE", "/repos/"+r.FullName, nil); code != http.StatusNoContent {
			t.Errorf("delete scratch repository %s: %d %s", r.FullName, code, b)
		}
	})
	time.Sleep(2 * time.Second) // let the first commit and default labels settle
	return r.FullName
}
