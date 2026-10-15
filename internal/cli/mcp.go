package cli

import (
	"context"
	"errors"
	"io"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rahulnakmol/agentic-ayudha/internal/mcp"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

func init() { register("mcp", runMCP) }

// agentRunner runs verbs in-process as an agent lane: AAYU_AGENT=1, so --yes,
// self-update, install --apply, and other human-only paths are refused, and no
// stdin is available (MCP clients cannot answer prompts).
func (e *Env) agentRunner() mcp.Runner {
	opts := e.Options
	get := opts.Getenv
	opts.Getenv = func(k string) string {
		if k == "AAYU_AGENT" {
			return "1"
		}
		return get(k)
	}
	opts.Environ = append(append([]string(nil), opts.Environ...), "AAYU_AGENT=1")
	opts.Dir = e.Dir
	return func(ctx context.Context, args []string, stdout, stderr io.Writer) int {
		full := append(append([]string{args[0]}, globalArgs(e)...), args[1:]...) // before any "--"
		return Run(ctx, full, IO{In: strings.NewReader(""), Out: stdout, Err: stderr}, opts)
	}
}

// globalArgs carries the server's own --repo and --set overrides into each call;
// they go right after the command name, ahead of the "--" before positionals.
func globalArgs(e *Env) []string {
	var out []string
	if e.Repo != "" {
		out = append(out, "--repo="+e.Repo)
	}
	for _, s := range e.Set {
		out = append(out, "--set="+s)
	}
	return out
}

// runMCP serves the verbs over stdio (WP3.3). Stdout carries only protocol
// frames; diagnostics go to stderr.
func runMCP(ctx context.Context, e *Env, in Values) error {
	if v := in.String("skills"); v != "" && v != verbs.SkillVersion {
		return WithNext(Errorf(ExitExternal, "mcp: the host plugin carries skills %s, and this aayu carries %s; the server does not start with mismatched skills", v, verbs.SkillVersion), "aayu self-update")
	}
	l, err := e.Config()
	if err != nil {
		return err
	}
	stage, why := l.Config.TeamStage()
	s, err := mcp.NewServer(mcp.Options{Version: e.Build.Version, ExposeWrites: l.Config.MCP.ExposeWrites, Run: e.agentRunner(), Assets: e.Assets,
		TeamStage: stage, TeamStageWhy: why})
	if err != nil {
		return err
	}
	err = s.Run(ctx, &sdk.IOTransport{Reader: io.NopCloser(e.In), Writer: nopWriteCloser{e.Out}})
	if err == nil || errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
		return nil
	}
	return Wrap(ExitExternal, err)
}

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }
