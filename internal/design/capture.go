package design

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rahulnakmol/agentic-ayudha/internal/verify"
)

//go:embed capture/capture.mjs
var captureScript []byte

// BrowserCapture renders states with the embedded Playwright script under
// node in the verify sandbox (scrubbed environment; the prototype is a network
// peer). Playwright resolves from root, then NODE_PATH.
func BrowserCapture(sb *verify.Sandbox, root, work string) Capture {
	return func(ctx context.Context, states []CaptureState) ([]Captured, error) {
		script := filepath.Join(work, "aayudha-capture.mjs")
		if err := os.WriteFile(script, captureScript, 0o644); err != nil {
			return nil, err
		}
		out := filepath.Join(work, "captures")
		in, _ := json.Marshal(map[string]any{"states": states, "out": out})
		o, err := sb.Run(ctx, verify.Cmd{Dir: root, Name: "node", Args: []string{script}, Stdin: in, Network: true})
		if err != nil {
			return nil, err
		}
		var caps []Captured
		if err := json.Unmarshal(o.Stdout, &caps); err != nil {
			return nil, fmt.Errorf("prototype capture exited %d: %s", o.Code, tail(o.Stderr))
		}
		for i := range caps {
			if caps[i].Error == "" {
				if caps[i].data, err = os.ReadFile(filepath.Join(out, caps[i].PNG)); err != nil {
					caps[i].Error = err.Error()
				}
			}
		}
		return caps, nil
	}
}

func tail(b []byte) string {
	if len(b) > 400 {
		b = b[len(b)-400:]
	}
	return string(b)
}
