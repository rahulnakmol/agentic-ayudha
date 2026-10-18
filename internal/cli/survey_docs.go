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
// PDF is converted with pdftotext when it is installed. Man pages also come
// as their section number (manPage).
var manualExts = map[string]bool{".txt": true, ".text": true, ".md": true, ".markdown": true, ".rst": true, ".adoc": true,
	".html": true, ".htm": true, ".man": true, ".pdf": true}

// manPage reports whether ext is a man page's section suffix: a section
// digit, then any letters, as in .1, .3p, .5, or .8.
func manPage(ext string) bool {
	if len(ext) < 2 || ext[1] < '1' || ext[1] > '9' {
		return false
	}
	return strings.Trim(ext[2:], "abcdefghijklmnopqrstuvwxyz") == ""
}

// surveyManuals reads the user manuals and notes at path (a file or a
// directory) for the index, and writes the exact text each is cited by to
// dir/<name>, so `manual:<name>:<line>` points at a line anyone can open. A
// PDF becomes <name>.txt through pdftotext; a file of another format is
// refused with the formats it reads.
func surveyManuals(ctx context.Context, path, dir string) (map[string]string, error) {
	if path == "" {
		return nil, nil
	}
	info, err := os.Lstat(path)
	if err == nil && info.Mode()&fs.ModeSymlink != 0 {
		info, err = os.Stat(path) // a linked directory the user named is read; a linked file is refused below
		if err == nil && !info.IsDir() {
			return nil, fmt.Errorf("survey --docs: %s is a symbolic link; name the file it points to", path)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("survey --docs: %w", err)
	}
	var files []string
	base := filepath.Dir(path)
	if info.IsDir() {
		// WalkDir does not follow a linked root: walk the directory it names.
		if base, err = filepath.EvalSymlinks(path); err != nil {
			return nil, fmt.Errorf("survey --docs: %w", err)
		}
		err = filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
			switch {
			case err != nil:
				return err
			case p == base:
				return nil
			case strings.HasPrefix(d.Name(), "."):
				if d.IsDir() {
					return filepath.SkipDir // .git and the like
				}
				return nil
			case d.IsDir():
				return nil
			case !d.Type().IsRegular():
				// A link (or device, or pipe) in the manuals tree could
				// copy any file the user can read into model input.
				return fmt.Errorf("survey --docs: %s is not a regular file (a link or special file); the survey reads only real files", p)
			}
			files = append(files, p)
			return nil
		})
		if err != nil {
			return nil, err
		}
	} else if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("survey --docs: %s is not a regular file", path)
	} else {
		files = []string{path}
	}
	// Each manual's name, checked for every file before any is read or
	// converted: guide.pdf becomes guide.pdf.txt, which another file may be.
	names, from := map[string]string{}, map[string]string{}
	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f))
		if !manualExts[ext] && !manPage(ext) {
			return nil, fmt.Errorf("survey --docs: %s: not a format it reads (text, Markdown, reStructuredText, AsciiDoc, HTML, man pages, or PDF through pdftotext)", f)
		}
		rel, _ := filepath.Rel(base, f)
		name := filepath.ToSlash(rel)
		if ext == ".pdf" {
			name += ".txt"
		}
		if prev, dup := from[name]; dup {
			return nil, fmt.Errorf("survey --docs: %s and %s would both be manual:%s; rename one", prev, f, name)
		}
		from[name], names[f] = f, name
	}
	out := map[string]string{}
	for _, f := range files {
		ext, name := strings.ToLower(filepath.Ext(f)), names[f]
		var text []byte
		if ext == ".pdf" {
			if _, err := exec.LookPath("pdftotext"); err != nil {
				return nil, fmt.Errorf("survey --docs: %s is a PDF and pdftotext is not installed; install poppler-utils or convert it to text first", f)
			}
			if text, err = exec.CommandContext(ctx, "pdftotext", "-layout", f, "-").Output(); err != nil {
				return nil, fmt.Errorf("survey --docs: pdftotext %s: %w", f, err)
			}
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
