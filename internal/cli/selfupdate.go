package cli

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// Release identity. The release workflow (WP0.4) signs checksums.txt keylessly
// with cosign and attests build provenance; self-update accepts only artifacts
// whose signature certificate names this repository's release workflow running
// on exactly the tag being installed, as install.sh does.
const (
	releaseRepo        = "rahulnakmol/agentic-ayudha"
	releaseIdentityRE  = `^https://github\.com/rahulnakmol/agentic-ayudha/\.github/workflows/release\.yml@refs/tags/` // + the exact tag
	releaseOIDCIssuer  = "https://token.actions.githubusercontent.com"
	releaseWorkflow    = releaseRepo + "/.github/workflows/release.yml"
	checksumsAsset     = "checksums.txt"
	checksumsSignature = "checksums.txt.sigstore.json" // cosign bundle: signature, certificate, and Rekor entry
)

// SelfUpdateOptions lets tests point self-update at a fixture release server and
// a fake verifier.
type SelfUpdateOptions struct {
	APIBase  string   // default https://api.github.com
	Verifier Verifier // default: the cosign and gh CLIs
	GOOS     string
	GOARCH   string
}

// Verifier checks a downloaded release before it replaces the running binary.
type Verifier interface {
	// VerifySignature checks the cosign bundle over the checksums file against
	// the release workflow identity on exactly tag.
	VerifySignature(ctx context.Context, tag, checksums, bundle string) error
	// VerifyProvenance checks the SLSA build provenance attestation of the archive.
	VerifyProvenance(ctx context.Context, archive string) error
}

func init() { register("self-update", runSelfUpdate) }

type ghRelease struct {
	TagName    string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func runSelfUpdate(ctx context.Context, e *Env, _ Values) error {
	if err := e.requireHuman("self-update"); err != nil {
		return err
	}
	exe, err := e.Executable()
	if err != nil {
		return Wrap(ExitInternal, fmt.Errorf("locate the running binary: %w", err))
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	if pm, cmd := packageManager(exe); pm != "" {
		return e.Emit(map[string]string{"package_manager": pm, "command": cmd}, func(w io.Writer) error {
			_, err := fmt.Fprintf(w, "aayu was installed with %s; upgrade it with:\n  %s\n", pm, cmd)
			return err
		})
	}

	o := e.SelfUpdate
	if o.APIBase == "" {
		o.APIBase = "https://api.github.com"
	}
	if o.Verifier == nil {
		o.Verifier = toolVerifier{}
	}
	if o.GOOS == "" {
		o.GOOS = runtime.GOOS
	}
	if o.GOARCH == "" {
		o.GOARCH = runtime.GOARCH
	}

	var rel ghRelease
	if err := getJSON(ctx, e.HTTP, o.APIBase+"/repos/"+releaseRepo+"/releases/latest", &rel); err != nil {
		return Wrap(ExitExternal, fmt.Errorf("fetch the latest release: %w", err))
	}
	if rel.Draft || rel.Prerelease {
		return Errorf(ExitExternal, "latest release %s is a draft or prerelease; refusing", rel.TagName)
	}
	latest, err := parseSemver(rel.TagName)
	if err != nil {
		return Wrap(ExitExternal, fmt.Errorf("latest release tag: %w", err))
	}
	current, err := parseSemver(e.Build.Version)
	if err != nil {
		return Wrap(ExitInternal, fmt.Errorf("running version: %w", err))
	}
	switch c := latest.compare(current); {
	case c < 0:
		return Errorf(ExitBlocked, "latest release %s is older than the running %s; refusing to downgrade", rel.TagName, e.Build.Version)
	case c == 0:
		return e.Emit(map[string]string{"version": e.Build.Version, "status": "up-to-date"}, func(w io.Writer) error {
			_, err := fmt.Fprintf(w, "aayu %s is the latest release\n", e.Build.Version)
			return err
		})
	}

	ext := "tar.gz"
	if o.GOOS == "windows" {
		ext = "zip"
	}
	archiveName := fmt.Sprintf("aayudha_%s_%s_%s.%s", strings.TrimPrefix(rel.TagName, "v"), o.GOOS, o.GOARCH, ext)
	urls := map[string]string{}
	for _, a := range rel.Assets {
		urls[a.Name] = a.URL
	}
	if urls[checksumsAsset] == "" || urls[checksumsSignature] == "" {
		return Errorf(ExitBlocked, "release %s is not signed (no %s and %s); refusing to install it", rel.TagName, checksumsAsset, checksumsSignature)
	}
	if urls[archiveName] == "" {
		return Errorf(ExitExternal, "release %s has no archive for %s/%s (%s)", rel.TagName, o.GOOS, o.GOARCH, archiveName)
	}

	dir, err := os.MkdirTemp("", "aayu-self-update-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	files := map[string]string{}
	for _, name := range []string{checksumsAsset, checksumsSignature, archiveName} {
		files[name] = filepath.Join(dir, name)
		if err := download(ctx, e.HTTP, urls[name], files[name]); err != nil {
			return Wrap(ExitExternal, fmt.Errorf("download %s: %w", name, err))
		}
	}

	if err := o.Verifier.VerifySignature(ctx, rel.TagName, files[checksumsAsset], files[checksumsSignature]); err != nil {
		return Wrap(ExitBlocked, fmt.Errorf("release %s: signature verification failed; refusing to install it: %w", rel.TagName, err))
	}
	if err := checkSum(files[checksumsAsset], files[archiveName], archiveName); err != nil {
		return Wrap(ExitBlocked, fmt.Errorf("release %s: %w; refusing to install it", rel.TagName, err))
	}
	if err := o.Verifier.VerifyProvenance(ctx, files[archiveName]); err != nil {
		return Wrap(ExitBlocked, fmt.Errorf("release %s: provenance verification failed; refusing to install it: %w", rel.TagName, err))
	}

	if e.DryRun {
		return e.Emit(map[string]string{"from": e.Build.Version, "to": rel.TagName, "status": "verified-dry-run"}, func(w io.Writer) error {
			_, err := fmt.Fprintf(w, "verified %s; --dry-run: not replacing %s\n", rel.TagName, exe)
			return err
		})
	}
	bin := "aayudha"
	if o.GOOS == "windows" {
		bin += ".exe"
	}
	if err := replaceBinary(files[archiveName], bin, exe); err != nil {
		return Wrap(ExitExternal, fmt.Errorf("install %s: %w", rel.TagName, err))
	}
	return e.Emit(map[string]string{"from": e.Build.Version, "to": rel.TagName, "status": "updated"}, func(w io.Writer) error {
		_, err := fmt.Fprintf(w, "updated aayu %s -> %s\n", e.Build.Version, rel.TagName)
		return err
	})
}

// packageManager reports the package manager that owns exe, if any, and the
// command that upgrades it. Self-update never replaces a managed binary.
func packageManager(exe string) (name, cmd string) {
	p := strings.ToLower(strings.ReplaceAll(exe, `\`, "/"))
	switch {
	case strings.Contains(p, "/cellar/") || strings.Contains(p, "/homebrew/") || strings.Contains(p, "/linuxbrew/"):
		return "Homebrew", "brew upgrade aayudha"
	case strings.Contains(p, "/scoop/apps/") || strings.Contains(p, "/scoop/shims/"):
		return "Scoop", "scoop update aayudha"
	case strings.Contains(p, "/winget/packages/") || strings.Contains(p, "/windowsapps/"):
		return "winget", "winget upgrade --id Rahulnakmol.AgenticAyudha"
	}
	return "", ""
}

func getJSON(ctx context.Context, c *http.Client, url string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

func download(ctx context.Context, c *http.Client, url, dst string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, io.LimitReader(resp.Body, 512<<20)); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// checkSum verifies that the archive's SHA-256 matches its line in the signed checksums file.
func checkSum(checksums, archive, name string) error {
	f, err := os.Open(checksums)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	want := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			want = fields[0]
		}
	}
	if want == "" {
		return fmt.Errorf("%s is not listed in %s", name, checksumsAsset)
	}
	a, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer func() { _ = a.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, a); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return fmt.Errorf("%s checksum mismatch: got %s, signed %s", name, got, want)
	}
	return nil
}

// replaceBinary extracts bin from the archive and atomically replaces exe with it.
func replaceBinary(archive, bin, exe string) error {
	tmp := exe + ".new"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	err = extract(archive, bin, out)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if runtime.GOOS == "windows" {
		// A running .exe cannot be overwritten but can be renamed.
		old := exe + ".old"
		_ = os.Remove(old)
		if err := os.Rename(exe, old); err != nil {
			_ = os.Remove(tmp)
			return err
		}
	}
	return os.Rename(tmp, exe)
}

func extract(archive, bin string, w io.Writer) error {
	if strings.HasSuffix(archive, ".zip") {
		zr, err := zip.OpenReader(archive)
		if err != nil {
			return err
		}
		defer func() { _ = zr.Close() }()
		for _, f := range zr.File {
			if path.Base(f.Name) == bin {
				r, err := f.Open()
				if err != nil {
					return err
				}
				defer func() { _ = r.Close() }()
				_, err = io.Copy(w, io.LimitReader(r, 512<<20))
				return err
			}
		}
		return fmt.Errorf("%s not found in archive", bin)
	}
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("%s not found in archive", bin)
		}
		if err != nil {
			return err
		}
		if h.Typeflag == tar.TypeReg && path.Base(h.Name) == bin {
			_, err = io.Copy(w, io.LimitReader(tr, 512<<20))
			return err
		}
	}
}

// toolVerifier verifies with the cosign and gh CLIs. Neither is linked into the
// binary: the sigstore libraries are not on the plan's dependency list (§2), so
// self-update shells out to them, as install.sh does (Appendix B D-1.1a).
type toolVerifier struct{}

func (toolVerifier) VerifySignature(ctx context.Context, tag, checksums, bundle string) error {
	return runTool(ctx, "cosign", "verify-blob", "--bundle", bundle,
		"--certificate-identity-regexp", releaseIdentityRE+regexp.QuoteMeta(tag)+"$",
		"--certificate-oidc-issuer", releaseOIDCIssuer, checksums)
}

func (toolVerifier) VerifyProvenance(ctx context.Context, archive string) error {
	return runTool(ctx, "gh", "attestation", "verify", archive, "--repo", releaseRepo, "--signer-workflow", releaseWorkflow)
}

func runTool(ctx context.Context, name string, args ...string) error {
	if _, err := exec.LookPath(name); err != nil {
		return Errorf(ExitExternal, "%s is required to verify releases and was not found on PATH; install it, or reinstall with install.sh", name)
	}
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// semver is a parsed semantic version (v prefix optional).
type semver struct {
	major, minor, patch int
	pre                 []string
}

func parseSemver(s string) (semver, error) {
	core := strings.TrimPrefix(s, "v")
	if i := strings.IndexByte(core, '+'); i >= 0 {
		core = core[:i]
	}
	var v semver
	if i := strings.IndexByte(core, '-'); i >= 0 {
		v.pre = strings.Split(core[i+1:], ".")
		core = core[:i]
	}
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return v, fmt.Errorf("%q is not a semantic version", s)
	}
	nums := make([]int, 3)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return v, fmt.Errorf("%q is not a semantic version", s)
		}
		nums[i] = n
	}
	v.major, v.minor, v.patch = nums[0], nums[1], nums[2]
	return v, nil
}

// compare returns -1, 0, or 1, following semver precedence rules.
func (a semver) compare(b semver) int {
	for _, d := range []int{a.major - b.major, a.minor - b.minor, a.patch - b.patch} {
		if d != 0 {
			return sign(d)
		}
	}
	switch {
	case len(a.pre) == 0 && len(b.pre) == 0:
		return 0
	case len(a.pre) == 0:
		return 1
	case len(b.pre) == 0:
		return -1
	}
	for i := 0; i < len(a.pre) && i < len(b.pre); i++ {
		x, y := a.pre[i], b.pre[i]
		xn, xerr := strconv.Atoi(x)
		yn, yerr := strconv.Atoi(y)
		switch {
		case xerr == nil && yerr == nil:
			if xn != yn {
				return sign(xn - yn)
			}
		case xerr == nil:
			return -1
		case yerr == nil:
			return 1
		default:
			if c := strings.Compare(x, y); c != 0 {
				return c
			}
		}
	}
	return sign(len(a.pre) - len(b.pre))
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	return 0
}
