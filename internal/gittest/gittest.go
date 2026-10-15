// Package gittest is test support for packages whose tests run git on
// temporary repositories.
package gittest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// noMaintenance turns off git's automatic maintenance. After a commit, fetch,
// merge, or push into a local bare repository, git starts `maintenance run
// --auto` in the background. That process outlives the command, so it can
// remove files a test is copying, such as objects/maintenance.lock (#193) or
// .git/worktrees/<name>/locked during `git worktree add`, or write into a
// t.TempDir while the test cleans it up ("directory not empty").
var noMaintenance = [][2]string{{"maintenance.auto", "false"}, {"gc.auto", "0"}}

// Main runs a package's tests with git's automatic maintenance off for every
// git they start. Call it from TestMain.
//
// It sets the setting two ways because tests start git in two ways:
//   - GIT_CONFIG_* variables reach any git that inherits the test's
//     environment, even one that drops the global config with
//     GIT_CONFIG_GLOBAL.
//   - A global config file under a temporary XDG_CONFIG_HOME reaches git run
//     by ledger.Writer, which builds a fresh environment but keeps
//     XDG_CONFIG_HOME. The file first includes the git config that
//     XDG_CONFIG_HOME pointed at before, so a developer's settings there still
//     apply; ~/.gitconfig is read as before. GOENV keeps go's own settings
//     file where it was.
func Main(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	fail := func(err error) int {
		fmt.Fprintln(os.Stderr, "gittest:", err)
		return 1
	}
	dir, err := os.MkdirTemp("", "gittest-")
	if err != nil {
		return fail(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	if goCfg, err := os.UserConfigDir(); err == nil && os.Getenv("GOENV") == "" {
		_ = os.Setenv("GOENV", filepath.Join(goCfg, "go", "env"))
	}
	var cfg strings.Builder
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if home, err := os.UserHomeDir(); xdg == "" && err == nil {
		xdg = filepath.Join(home, ".config") // git's default, also on macOS and Windows
	}
	if xdg != "" { // git skips an include that does not exist
		fmt.Fprintf(&cfg, "[include]\n\tpath = \"%s\"\n", filepath.ToSlash(filepath.Join(xdg, "git", "config")))
	}
	n := 0
	if c := os.Getenv("GIT_CONFIG_COUNT"); c != "" {
		if _, err := fmt.Sscan(c, &n); err != nil {
			return fail(fmt.Errorf("GIT_CONFIG_COUNT: %w", err))
		}
	}
	for _, kv := range noMaintenance {
		section, key, _ := strings.Cut(kv[0], ".")
		fmt.Fprintf(&cfg, "[%s]\n\t%s = %s\n", section, key, kv[1])
		_ = os.Setenv(fmt.Sprintf("GIT_CONFIG_KEY_%d", n), kv[0])
		_ = os.Setenv(fmt.Sprintf("GIT_CONFIG_VALUE_%d", n), kv[1])
		n++
	}
	_ = os.Setenv("GIT_CONFIG_COUNT", fmt.Sprint(n))
	if err := os.MkdirAll(filepath.Join(dir, "git"), 0o755); err != nil {
		return fail(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "git", "config"), []byte(cfg.String()), 0o644); err != nil {
		return fail(err)
	}
	_ = os.Setenv("XDG_CONFIG_HOME", dir)
	return m.Run()
}
