package evals

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// SurveyDocs is a manual a survey case fetches at run time rather than
// carries, for a document the repository may not redistribute: tier 2
// downloads it once into the eval docs cache, checks the file and the text
// pdftotext makes of it against their pinned SHA-256s, and copies it to a
// temporary directory the host surveys with `aayu survey --docs`. Neither the
// file nor its text is ever committed. A fetch that fails or a hash that
// differs makes the case not run (NotRunError): no score, no spend, no
// failure.
type SurveyDocs struct {
	Name string `toml:"name"` // the file's name under --docs; a PDF is cited as manual:<name>.txt
	DOI  string `toml:"doi"`  // what URL was resolved from, for the record
	URL  string `toml:"url"`  // the official file the DOI resolves to
	// SHA256 is the file's; TextSHA256 is pdftotext's text of it, so a
	// converter that changes the text cannot move the cited lines.
	SHA256     string `toml:"sha256"`
	TextSHA256 string `toml:"text_sha256"`
}

var sha256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)

func (d *SurveyDocs) check() error {
	switch {
	case d.Name == "" || strings.ContainsAny(d.Name, `/\`) || !strings.HasSuffix(d.Name, ".pdf"):
		return fmt.Errorf("[survey.docs] name %q: want a PDF's file name", d.Name)
	case !strings.HasPrefix(d.URL, "https://"):
		return fmt.Errorf("[survey.docs] url %q: want an https URL", d.URL)
	case !sha256Hex.MatchString(d.SHA256) || !sha256Hex.MatchString(d.TextSHA256):
		return errors.New("[survey.docs] sha256 and text_sha256 must each be 64 lowercase hex digits")
	}
	return nil
}

// NotRunError is a case that cannot run where it is: the reason is reported,
// and the case gets no score, spends nothing, and fails nothing.
type NotRunError struct{ Why string }

func (e *NotRunError) Error() string { return "not run: " + e.Why }

// DocsCacheEnv names the directory the fetched manuals are cached in, by
// their SHA-256 (evals.yml restores and saves it with actions/cache).
const DocsCacheEnv = "AAYU_EVAL_DOCS"

func docsCache() (string, error) {
	if d := os.Getenv(DocsCacheEnv); d != "" {
		return d, nil
	}
	d, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "aayudha", "eval-docs"), nil
}

// cached returns the cached file's bytes when they match the pin.
func (d *SurveyDocs) cached() ([]byte, bool) {
	dir, err := docsCache()
	if err != nil {
		return nil, false
	}
	b, err := os.ReadFile(filepath.Join(dir, d.SHA256))
	if err != nil {
		return nil, false
	}
	return b, sumHex(b) == d.SHA256
}

func sumHex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// maxDocBytes bounds a download, so a wrong URL cannot fill the disk.
const maxDocBytes = 64 << 20

// fetch puts the manual in the cache unless it is there: one GET of the
// pinned URL, checked against the pinned SHA-256 before it is kept.
func (d *SurveyDocs) fetch(ctx context.Context) error {
	if _, ok := d.cached(); ok {
		return nil
	}
	dir, err := docsCache()
	if err != nil {
		return &NotRunError{Why: "no cache directory for " + d.Name + ": " + err.Error()}
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "aayudha-evals (+https://github.com/rahulnakmol/agentic-ayudha)")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return &NotRunError{Why: fmt.Sprintf("fetching %s from %s: %v", d.Name, d.URL, err)}
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return &NotRunError{Why: fmt.Sprintf("fetching %s from %s: HTTP %d", d.Name, d.URL, resp.StatusCode)}
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxDocBytes+1))
	switch {
	case err != nil:
		return &NotRunError{Why: fmt.Sprintf("fetching %s from %s: %v", d.Name, d.URL, err)}
	case len(b) > maxDocBytes:
		return &NotRunError{Why: fmt.Sprintf("%s from %s is larger than %d bytes", d.Name, d.URL, maxDocBytes)}
	}
	if got := sumHex(b); got != d.SHA256 {
		return &NotRunError{Why: fmt.Sprintf("%s from %s has SHA-256 %s; the case pins %s", d.Name, d.URL, got, d.SHA256)}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := filepath.Join(dir, d.SHA256+".tmp")
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, d.SHA256))
}

// dir copies the cached manual to a new temporary directory for --docs; the
// caller removes it. Without the cached file or pdftotext the case does not run.
func (d *SurveyDocs) dir() (string, error) {
	b, ok := d.cached()
	if !ok {
		return "", &NotRunError{Why: fmt.Sprintf("%s (SHA-256 %s) is not in the eval docs cache; tier 2 fetches it from %s", d.Name, d.SHA256, d.URL)}
	}
	if _, err := exec.LookPath("pdftotext"); err != nil {
		return "", &NotRunError{Why: d.Name + " is a PDF and pdftotext is not installed (poppler-utils)"}
	}
	dir, err := os.MkdirTemp("", "aayu-survey-docs-")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, d.Name), b, 0o644); err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	return dir, nil
}

// checkText checks the text `aayu survey --docs` made of the manual, under
// manuals, against the pin.
func (d *SurveyDocs) checkText(manuals string) error {
	b, err := os.ReadFile(filepath.Join(manuals, d.Name+".txt"))
	if err != nil {
		return err
	}
	if got := sumHex(b); got != d.TextSHA256 {
		return &NotRunError{Why: fmt.Sprintf("pdftotext's text of %s has SHA-256 %s; the case pins %s (another poppler version would move the cited lines)", d.Name, got, d.TextSHA256)}
	}
	return nil
}
