package models

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeCopilot writes a copilot executable into a new directory that records
// its arguments, environment, and the first bytes of each attachment, writes
// the recorded probe's usage file where --usage-output-file says, and prints
// the recorded JSONL (testdata/copilot, from a gemini-4-argon probe of lane
// A's rest-only prompt on 2026-10-18). exit is its exit status.
func fakeCopilot(t *testing.T, exit int, usage bool) (dir, record string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake copilot is a shell script")
	}
	dir, record = t.TempDir(), filepath.Join(t.TempDir(), "record")
	data, _ := filepath.Abs("testdata/copilot")
	script := `#!/bin/sh
rec='` + record + `'
env > "$rec.env"
: > "$rec.args"
usage=
while [ $# -gt 0 ]; do
  printf '%s\n' "$1" >> "$rec.args"
  case "$1" in
    --usage-output-file) usage="$2" ;;
    --attachment) head -c 8 "$2" > "$rec.attachment" ;;
  esac
  shift
done
pwd > "$rec.pwd"
` + map[bool]string{true: "cp '" + data + "/usage.json' \"$usage\"", false: ": no usage file"}[usage] + `
cat '` + data + `/out.jsonl'
echo "auth failed for this account" >&2
exit ` + map[bool]string{true: "0", false: "1"}[exit == 0] + `
`
	if err := os.WriteFile(filepath.Join(dir, "copilot"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir + string(os.PathListSeparator) + "/usr/bin" + string(os.PathListSeparator) + "/bin", record
}

// TestCopilotCLI: the opt-in Copilot route runs copilot -p with the request's
// model and effort, the image as an attachment, no tools, in an empty
// directory with a fresh COPILOT_HOME and only COPILOT_GITHUB_TOKEN of the
// caller's credentials; it reads the reply and charges the AI credits Copilot
// reports. It fails with ErrCopilot when copilot is missing, not signed in, or
// exits non-zero.
func TestCopilotCLI(t *testing.T) {
	dir, rec := fakeCopilot(t, 0, true)
	t.Setenv("GH_TOKEN", "must-not-leak")
	png := []byte("\x89PNG\r\n\x1a\nrest")
	req := Request{Model: "gemini-4-argon", Effort: "high", System: "You are lane A.",
		Messages: []Message{{Role: "user", Content: "State \"s\".", Images: []Image{{MediaType: "image/png", Data: png}}}}}
	resp, err := CopilotCLI{Token: "copilot-tok", Path: dir}.Complete(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	// The recorded reply and usage: 19,098 input, 437 output, 3,865 reasoning
	// tokens; 4,256,600,000 nano AI units = 4.2566 credits = 0.042566 USD.
	if !strings.HasPrefix(resp.Text, `{"findings":[`) || resp.Model != "gemini-4-argon" || resp.TokensIn != 19098 || resp.TokensOut != 437+3865 ||
		math.Abs(resp.CostUSD-0.042566) > 1e-9 {
		t.Fatalf("response: model %q, in %d, out %d, cost %v, text %.40q", resp.Model, resp.TokensIn, resp.TokensOut, resp.CostUSD, resp.Text)
	}
	if usd, src := Cost(nil, resp.Model, resp); src != CostReported || usd != resp.CostUSD {
		t.Fatalf("cost %v from %s, want the reported credits", usd, src)
	}
	args, _ := os.ReadFile(rec + ".args")
	for _, want := range []string{"-p\nYou are lane A.\n\nState \"s\".\n", "--model\ngemini-4-argon\n", "--reasoning-effort\nhigh\n", "--available-tools=\n", "--no-ask-user\n", "--no-custom-instructions\n", "--output-format\njson\n"} {
		if !strings.Contains(string(args), want) {
			t.Errorf("args lack %q:\n%s", want, args)
		}
	}
	if b, _ := os.ReadFile(rec + ".attachment"); string(b) != string(png[:8]) {
		t.Errorf("attachment starts %q, want the PNG", b)
	}
	env, _ := os.ReadFile(rec + ".env")
	home, _ := os.UserHomeDir()
	if !strings.Contains(string(env), "COPILOT_GITHUB_TOKEN=copilot-tok\n") || strings.Contains(string(env), "must-not-leak") ||
		strings.Contains(string(env), "HOME="+home+"\n") || !strings.Contains(string(env), "COPILOT_AUTO_UPDATE=false\n") {
		t.Errorf("environment:\n%s", env)
	}
	if wd, _ := os.ReadFile(rec + ".pwd"); !strings.Contains(string(wd), "aayu-copilot-") {
		t.Errorf("ran in %s, want a fresh temporary directory", wd)
	}
	// A retry's turns are labelled in the one prompt.
	req.Messages = append(req.Messages, Message{Role: "assistant", Content: "nope"}, Message{Role: "user", Content: "Again."})
	if _, err := (CopilotCLI{Token: "t", Path: dir}).Complete(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if args, _ := os.ReadFile(rec + ".args"); !strings.Contains(string(args), "User:\nState \"s\".\n\nAssistant:\nnope\n\nUser:\nAgain.") {
		t.Errorf("retry prompt:\n%s", args)
	}

	failing, _ := fakeCopilot(t, 1, true)
	unmetered, _ := fakeCopilot(t, 0, false)
	for _, tc := range []struct {
		c    CopilotCLI
		want string
	}{
		{CopilotCLI{Token: "t", Path: t.TempDir()}, "not installed"},
		{CopilotCLI{Path: dir}, "not signed in"},
		{CopilotCLI{Token: "t", Path: failing}, "auth failed for this account"},
		{CopilotCLI{Token: "t", Path: unmetered}, "no usage report"},
	} {
		if _, err := tc.c.Complete(context.Background(), req); !errors.Is(err, ErrCopilot) || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("want ErrCopilot mentioning %q, got %v", tc.want, err)
		}
	}
	// A reply with no usage report is charged an upper bound, not nothing:
	// more than the recorded call's 0.043 USD, so the breaker sees the spend.
	if resp, _ := (CopilotCLI{Token: "t", Path: unmetered}).Complete(context.Background(), req); resp.CostUSD < 0.1 {
		t.Errorf("unmetered reply charged %v USD", resp.CostUSD)
	}
}
