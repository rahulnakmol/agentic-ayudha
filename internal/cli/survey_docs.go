package cli

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// manualExts are the manual and note formats survey --docs reads as text; a
// PDF is converted with pdftotext when it is installed.
var manualExts = map[string]bool{".txt": true, ".text": true, ".md": true, ".markdown": true, ".rst": true, ".adoc": true,
	".html": true, ".htm": true, ".1": true, ".man": true, ".pdf": true}

// surveyManuals reads the user manuals and notes at path (a file or a
// directory) for the index, and writes the exact text each is cited by to
// dir/<name>, so `manual:<name>:<line>` points at a line anyone can open. A
// PDF becomes <name>.txt through pdftotext; a file of another format is
// refused with the formats it reads.
func surveyManuals(ctx context.Context, path, dir string) (map[string]string, error) {
	if path == "" {
		return nil, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("survey --docs: %w", err)
	}
	var files []string
	base := filepath.Dir(path)
	if info.IsDir() {
		base = path
		err = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && !strings.HasPrefix(d.Name(), ".") {
				files = append(files, p)
			}
			return err
		})
		if err != nil {
			return nil, err
		}
	} else {
		files = []string{path}
	}
	out := map[string]string{}
	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f))
		if !manualExts[ext] {
			return nil, fmt.Errorf("survey --docs: %s: not a format it reads (text, Markdown, reStructuredText, AsciiDoc, HTML, man pages, or PDF through pdftotext)", f)
		}
		rel, _ := filepath.Rel(base, f)
		name := filepath.ToSlash(rel)
		var text []byte
		if ext == ".pdf" {
			if _, err := exec.LookPath("pdftotext"); err != nil {
				return nil, fmt.Errorf("survey --docs: %s is a PDF and pdftotext is not installed; install poppler-utils or convert it to text first", f)
			}
			if text, err = exec.CommandContext(ctx, "pdftotext", "-layout", f, "-").Output(); err != nil {
				return nil, fmt.Errorf("survey --docs: pdftotext %s: %w", f, err)
			}
			name += ".txt"
		} else if text, err = os.ReadFile(f); err != nil {
			return nil, err
		}
		if !utf8.Valid(text) {
			return nil, fmt.Errorf("survey --docs: %s is not UTF-8 text", f)
		}
		dst := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(dst, text, 0o644); err != nil {
			return nil, err
		}
		out[name] = strings.ReplaceAll(string(text), "\r\n", "\n")
	}
	return out, nil
}
