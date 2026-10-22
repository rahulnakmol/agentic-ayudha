package agentenv

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Fetch reads a URL (tests inject a fake).
type Fetch func(ctx context.Context, url string) ([]byte, error)

// HTTPFetch is Fetch over an HTTP client.
func HTTPFetch(c *http.Client) Fetch {
	if c == nil {
		c = http.DefaultClient
	}
	return func(ctx context.Context, u string) ([]byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		resp, err := c.Do(req)
		if err != nil {
			return nil, err
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("GET %s: %s", u, resp.Status)
		}
		return io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	}
}

// Resolve pins every tool the spec installs to an exact version, with the
// SHA-256 of each download for amd64 and arm64, from the vendors' own release
// indexes. It is the only step that uses the network; generation from a
// resolved spec is offline and deterministic. A tool already pinned with its
// checksums is left alone.
func Resolve(ctx context.Context, s *Spec, get Fetch) error {
	needsCorepack := false
	for i := range s.Tools {
		t := &s.Tools[i]
		var err error
		switch t.Name {
		case "go":
			err = resolveGo(ctx, t, get)
		case "node":
			err = resolveNode(ctx, t, get)
		case "uv":
			err = resolveUV(ctx, t, get)
		case "rust":
			err = resolveRustup(ctx, t, get)
		case "aayu":
			err = resolveAayu(ctx, t, get)
		case "pnpm", "yarn":
			needsCorepack = true
		}
		if err != nil {
			return fmt.Errorf("resolve %s: %w", t.Name, err)
		}
	}
	if needsCorepack && s.Tool("corepack") == nil {
		b, err := get(ctx, "https://registry.npmjs.org/corepack/latest")
		if err != nil {
			return fmt.Errorf("resolve corepack: %w", err)
		}
		var v struct{ Version string }
		if err := json.Unmarshal(b, &v); err != nil || v.Version == "" {
			return fmt.Errorf("resolve corepack: no version in the npm registry's answer")
		}
		s.Tools = append(s.Tools, Tool{Name: "corepack", Want: ">=" + v.Version, Version: v.Version, Via: "npm",
			Evidence: "pnpm and yarn come through corepack; a current corepack has the registry's current signing keys"})
	}
	return nil
}

// aayuReleases is where aayu's signed release assets live (install.sh).
const aayuReleases = "https://github.com/rahulnakmol/agentic-ayudha/releases/download/v"

// resolveAayu pins the release's linux archives to their lines in its
// checksums.txt, the file install.sh checks the archive against.
func resolveAayu(ctx context.Context, t *Tool, get Fetch) error {
	if t.Via != "release" || len(t.SHA256) == 2 {
		return nil
	}
	b, err := get(ctx, aayuReleases+t.Version+"/checksums.txt")
	if err != nil {
		return err
	}
	t.SHA256 = map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		for _, a := range []string{"amd64", "arm64"} {
			if len(f) == 2 && f[1] == "aayudha_"+t.Version+"_linux_"+a+".tar.gz" {
				t.SHA256[a] = f[0]
			}
		}
	}
	if len(t.SHA256) != 2 {
		return fmt.Errorf("checksums.txt of v%s lists no linux amd64 and arm64 archives", t.Version)
	}
	return nil
}

var goArch = map[string]string{"amd64": "amd64", "arm64": "arm64"}

func resolveGo(ctx context.Context, t *Tool, get Fetch) error {
	if t.Version != "" && len(t.SHA256) == 2 {
		return nil
	}
	b, err := get(ctx, "https://go.dev/dl/?mode=json&include=all")
	if err != nil {
		return err
	}
	var rels []struct {
		Version string `json:"version"`
		Stable  bool   `json:"stable"`
		Files   []struct {
			Filename, OS, Arch, Kind, SHA256 string
		} `json:"files"`
	}
	if err := json.Unmarshal(b, &rels); err != nil {
		return err
	}
	want := t.Version
	if want == "" { // the newest stable patch of go.mod's minor
		floor := strings.TrimPrefix(t.Want, ">=")
		minor := strings.Join(strings.SplitN(floor, ".", 3)[:min(2, len(strings.Split(floor, ".")))], ".")
		for _, r := range rels {
			v := strings.TrimPrefix(r.Version, "go")
			if r.Stable && (v == minor || strings.HasPrefix(v, minor+".")) && !olderVersion(v, floor) && (want == "" || olderVersion(want, v)) {
				want = v
			}
		}
		if want == "" {
			return fmt.Errorf("no stable release of Go %s on go.dev", minor)
		}
	}
	t.Version, t.SHA256, t.Via = want, map[string]string{}, "tarball"
	for _, r := range rels {
		if r.Version != "go"+want {
			continue
		}
		for _, f := range r.Files {
			if f.OS == "linux" && f.Kind == "archive" && goArch[f.Arch] != "" {
				t.SHA256[f.Arch] = f.SHA256
			}
		}
	}
	if len(t.SHA256) != 2 {
		return fmt.Errorf("go.dev lists no linux amd64 and arm64 archives for go%s", want)
	}
	return nil
}

// version is a dotted version; missing parts are zero.
type version [3]int

func parseVersion(s string) (version, bool) {
	var v version
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	parts := strings.Split(s, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return v, false
	}
	for i, p := range parts {
		if p == "x" || p == "*" {
			break
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

func (v version) String() string { return fmt.Sprintf("%d.%d.%d", v[0], v[1], v[2]) }

func (v version) less(w version) bool {
	for i := range v {
		if v[i] != w[i] {
			return v[i] < w[i]
		}
	}
	return false
}

// Ranges parses an npm-style engines range (`^22.12.0 || >=26`) into "lo hi"
// half-open ranges (hi empty: no upper bound).
func Ranges(want string) ([]string, error) {
	var out []string
	for _, clause := range strings.Split(want, "||") {
		f := strings.Fields(clause)
		if len(f) == 0 {
			continue
		}
		lo, hi := "", ""
		for _, tok := range f {
			op := strings.TrimRight(tok, "0123456789.xX*")
			v, ok := parseVersion(tok[len(op):])
			if !ok {
				return nil, fmt.Errorf("cannot read %q in %q", tok, want)
			}
			parts := len(strings.Split(strings.TrimRight(tok[len(op):], ".xX*"), "."))
			switch op {
			case "^":
				lo, hi = v.String(), version{v[0] + 1}.String()
				if v[0] == 0 {
					hi = version{0, v[1] + 1}.String()
				}
			case "~":
				lo, hi = v.String(), version{v[0], v[1] + 1}.String()
			case ">=":
				lo = v.String()
			case ">":
				lo = version{v[0], v[1], v[2] + 1}.String()
			case "<":
				hi = v.String()
			case "<=":
				hi = version{v[0], v[1], v[2] + 1}.String()
			case "", "=":
				lo = v.String()
				switch parts {
				case 1:
					hi = version{v[0] + 1}.String()
				case 2:
					hi = version{v[0], v[1] + 1}.String()
				default:
					hi = version{v[0], v[1], v[2] + 1}.String()
				}
			default:
				return nil, fmt.Errorf("cannot read %q in %q", tok, want)
			}
		}
		if lo == "" {
			lo = "0.0.0"
		}
		out = append(out, strings.TrimSpace(lo+" "+hi))
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty range %q", want)
	}
	return out, nil
}

func inRanges(v version, rs []string) bool {
	for _, r := range rs {
		f := strings.Fields(r)
		lo, _ := parseVersion(f[0])
		if v.less(lo) {
			continue
		}
		if len(f) > 1 {
			if hi, _ := parseVersion(f[1]); !v.less(hi) {
				continue
			}
		}
		return true
	}
	return false
}

func resolveNode(ctx context.Context, t *Tool, get Fetch) error {
	if t.Version != "" && len(t.SHA256) == 2 {
		return nil
	}
	b, err := get(ctx, "https://nodejs.org/dist/index.json")
	if err != nil {
		return err
	}
	var rels []struct {
		Version string `json:"version"`
		LTS     any    `json:"lts"`
	}
	if err := json.Unmarshal(b, &rels); err != nil {
		return err
	}
	var vs []version
	for _, r := range rels {
		if lts, ok := r.LTS.(string); !ok || lts == "" {
			continue
		}
		if v, ok := parseVersion(r.Version); ok {
			vs = append(vs, v)
		}
	}
	sort.Slice(vs, func(i, j int) bool { return vs[j].less(vs[i]) }) // newest first
	if t.Want == "lts" {
		if len(vs) == 0 {
			return fmt.Errorf("nodejs.org lists no LTS release")
		}
		t.Accept = []string{version{vs[0][0]}.String() + " " + version{vs[0][0] + 1}.String()}
	} else if t.Accept, err = Ranges(t.Want); err != nil {
		return err
	}
	// The newest LTS release of the lowest major the range accepts: the
	// release line the project supports longest.
	var pick version
	found := false
	for i := len(vs) - 1; i >= 0 && !found; i-- {
		if inRanges(vs[i], t.Accept) {
			major := vs[i][0]
			for _, v := range vs {
				if v[0] == major && inRanges(v, t.Accept) {
					pick, found = v, true
					break
				}
			}
		}
	}
	if !found {
		return fmt.Errorf("no LTS release of Node.js satisfies %q", t.Want)
	}
	t.Version, t.Via = pick.String(), "tarball"
	sums, err := get(ctx, "https://nodejs.org/dist/v"+t.Version+"/SHASUMS256.txt")
	if err != nil {
		return err
	}
	t.SHA256 = map[string]string{}
	sc := bufio.NewScanner(strings.NewReader(string(sums)))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) != 2 {
			continue
		}
		switch f[1] {
		case "node-v" + t.Version + "-linux-x64.tar.xz":
			t.SHA256["amd64"] = f[0]
		case "node-v" + t.Version + "-linux-arm64.tar.xz":
			t.SHA256["arm64"] = f[0]
		}
	}
	if len(t.SHA256) != 2 {
		return fmt.Errorf("SHASUMS256.txt for v%s lists no linux x64 and arm64 tarballs", t.Version)
	}
	return nil
}

var uvTriple = map[string]string{"amd64": "x86_64-unknown-linux-gnu", "arm64": "aarch64-unknown-linux-gnu"}

func resolveUV(ctx context.Context, t *Tool, get Fetch) error {
	if t.Version != "" && len(t.SHA256) == 2 {
		return nil
	}
	if t.Version == "" {
		b, err := get(ctx, "https://api.github.com/repos/astral-sh/uv/releases/latest")
		if err != nil {
			return err
		}
		var rel struct {
			TagName string `json:"tag_name"`
		}
		if err := json.Unmarshal(b, &rel); err != nil || rel.TagName == "" {
			return fmt.Errorf("no tag in the latest uv release")
		}
		t.Version = rel.TagName
	}
	t.SHA256 = map[string]string{}
	for arch, triple := range uvTriple {
		b, err := get(ctx, "https://github.com/astral-sh/uv/releases/download/"+t.Version+"/uv-"+triple+".tar.gz.sha256")
		if err != nil {
			return err
		}
		f := strings.Fields(string(b))
		if len(f) == 0 || len(f[0]) != 64 {
			return fmt.Errorf("uv %s: no SHA-256 for %s", t.Version, triple)
		}
		t.SHA256[arch] = f[0]
	}
	return nil
}

var rustupVersion = regexp.MustCompile(`(?m)^version\s*=\s*"([0-9.]+)"`)

func resolveRustup(ctx context.Context, t *Tool, get Fetch) error {
	if t.Version != "" && len(t.SHA256) == 2 {
		return nil
	}
	b, err := get(ctx, "https://static.rust-lang.org/rustup/release-stable.toml")
	if err != nil {
		return err
	}
	m := rustupVersion.FindSubmatch(b)
	if m == nil {
		return fmt.Errorf("no version in rustup's release-stable.toml")
	}
	t.Version, t.SHA256 = string(m[1]), map[string]string{}
	for arch, triple := range uvTriple {
		b, err := get(ctx, "https://static.rust-lang.org/rustup/archive/"+t.Version+"/"+triple+"/rustup-init.sha256")
		if err != nil {
			return err
		}
		f := strings.Fields(string(b))
		if len(f) == 0 || len(f[0]) != 64 {
			return fmt.Errorf("rustup %s: no SHA-256 for %s", t.Version, triple)
		}
		t.SHA256[arch] = f[0]
	}
	return nil
}
