package identity

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"filippo.io/age"
)

// BrokerWorkflow is the token broker workflow template; `init --gates` (WP2.6)
// installs it as .github/workflows/aayudha-token.yml.
//
//go:embed aayudha-token.yml
var BrokerWorkflow []byte

// MaxTokenLifetime is the longest-lived bot token a laptop accepts.
const MaxTokenLifetime = time.Hour

// Broker mints App installation tokens through the broker workflow.
type Broker struct {
	API      string // e.g. https://api.github.com
	Repo     string // owner/name
	Workflow string // workflow file name, e.g. aayudha-token.yml
	Ref      string // branch the workflow runs from, e.g. dev
	Human    Token  // authorises the dispatch; the run is attributed to this human
	HTTP     *http.Client
	Poll     time.Duration // default 3s
	Timeout  time.Duration // default 3m
	Now      func() time.Time
}

// Mint dispatches the broker and returns the decrypted installation token. The
// one-time age identity exists only in memory for the duration of the call.
func (b Broker) Mint(ctx context.Context) (Token, error) {
	if b.Poll == 0 {
		b.Poll = 3 * time.Second
	}
	if b.Timeout == 0 {
		b.Timeout = 3 * time.Minute
	}
	if b.Now == nil {
		b.Now = time.Now
	}
	ctx, cancel := context.WithTimeout(ctx, b.Timeout)
	defer cancel()

	id, err := age.GenerateX25519Identity()
	if err != nil {
		return Token{}, err
	}
	rid := make([]byte, 16)
	if _, err := rand.Read(rid); err != nil {
		return Token{}, err
	}
	requestID := hex.EncodeToString(rid)

	body, _ := json.Marshal(map[string]any{"ref": b.Ref, "inputs": map[string]string{
		"recipient": id.Recipient().String(), "request_id": requestID,
	}})
	if _, err := b.call(ctx, http.MethodPost, fmt.Sprintf("/repos/%s/actions/workflows/%s/dispatches", b.Repo, b.Workflow), body, nil); err != nil {
		return Token{}, fmt.Errorf("dispatch %s: %w", b.Workflow, err)
	}

	runID, err := b.waitForRun(ctx, requestID)
	if err != nil {
		return Token{}, err
	}
	var arts struct {
		Artifacts []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
			URL  string `json:"archive_download_url"`
		} `json:"artifacts"`
	}
	if _, err := b.call(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/actions/runs/%d/artifacts", b.Repo, runID), nil, &arts); err != nil {
		return Token{}, err
	}
	want := "aayudha-token-" + requestID
	for _, a := range arts.Artifacts {
		if a.Name != want {
			continue
		}
		zipped, err := b.download(ctx, a.URL)
		// The ciphertext is useless to anyone else, but do not leave it lying around.
		_, _ = b.call(context.WithoutCancel(ctx), http.MethodDelete, fmt.Sprintf("/repos/%s/actions/artifacts/%d", b.Repo, a.ID), nil, nil)
		if err != nil {
			return Token{}, fmt.Errorf("download %s: %w", want, err)
		}
		return b.open(zipped, id)
	}
	return Token{}, fmt.Errorf("broker run %d produced no artifact %s", runID, want)
}

func (b Broker) waitForRun(ctx context.Context, requestID string) (int64, error) {
	name := "aayudha-token " + requestID
	var runID int64
	for {
		var runs struct {
			WorkflowRuns []struct {
				ID         int64  `json:"id"`
				Name       string `json:"name"`
				Status     string `json:"status"`
				Conclusion string `json:"conclusion"`
				HTMLURL    string `json:"html_url"`
			} `json:"workflow_runs"`
		}
		path := fmt.Sprintf("/repos/%s/actions/workflows/%s/runs?event=workflow_dispatch&per_page=30", b.Repo, b.Workflow)
		if _, err := b.call(ctx, http.MethodGet, path, nil, &runs); err != nil {
			return 0, err
		}
		for _, r := range runs.WorkflowRuns {
			if r.Name != name {
				continue
			}
			runID = r.ID
			if r.Status == "completed" {
				if r.Conclusion != "success" {
					return 0, fmt.Errorf("broker run %s concluded %s", r.HTMLURL, r.Conclusion)
				}
				return r.ID, nil
			}
		}
		select {
		case <-ctx.Done():
			if runID == 0 {
				return 0, fmt.Errorf("broker run for request %s never appeared: %w", requestID, ctx.Err())
			}
			return 0, fmt.Errorf("broker run %d did not finish: %w", runID, ctx.Err())
		case <-time.After(b.Poll):
		}
	}
}

func (b Broker) open(zipped []byte, id *age.X25519Identity) (Token, error) {
	zr, err := zip.NewReader(bytes.NewReader(zipped), int64(len(zipped)))
	if err != nil {
		return Token{}, err
	}
	for _, f := range zr.File {
		if f.Name != "token.age" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return Token{}, err
		}
		defer func() { _ = rc.Close() }()
		r, err := age.Decrypt(rc, id)
		if err != nil {
			return Token{}, fmt.Errorf("decrypt broker token: %w", err)
		}
		var payload struct {
			Token     string    `json:"token"`
			ExpiresAt time.Time `json:"expires_at"`
		}
		if err := json.NewDecoder(io.LimitReader(r, 1<<16)).Decode(&payload); err != nil {
			return Token{}, err
		}
		if payload.Token == "" {
			return Token{}, errors.New("broker returned an empty token")
		}
		if left := payload.ExpiresAt.Sub(b.Now()); left <= 0 || left > MaxTokenLifetime {
			return Token{}, fmt.Errorf("broker token expires at %s; refusing a token that is expired or outlives %s", payload.ExpiresAt.Format(time.RFC3339), MaxTokenLifetime)
		}
		return Token{Value: payload.Token, Source: "token broker (" + b.Workflow + ", expires " + payload.ExpiresAt.Format(time.RFC3339) + ")"}, nil
	}
	return Token{}, errors.New("broker artifact has no token.age")
}

func (b Broker) call(ctx context.Context, method, path string, body []byte, out any) (int, error) {
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimSuffix(b.API, "/")+path, rd)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+b.Human.Value)
	req.Header.Set("Accept", "application/vnd.github+json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := b.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return resp.StatusCode, fmt.Errorf("%s %s: %s %s", method, path, resp.Status, strings.TrimSpace(string(msg)))
	}
	if out != nil {
		return resp.StatusCode, json.NewDecoder(resp.Body).Decode(out)
	}
	return resp.StatusCode, nil
}

func (b Broker) download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+b.Human.Value)
	resp, err := b.HTTP.Do(req) // GitHub redirects to a signed blob URL; Go drops the header across hosts
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET artifact: %s", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}
