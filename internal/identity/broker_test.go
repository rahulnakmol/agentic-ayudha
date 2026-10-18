package identity

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"filippo.io/age"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/secrets"
)

// fakeBroker simulates GitHub running aayudha-token.yml: it holds the App
// private key (a canary) server-side only, and answers a dispatch with a
// completed run whose artifact is the token encrypted to the caller's key.
type fakeBroker struct {
	mu        sync.Mutex
	appKey    string // never sent to the client
	lifetime  time.Duration
	runs      []map[string]any
	artifacts map[int64][]byte
	deleted   []int64
	auth      []string
}

func (f *fakeBroker) handler(t *testing.T) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /repos/o/r/actions/workflows/aayudha-token.yml/dispatches", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.auth = append(f.auth, r.Header.Get("Authorization"))
		var body struct {
			Ref    string            `json:"ref"`
			Inputs map[string]string `json:"inputs"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		rcpt, err := age.ParseX25519Recipient(body.Inputs["recipient"])
		if err != nil || body.Ref != "dev" {
			http.Error(w, "bad inputs", http.StatusUnprocessableEntity)
			return
		}
		// "Mint" with the App key: the token is derived server-side and the key stays here.
		token := "ghs_minted_" + fmt.Sprint(len(f.appKey))
		payload, _ := json.Marshal(map[string]any{"token": token, "expires_at": time.Now().Add(f.lifetime).UTC()})
		var enc bytes.Buffer
		wc, _ := age.Encrypt(&enc, rcpt)
		_, _ = wc.Write(payload)
		_ = wc.Close()
		var zipped bytes.Buffer
		zw := zip.NewWriter(&zipped)
		fw, _ := zw.Create("token.age")
		_, _ = fw.Write(enc.Bytes())
		_ = zw.Close()
		id := int64(100 + len(f.runs))
		f.runs = append(f.runs, map[string]any{"id": id, "name": "aayudha-token " + body.Inputs["request_id"], "status": "completed", "conclusion": "success"})
		f.artifacts[id] = zipped.Bytes()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /repos/o/r/actions/workflows/aayudha-token.yml/runs", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"workflow_runs": f.runs})
	})
	mux.HandleFunc("GET /repos/o/r/actions/runs/{id}/artifacts", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		_ = json.NewEncoder(w).Encode(map[string]any{"artifacts": []map[string]any{{
			"id": 9000, "name": "aayudha-token-" + strings.TrimPrefix(f.runName(id), "aayudha-token "),
			"archive_download_url": "http://" + r.Host + "/dl/" + id,
		}}})
	})
	mux.HandleFunc("GET /dl/{id}", func(w http.ResponseWriter, r *http.Request) {
		var id int64
		_, _ = fmt.Sscan(r.PathValue("id"), &id)
		f.mu.Lock()
		defer f.mu.Unlock()
		_, _ = w.Write(f.artifacts[id])
	})
	mux.HandleFunc("DELETE /repos/o/r/actions/artifacts/{id}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.deleted = append(f.deleted, 9000)
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

func (f *fakeBroker) runName(id string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.runs {
		if fmt.Sprint(r["id"]) == id {
			return r["name"].(string)
		}
	}
	return ""
}

const appKeyCanary = "APP-PRIVATE-KEY-CANARY-a6f1d9e2"

func TestBrokerRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		lifetime time.Duration
		wantErr  string
	}{
		{"one-hour token is accepted", 55 * time.Minute, ""},
		{"token outliving an hour is refused", 2 * time.Hour, "outlives"},
		{"expired token is refused", -time.Minute, "expired"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home, tmp, work := t.TempDir(), t.TempDir(), t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("TMPDIR", tmp)
			t.Chdir(work)
			f := &fakeBroker{appKey: appKeyCanary, lifetime: tt.lifetime, artifacts: map[int64][]byte{}}
			srv := httptest.NewServer(f.handler(t))
			defer srv.Close()

			b := Broker{API: srv.URL, Repo: "o/r", Workflow: "aayudha-token.yml", Ref: "dev",
				Human: Token{"human-token", "test"}, HTTP: srv.Client(), Poll: time.Millisecond}
			tok, err := b.Mint(context.Background())
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tok.Value != "ghs_minted_"+fmt.Sprint(len(appKeyCanary)) || !strings.Contains(tok.Source, "expires") {
				t.Fatalf("token = %+v", tok)
			}
			if len(f.auth) != 1 || f.auth[0] != "Bearer human-token" {
				t.Fatalf("dispatch was not authorised by the human token: %q", f.auth)
			}
			if len(f.deleted) != 1 {
				t.Fatalf("artifact not deleted after download")
			}
			// The App private key must never reach the client's environment or disk.
			for _, kv := range os.Environ() {
				if strings.Contains(kv, "CANARY-a6f1d9e2") {
					t.Fatalf("App key canary in the client environment")
				}
			}
			for _, dir := range []string{home, tmp, work} {
				scanForCanary(t, dir, "CANARY-a6f1d9e2")
				scanForCanary(t, dir, tok.Value) // nor the minted token
			}
		})
	}
}

// scanForCanary fails if any file under dir contains canary.
func scanForCanary(t *testing.T, dir, canary string) {
	t.Helper()
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.Contains(b, []byte(canary)) {
			t.Errorf("canary %q found in %s", canary, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestBrokerLive mints a real token through a scratch repo that has the bot App
// installed and aayudha-token.yml on its default branch. Founder setup: create
// the App, install it on the scratch repo, set AAYUDHA_APP_CLIENT_ID and
// AAYUDHA_APP_PRIVATE_KEY there, then run with
// AAYU_LIVE=1 AAYU_LIVE_BROKER_REPO=owner/scratch AAYU_GITHUB_TOKEN=... go test -run Live ./internal/identity
func TestBrokerLive(t *testing.T) {
	repo := os.Getenv("AAYU_LIVE_BROKER_REPO")
	if os.Getenv("AAYU_LIVE") != "1" || repo == "" {
		t.Skip("live broker test: set AAYU_LIVE=1 and AAYU_LIVE_BROKER_REPO")
	}
	human, err := Resolver{Getenv: os.Getenv}.Human(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ref := os.Getenv("AAYU_LIVE_BROKER_REF")
	if ref == "" {
		ref = "main"
	}
	start := time.Now()
	tok, err := Broker{API: "https://api.github.com", Repo: repo, Workflow: "aayudha-token.yml", Ref: ref,
		Human: human, HTTP: http.DefaultClient}.Mint(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// The installation token must work and must not outlive an hour (Mint enforces it).
	u, err := whoAmIApp(context.Background(), tok)
	if err != nil {
		t.Fatalf("minted token does not work: %v", err)
	}
	t.Logf("minted an installation token for %d repos in %s", u, time.Since(start).Round(time.Second))
	for _, kv := range os.Environ() {
		if strings.Contains(kv, "PRIVATE KEY") {
			t.Fatal("a private key is present in the client environment")
		}
	}
	home, _ := os.UserHomeDir()
	for _, dir := range []string{os.TempDir(), filepath.Join(home, ".config", "aayudha"), filepath.Join(home, ".local", "share", "aayudha")} {
		if _, err := os.Stat(dir); err == nil {
			scanForCanary(t, dir, "PRIVATE KEY-----")
		}
	}
}

func whoAmIApp(ctx context.Context, tok Token) (int, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/installation/repositories", nil)
	req.Header.Set("Authorization", "Bearer "+tok.Value)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return 0, errors.New(resp.Status + " " + string(b))
	}
	var v struct {
		TotalCount int `json:"total_count"`
	}
	return v.TotalCount, json.NewDecoder(resp.Body).Decode(&v)
}

func TestBotResolution(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	noGH := func(context.Context) (string, error) { return "", errors.New("no gh") }
	app := config.Identity{AppID: 42, AppSlug: "aayudha-bot"}
	store := secrets.New(secrets.Env{Getenv: env(map[string]string{"AAYU_SECRET_MACHINE_PAT": "pat-1"})})

	tests := []struct {
		name     string
		r        Resolver
		id       config.Identity
		wantKind string
		wantTag  string
		wantErr  error
	}{
		{"CI with only GITHUB_TOKEN has no bot", Resolver{Getenv: env(map[string]string{"GITHUB_TOKEN": "ghs_actions"}), CI: true, GH: noGH}, app, "", "", ErrNoBotInCI},
		{"CI refuses GITHUB_TOKEN passed as the bot token", Resolver{Getenv: env(map[string]string{"GITHUB_TOKEN": "ghs_x", "AAYU_BOT_TOKEN": "ghs_x"}), CI: true, GH: noGH}, app, "", "", ErrNoBotInCI},
		{"CI uses the App installation token", Resolver{Getenv: env(map[string]string{"GITHUB_TOKEN": "ghs_x", "AAYU_BOT_TOKEN": "ghs_app"}), CI: true, GH: noGH}, app, KindApp, "", nil},
		{"machine user from the secrets store", Resolver{Getenv: env(nil), GH: noGH, Secrets: store}, config.Identity{MachineUser: "aayu-machine", MachineUserSecret: "machine-pat"}, KindMachineUser, "", nil},
		{"day one: human fallback, tagged", Resolver{Getenv: env(map[string]string{"AAYU_GITHUB_TOKEN": "h"}), GH: noGH}, config.Identity{}, KindHumanFallback, HumanFallbackTag, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := tt.r.Bot(context.Background(), tt.id, nil, "rahulnakmol")
			if !errors.Is(err, tt.wantErr) || b.Kind != tt.wantKind || b.Tag != tt.wantTag {
				t.Fatalf("bot = %+v, err = %v; want kind %q tag %q err %v", b, err, tt.wantKind, tt.wantTag, tt.wantErr)
			}
		})
	}
}

func TestHumanTokenOrder(t *testing.T) {
	gh := func(context.Context) (string, error) { return "from-gh", nil }
	noGH := func(context.Context) (string, error) { return "", errors.New("no gh") }
	tests := []struct {
		name string
		r    Resolver
		want string
	}{
		{"AAYU_GITHUB_TOKEN first", Resolver{Getenv: func(k string) string { return map[string]string{"AAYU_GITHUB_TOKEN": "a", "GITHUB_TOKEN": "g"}[k] }, CI: true, GH: gh}, "AAYU_GITHUB_TOKEN"},
		{"GITHUB_TOKEN with --ci", Resolver{Getenv: func(k string) string { return map[string]string{"GITHUB_TOKEN": "g"}[k] }, CI: true, GH: gh}, "GITHUB_TOKEN (--ci)"},
		{"GITHUB_TOKEN ignored without --ci", Resolver{Getenv: func(k string) string { return map[string]string{"GITHUB_TOKEN": "g"}[k] }, GH: gh}, "gh auth token"},
		{"secrets store after gh", Resolver{Getenv: func(string) string { return "" }, GH: noGH,
			Secrets: secrets.New(secrets.Env{Getenv: func(k string) string { return map[string]string{"AAYU_SECRET_GITHUB_TOKEN": "s"}[k] }})}, "secrets store (env)"},
		{"prompt last", Resolver{Getenv: func(string) string { return "" }, GH: noGH, Prompt: func(context.Context) (string, error) { return "p", nil }}, "prompt"},
	}
	for _, tt := range tests {
		tok, err := tt.r.Human(context.Background())
		if err != nil || tok.Source != tt.want {
			t.Errorf("%s: source %q, err %v; want %q", tt.name, tok.Source, err, tt.want)
		}
	}
	if _, err := (Resolver{Getenv: func(string) string { return "" }, GH: noGH}).Human(context.Background()); !errors.Is(err, ErrNoToken) {
		t.Errorf("no source: err = %v", err)
	}
	// Copilot CLI's own token is never aayu's: under copilot, aayu's GitHub
	// identity stays the one gh or AAYU_GITHUB_TOKEN gives it.
	copilot := func(k string) string { return map[string]string{"COPILOT_GITHUB_TOKEN": "copilot-account"}[k] }
	if tok, err := (Resolver{Getenv: copilot, CI: true, GH: noGH}).Human(context.Background()); !errors.Is(err, ErrNoToken) {
		t.Errorf("COPILOT_GITHUB_TOKEN alone resolved a human token from %s", tok.Source)
	}
}
