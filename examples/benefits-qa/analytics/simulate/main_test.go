package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	out := filepath.Join(t.TempDir(), "x.jsonl")
	var stderr bytes.Buffer
	if err := run([]string{"-channel", "hr-ticket", "-seed", "1", "-from", "2026-08-10", "-out", out}, &bytes.Buffer{}, &stderr); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(out)
	want, _ := os.ReadFile(filepath.Join("..", "testdata", "hr-ticket-seed1-2026-08-10.jsonl"))
	if !bytes.Equal(got, want) || !strings.Contains(stderr.String(), "hr-ticket, seed 1: 120 users") {
		t.Errorf("run wrote %d bytes, want the committed baseline; %s", len(got), stderr.String())
	}
	var stdout bytes.Buffer
	if err := run([]string{"-from", "2026-09-14", "-users", "3"}, &stdout, &stderr); err != nil || !strings.Contains(stdout.String(), "benefits_visit") {
		t.Errorf("stdout run: %v %q", err, stdout.String())
	}
	for _, args := range [][]string{{"-from", "yesterday"}, {"-from", "2026-09-14", "-channel", "fax"}, {"-nope"}, {"-from", "2026-09-14", "-out", filepath.Join(t.TempDir(), "no", "dir", "x")}} {
		if err := run(args, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
			t.Errorf("%v accepted", args)
		}
	}
}
