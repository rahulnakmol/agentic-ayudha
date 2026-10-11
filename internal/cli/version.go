package cli

import (
	"context"
	"fmt"
	"io"
	"runtime"
)

func init() { register("version", runVersion) }

func runVersion(_ context.Context, e *Env, _ Values) error {
	v := map[string]string{
		"version":  e.Build.Version,
		"commit":   e.Build.Commit,
		"date":     e.Build.Date,
		"go":       runtime.Version(),
		"platform": runtime.GOOS + "/" + runtime.GOARCH,
	}
	return e.Emit(v, func(w io.Writer) error {
		_, err := fmt.Fprintf(w, "aayu %s\n", e.Build.Version)
		if e.Verbose {
			fmt.Fprintf(w, "commit %s, built %s, %s, %s\n", orNone(e.Build.Commit), orNone(e.Build.Date), v["go"], v["platform"])
		}
		return err
	})
}

func orNone(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}
