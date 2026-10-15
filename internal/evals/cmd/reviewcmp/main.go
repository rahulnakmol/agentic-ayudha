// Command reviewcmp is a research harness (scratch branch only, docs/research/verification-skills-2026-10.md):
// it runs inspect's real prompt and post-processing with a reply from an outside model, and scores OCR's
// delegation-mode comments with the same matcher, so both pipelines are measured on one model.
//
//	reviewcmp prep <case-dir> <out-dir>          inspect's system prompt and user prompt for the case
//	reviewcmp inspect <case-dir> <reply.json>    run inspect's Runner on the reply (filters, quote placement) and score
//	reviewcmp ocr <case-dir> <comments.json>     score OCR delegation-mode comments
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/evals"
	"github.com/rahulnakmol/agentic-ayudha/internal/inspect"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: reviewcmp prep|inspect|ocr <case-dir> <file>")
		os.Exit(2)
	}
	ctx := context.Background()
	c, err := evals.LoadCase(os.Args[2])
	check(err)
	in, err := evals.ReviewInput(ctx, c)
	check(err)
	in.Mode = inspect.ModeGeneral
	switch os.Args[1] {
	case "prep":
		out := os.Args[3]
		check(os.MkdirAll(out, 0o755))
		sys := inspect.System + "\n\nReply with one JSON value and nothing else. It must validate against this JSON Schema:\n" + inspect.Schema
		check(os.WriteFile(filepath.Join(out, "system.txt"), []byte(sys), 0o644))
		check(os.WriteFile(filepath.Join(out, "prompt.txt"), []byte(inspect.Prompt(in)), 0o644))
		fmt.Printf("{\"system_chars\":%d,\"prompt_chars\":%d}\n", len(sys), len(inspect.Prompt(in)))
	case "inspect":
		reply, err := os.ReadFile(os.Args[3])
		check(err)
		r := inspect.Reviewer{Model: "outside", Complete: func(context.Context, models.Request) (models.Response, float64, error) {
			return models.Response{Text: string(reply)}, 0, nil
		}}
		fl := inspect.Filter{MinConfidence: 0.5, SuppressAfter: 2, Generated: []string{"go.sum"}}
		res, _, err := inspect.Runner{Reviewers: []inspect.Reviewer{r}, Filter: fl}.Run(ctx, in, inspect.State{})
		check(err)
		emit(c, res.Findings, map[string]any{"disposition": res.Disposition, "dropped": res.Dropped})
	case "ocr":
		b, err := os.ReadFile(os.Args[3])
		check(err)
		var raw struct {
			Comments []struct {
				Path      string `json:"path"`
				Content   string `json:"content"`
				StartLine int    `json:"start_line"`
				EndLine   int    `json:"end_line"`
				Category  string `json:"category"`
				Severity  string `json:"severity"`
			} `json:"comments"`
			Summary map[string]any `json:"summary"`
		}
		check(json.Unmarshal(b, &raw))
		sev := map[string]string{"critical": "P0", "high": "P1", "medium": "P2", "low": "P3"}
		var fs, noLow []inspect.Finding
		for _, x := range raw.Comments {
			title, _, _ := strings.Cut(x.Content, ". ")
			f := inspect.Finding{File: x.Path, Line: x.StartLine, Severity: sev[x.Severity], Category: x.Category, Title: title, Detail: x.Content}
			fs = append(fs, f)
			if x.Severity != "low" {
				noLow = append(noLow, f)
			}
		}
		tp, fp, fn := evals.ScoreFindings(noLow, c.Review.Defects)
		emit(c, fs, map[string]any{"coverage": raw.Summary, "without_low": map[string]int{"tp": tp, "fp": fp, "fn": fn}})
	}
}

func emit(c evals.Case, fs []inspect.Finding, extra map[string]any) {
	tp, fp, fn := evals.ScoreFindings(fs, c.Review.Defects)
	type row struct {
		Sev, File, Title string
		Line             int
	}
	var rows []row
	for _, f := range fs {
		rows = append(rows, row{f.Severity, f.File, f.Title, f.Line})
	}
	out := map[string]any{"case": filepath.Base(c.Dir), "planted": len(c.Review.Defects), "posted": len(fs), "tp": tp, "fp": fp, "fn": fn, "findings": rows}
	for k, v := range extra {
		out[k] = v
	}
	b, _ := json.Marshal(out)
	fmt.Println(string(b))
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "reviewcmp:", err)
		os.Exit(1)
	}
}
