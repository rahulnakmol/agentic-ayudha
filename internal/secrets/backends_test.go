package secrets

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// sessionDaemons counts the session dbus-daemons running now: what
// dbus-launch forks and leaves behind.
func sessionDaemons(t *testing.T) int {
	t.Helper()
	n := 0
	cmdlines, _ := filepath.Glob("/proc/[0-9]*/cmdline")
	for _, f := range cmdlines {
		b, err := os.ReadFile(f)
		if err == nil && strings.Contains(string(b), "dbus-daemon") && strings.Contains(string(b), "--session") {
			n++
		}
	}
	return n
}

// Without a D-Bus session bus, the keychain is unavailable, and asking it
// starts no dbus-daemon that outlives aayu (go-keyring's library would run
// dbus-launch, which forks one per probe).
func TestKeychainStartsNoSessionBus(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the session-bus check is for Linux")
	}
	// A bus socket under XDG_RUNTIME_DIR is not one godbus finds.
	xdg := t.TempDir()
	if err := os.WriteFile(filepath.Join(xdg, "bus"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_RUNTIME_DIR", xdg)
	old := runUserDir
	runUserDir = t.TempDir()
	t.Cleanup(func() { runUserDir = old })
	before := sessionDaemons(t)
	for _, addr := range []string{"", "autolaunch:"} {
		t.Setenv("DBUS_SESSION_BUS_ADDRESS", addr)
		k := Keychain{}
		if err := k.Available(); err == nil || !strings.Contains(err.Error(), "no D-Bus session bus") {
			t.Errorf("%q: Available: %v", addr, err)
		}
		if _, err := k.Get("x"); err == nil {
			t.Errorf("%q: Get found a keychain without a session bus", addr)
		}
		if err := k.Set("x", "y"); err == nil {
			t.Errorf("%q: Set wrote a keychain without a session bus", addr)
		}
	}
	if after := sessionDaemons(t); after > before {
		t.Errorf("the keychain started %d session dbus-daemon(s)", after-before)
	}
}
