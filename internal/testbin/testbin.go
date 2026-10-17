// Package testbin builds fixture programs for tests that run a program the
// way a product does, by its path. A `#!/bin/sh` script runs that way only
// where the OS honours a shebang; Windows does not ("%1 is not a valid Win32
// application"). A one-file Go program built here runs on every OS.
package testbin

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// Exe is the file-name suffix of an executable on this OS.
var Exe = map[bool]string{true: ".exe", false: ""}[runtime.GOOS == "windows"]

// Build compiles src, a Go package main that imports only the standard
// library, into dir/name (name.exe on Windows) and returns its path.
func Build(t testing.TB, dir, name, src string) string {
	t.Helper()
	srcDir := t.TempDir()
	main := filepath.Join(srcDir, "main.go")
	if err := os.WriteFile(main, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, name+Exe)
	cmd := exec.Command("go", "build", "-o", out, main)
	cmd.Dir = srcDir
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fixture %s: %v\n%s", name, err, b)
	}
	return out
}
