package models

import (
	"bytes"
	"context"
	"errors"
	"image"
	pngenc "image/png"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
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
cat > "$rec.stdin"
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

// TestCopilotCLI: the opt-in Copilot route runs Copilot CLI with the prompt
// on stdin, the request's model and effort, the image as an attachment, no tools, in an empty
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
	if in, _ := os.ReadFile(rec + ".stdin"); string(in) != "You are lane A.\n\nState \"s\"." {
		t.Errorf("stdin %q, want the prompt", in)
	}
	if strings.Contains(string(args), "lane A") || strings.Contains(string(args), "-p\n") {
		t.Errorf("the prompt is on the command line:\n%s", args)
	}
	for _, want := range []string{"-s\n", "--model\ngemini-4-argon\n", "--reasoning-effort\nhigh\n", "--available-tools=\n", "--no-ask-user\n", "--no-custom-instructions\n", "--output-format\njson\n"} {
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
	if args, _ := os.ReadFile(rec + ".stdin"); !strings.Contains(string(args), "User:\nState \"s\".\n\nAssistant:\nnope\n\nUser:\nAgain.") {
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
	// A reply with no usage report is charged upper-bound tokens, priced as any
	// call. The request's 8k output cap does not bound it, since copilot never
	// receives it: 128k output tokens at Argon's 10 USD per million is 1.28 USD.
	req.MaxTokens = 8000
	resp, _ = (CopilotCLI{Token: "t", Path: unmetered}).Complete(context.Background(), req)
	if usd, _ := Cost(nil, req.Model, resp); usd < 1.28 {
		t.Errorf("unmetered reply charged %v USD", usd)
	}
	// A configured price applies to it: at 100 USD per million both ways it is
	// over 12.8 USD.
	if usd, src := Cost([]config.Price{{Model: req.Model, InputPerMTok: 100, OutputPerMTok: 100}}, req.Model, resp); usd < 12.8 || src != CostPriceTable {
		t.Errorf("unmetered reply with a configured price charged %v USD from %s", usd, src)
	}
	// A copilot that dies with no reply and no usage is charged the same.
	resp, err = (CopilotCLI{Token: "t", Bin: "/bin/false", Path: unmetered}).Complete(context.Background(), req)
	if usd, _ := Cost(nil, req.Model, resp); !errors.Is(err, ErrCopilot) || usd < 1.28 {
		t.Errorf("a copilot that died charged %v USD (%v)", usd, err)
	}
	// One that never starts (not an executable) spent nothing.
	junk := filepath.Join(t.TempDir(), "copilot")
	if err := os.WriteFile(junk, []byte("not a program"), 0o755); err != nil {
		t.Fatal(err)
	}
	if resp, err := (CopilotCLI{Token: "t", Bin: junk, Path: unmetered}).Complete(context.Background(), req); !errors.Is(err, ErrCopilot) || resp.TokensIn+resp.TokensOut != 0 {
		t.Errorf("a copilot that never started: %d+%d tokens (%v)", resp.TokensIn, resp.TokensOut, err)
	}
	// The bound counts a token per prompt byte, 25k for Copilot's own prompt,
	// and a token per 750 pixels of each attachment: 1 + 25,000 + 3000×1500/750.
	var pic bytes.Buffer
	if err := pngenc.Encode(&pic, image.NewGray(image.Rect(0, 0, 3000, 1500))); err != nil {
		t.Fatal(err)
	}
	wide := Request{Model: req.Model, System: "s", Messages: []Message{{Role: "user", Images: []Image{{MediaType: "image/png", Data: pic.Bytes()}}}}}
	if resp, _ := (CopilotCLI{Token: "t", Path: unmetered}).Complete(context.Background(), wide); resp.TokensIn != 1+25000+6000 {
		t.Errorf("unmetered input bound %d tokens, want 31001", resp.TokensIn)
	}
	// It is priced at the long-context rate past the threshold: a 100k-byte
	// prompt is 100k + 25k = 125k input tokens, over Haiku 5.5's 100k, so
	// 125k × 0.50 + 128k × 2.50 per million = 0.3825 USD.
	big := Request{Model: "claude-haiku-5-5", System: strings.Repeat("x", 100000)}
	resp, _ = (CopilotCLI{Token: "t", Path: unmetered}).Complete(context.Background(), big)
	if usd, _ := Cost(nil, big.Model, resp); math.Abs(usd-0.3825) > 1e-9 {
		t.Errorf("long-context unmetered reply charged %v USD, want 0.3825", usd)
	}
}
