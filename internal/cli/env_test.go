package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Zero Options run against the real process: the environment (here the
// config env layer, AAYU_<SECTION>__<KEY>) and the working directory (the
// repository whose config applies). --verbose writes diagnostics, here the
// model substitution, to stderr.
func TestRunUsesProcessEnvironment(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AAYU_AGENT", "")
	t.Setenv("AAYU_MODELS__REVIEW__CURSOR__MODEL", "gpt-6.1-sol")
	t.Chdir(repo)
	var out, errb bytes.Buffer
	code := Run(context.Background(), []string{"models", "show", "--host", "cursor", "--verbose"}, IO{In: strings.NewReader(""), Out: &out, Err: &errb},
		Options{GHToken: func(context.Context) (string, error) { return "", errors.New("gh disabled in tests") }})
	if code != 0 || !regexp.MustCompile(`review\s+cursor\s+grok-4\.7\s+xai\s+fallback`).MatchString(out.String()) {
		t.Fatalf("exit %d\n%s%s", code, out.String(), errb.String())
	}
	if !strings.Contains(errb.String(), "models: review lane on cursor: gpt-6.1-sol (openai) is not available on Cursor") {
		t.Fatalf("--verbose stderr:\n%s", errb.String())
	}
}

// Values reads each input as its declared type, and an unset or mistyped
// input as the zero value.
func TestValues(t *testing.T) {
	v := Values{"days": 14, "title": "x", "create": true, "bad": "14"}
	if v.Int("days") != 14 || v.Int("bad") != 0 || v.Int("unset") != 0 || v.String("title") != "x" || !v.Bool("create") || v.Has("unset") || !v.Has("bad") {
		t.Fatalf("values: %v", v)
	}
}
