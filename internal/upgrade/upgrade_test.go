package upgrade

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// copyTree copies testdata/<name> into a temp dir.
func copyTree(t *testing.T, name string) string {
	t.Helper()
	dst := t.TempDir()
	src := filepath.Join("testdata", name)
	err := filepath.Walk(src, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		_ = os.MkdirAll(filepath.Join(dst, filepath.Dir(rel)), 0o755)
		return os.WriteFile(filepath.Join(dst, rel), b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

// A hypothetical v2 binary: config renames sla_hours, state renames labels,
// lock only bumps its version.
var v2 = Versions{KindConfig: 2, KindLock: 2, KindState: 2}

var v2Migrations = Migrations{
	KindConfig: {1: func(b []byte) ([]byte, error) {
		return bytes.ReplaceAll(b, []byte("\nsla_hours ="), []byte("\npage_after_hours =")), nil
	}},
	KindState: {1: func(b []byte) ([]byte, error) {
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			return nil, err
		}
		m["label_ids"] = m["labels"]
		delete(m, "labels")
		return json.Marshal(m)
	}},
}

func TestUpgradeV1ToV2ByteExact(t *testing.T) {
	root := copyTree(t, "v1")
	changes, err := Plan(root, v2, v2Migrations)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := os.ReadFile("testdata/v1-to-v2.diff")
	if got := Diff(changes); got != string(want) {
		t.Fatalf("diff differs from testdata/v1-to-v2.diff:\n%s", got)
	}
	// The diff is shown first; nothing is written until Apply.
	if b, _ := os.ReadFile(filepath.Join(root, ".aayudha/config.toml")); !bytes.Contains(b, []byte("version = 1")) {
		t.Fatal("Plan wrote files")
	}
	if err := Apply(root, changes); err != nil {
		t.Fatal(err)
	}
	if again, err := Plan(root, v2, v2Migrations); err != nil || len(again) != 0 {
		t.Fatalf("second run: %d changes, %v; want none", len(again), err)
	}
}

func TestNewerRepoIsRefused(t *testing.T) {
	root := copyTree(t, "v1")
	cfg := filepath.Join(root, ".aayudha/config.toml")
	b, _ := os.ReadFile(cfg)
	_ = os.WriteFile(cfg, bytes.Replace(b, []byte("version = 1"), []byte("version = 3"), 1), 0o644)
	if err := Check(root, v2); !errors.Is(err, ErrNewer) || !strings.Contains(err.Error(), "v3, this aayu supports up to v2") {
		t.Fatalf("v3 repo on a v2 binary: %v", err)
	}
	if err := Check(copyTree(t, "v1"), v2); err != nil {
		t.Fatalf("v1 repo on a v2 binary: %v", err)
	}
}

func TestConfigV0Migration(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, ".aayudha"), 0o755)
	v0 := "# product config\n\n[lfs]\nserver = \"github\" # plan v0.1\n\n[budget]\nper_day_usd = 5.0\n"
	_ = os.WriteFile(filepath.Join(root, ".aayudha/config.toml"), []byte(v0), 0o644)
	changes, err := Plan(root, Current, Default)
	if err != nil || len(changes) != 1 {
		t.Fatalf("%v %v", changes, err)
	}
	want := "# product config\n\nversion = 1\n\n[lfs]\n\n[budget]\nper_day_usd = 5.0\n"
	if string(changes[0].New) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", changes[0].New, want)
	}
	_ = os.WriteFile(filepath.Join(root, ".aayudha/config.toml"), []byte(strings.Replace(v0, `"github"`, `"s3://b"`, 1)), 0o644)
	if _, err := Plan(root, Current, Default); err == nil || !strings.Contains(err.Error(), "only GitHub LFS") {
		t.Fatalf("S3 server: %v", err)
	}
}
