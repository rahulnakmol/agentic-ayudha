package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeVerifier struct{ sigErr, provErr error }

func (f fakeVerifier) VerifySignature(context.Context, string, string, string) error { return f.sigErr }
func (f fakeVerifier) VerifyProvenance(context.Context, string) error                { return f.provErr }

// releaseFixture serves a GitHub "latest release" for tag with the given assets.
// The archive holds a fake aayudha binary whose content is newBinary.
func releaseFixture(t *testing.T, tag string, signed bool, tamper bool) *httptest.Server {
	t.Helper()
	archiveName := fmt.Sprintf("aayudha_%s_linux_amd64.tar.gz", strings.TrimPrefix(tag, "v"))
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{Name: "aayudha", Mode: 0o755, Size: int64(len(newBinary)), Typeflag: tar.TypeReg})
	_, _ = tw.Write([]byte(newBinary))
	_ = tw.Close()
	_ = gz.Close()
	archive := buf.Bytes()
	sum := sha256.Sum256(archive)
	checksums := hex.EncodeToString(sum[:]) + "  " + archiveName + "\n"
	if tamper {
		archive = append([]byte{}, archive...)
		archive[len(archive)/2] ^= 0xff
	}
	assets := map[string][]byte{archiveName: archive, "checksums.txt": []byte(checksums)}
	if signed {
		assets["checksums.txt.sigstore.json"] = []byte(`{"mediaType":"application/vnd.dev.sigstore.bundle.v0.3+json"}`)
	}

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/rahulnakmol/agentic-ayudha/releases/latest" {
			type asset struct {
				Name string `json:"name"`
				URL  string `json:"browser_download_url"`
			}
			var list []asset
			for name := range assets {
				list = append(list, asset{name, srv.URL + "/dl/" + name})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"tag_name": tag, "assets": list})
			return
		}
		if b, ok := assets[strings.TrimPrefix(r.URL.Path, "/dl/")]; ok {
			_, _ = w.Write(b)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

const (
	oldBinary = "old aayudha"
	newBinary = "new aayudha"
)

func selfUpdate(t *testing.T, srv *httptest.Server, v Verifier, version string, env map[string]string, exeDir string) (int, string, string) {
	t.Helper()
	exe := filepath.Join(exeDir, "aayudha")
	if err := os.WriteFile(exe, []byte(oldBinary), 0o755); err != nil {
		t.Fatal(err)
	}
	code, out, errs := run(t, env, Options{
		Build:      BuildInfo{Version: version},
		Executable: func() (string, error) { return exe, nil },
		SelfUpdate: SelfUpdateOptions{APIBase: srv.URL, Verifier: v, GOOS: "linux", GOARCH: "amd64"},
	}, "self-update")
	b, _ := os.ReadFile(exe)
	return code, string(b), out + errs
}

func TestSelfUpdate(t *testing.T) {
	tests := []struct {
		name     string
		tag      string
		signed   bool
		tamper   bool
		verifier Verifier
		version  string
		env      map[string]string
		wantCode int
		wantBin  string
		wantMsg  string
	}{
		{"unsigned release is rejected", "v0.2.0", false, false, fakeVerifier{}, "0.1.0", nil, ExitBlocked, oldBinary, "not signed"},
		{"bad signature is rejected", "v0.2.0", true, false, fakeVerifier{sigErr: errors.New("certificate identity mismatch")}, "0.1.0", nil, ExitBlocked, oldBinary, "signature verification failed"},
		{"tampered archive is rejected", "v0.2.0", true, true, fakeVerifier{}, "0.1.0", nil, ExitBlocked, oldBinary, "checksum mismatch"},
		{"missing provenance is rejected", "v0.2.0", true, false, fakeVerifier{provErr: errors.New("no attestations")}, "0.1.0", nil, ExitBlocked, oldBinary, "provenance"},
		{"downgrade is refused", "v0.1.0", true, false, fakeVerifier{}, "0.2.0", nil, ExitBlocked, oldBinary, "refusing to downgrade"},
		{"prerelease is older than its release", "v0.2.0-rc.1", true, false, fakeVerifier{}, "0.2.0", nil, ExitBlocked, oldBinary, "refusing to downgrade"},
		{"same version is a no-op", "v0.2.0", true, false, fakeVerifier{}, "0.2.0", nil, ExitOK, oldBinary, "is the latest release"},
		{"agents are refused", "v0.2.0", true, false, fakeVerifier{}, "0.1.0", map[string]string{"AAYU_AGENT": "1"}, ExitBlocked, oldBinary, "AAYU_AGENT"},
		{"signed newer release is installed", "v0.2.0", true, false, fakeVerifier{}, "0.1.0", nil, ExitOK, newBinary, "0.1.0 -> v0.2.0"},
		{"dev build updates to a release", "v0.2.0", true, false, fakeVerifier{}, "0.0.0-dev", nil, ExitOK, newBinary, "updated"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := releaseFixture(t, tt.tag, tt.signed, tt.tamper)
			code, bin, msg := selfUpdate(t, srv, tt.verifier, tt.version, tt.env, t.TempDir())
			if code != tt.wantCode || bin != tt.wantBin || !strings.Contains(msg, tt.wantMsg) {
				t.Fatalf("exit %d, binary %q, output %q; want exit %d, binary %q, output containing %q",
					code, bin, msg, tt.wantCode, tt.wantBin, tt.wantMsg)
			}
		})
	}
}

func TestSelfUpdateDefersToPackageManagers(t *testing.T) {
	for dir, want := range map[string]string{
		"/opt/homebrew/Cellar/aayudha/0.1.0/bin":                     "brew upgrade aayudha",
		`C:\Users\a\scoop\apps\aayudha\current`:                      "scoop update aayudha",
		`C:\Users\a\AppData\Local\Microsoft\WinGet\Packages\aayudha`: "winget upgrade aayudha",
	} {
		if _, cmd := packageManager(dir + "/aayudha"); cmd != want {
			t.Errorf("packageManager(%s) = %q, want %q", dir, cmd, want)
		}
	}
	if pm, _ := packageManager("/home/a/.local/bin/aayudha"); pm != "" {
		t.Errorf("a plain install was attributed to %s", pm)
	}
}
