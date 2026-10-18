package secrets

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"filippo.io/age"
	"github.com/zalando/go-keyring"
)

// keyringService is the service name aayudha uses in the OS keychain.
const keyringService = "aayudha"

// Keychain is the OS keychain backend (macOS Keychain, Windows Credential
// Manager, Secret Service over D-Bus on Linux).
type Keychain struct{}

func (Keychain) Name() string { return "keychain" }

// errNoSessionBus is the Secret Service's answer before it is asked: there is
// no D-Bus session bus to ask it on.
var errNoSessionBus = errors.New("OS keychain unavailable: no D-Bus session bus (DBUS_SESSION_BUS_ADDRESS is unset or autolaunch:, and /run/user/<uid> has no bus or dbus-session)")

// runUserDir is where godbus looks for a running session bus when
// DBUS_SESSION_BUS_ADDRESS is unset (conn_other.go, getRuntimeDirectory):
// /run/user/<uid> from user.Current, never XDG_RUNTIME_DIR. When the lookup
// fails, godbus skips this discovery and runs dbus-launch.
var runUserDir = func() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	return "/run/user/" + u.Uid, nil
}

// sessionBus reports whether the Secret Service can be reached without
// starting anything. go-keyring's D-Bus library otherwise runs dbus-launch,
// which forks a session dbus-daemon that outlives aayu: one per probe on a
// headless machine. It mirrors the lookup godbus v5 runs before it falls back
// to dbus-launch: an address it would dial, which an empty or autolaunch: one
// is not. macOS and Windows use no D-Bus.
func sessionBus() error {
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		return nil
	}
	dialable := func(a string) bool { return a != "" && !strings.HasPrefix(a, "autolaunch:") }
	if dialable(os.Getenv("DBUS_SESSION_BUS_ADDRESS")) {
		return nil
	}
	dir, err := runUserDir()
	if err != nil {
		return errNoSessionBus
	}
	// godbus's discovery, in its order: the bus socket, then a file naming the address.
	if _, err := os.Stat(filepath.Join(dir, "bus")); err == nil {
		return nil
	}
	if b, err := os.ReadFile(filepath.Join(dir, "dbus-session")); err == nil {
		if a, ok := strings.CutPrefix(string(b), "DBUS_SESSION_BUS_ADDRESS="); ok && dialable(strings.TrimRight(a, "\n\r")) {
			return nil
		}
	}
	return errNoSessionBus
}

// Available probes the keychain; headless Linux without a Secret Service fails here.
func (Keychain) Available() error {
	if err := sessionBus(); err != nil {
		return err
	}
	_, err := keyring.Get(keyringService, "aayudha-probe")
	if err == nil || errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return fmt.Errorf("OS keychain unavailable: %w", err)
}

func (Keychain) Get(name string) (string, error) {
	if err := sessionBus(); err != nil {
		return "", err
	}
	v, err := keyring.Get(keyringService, name)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	}
	return v, err
}

func (Keychain) Set(name, value string) error {
	if err := sessionBus(); err != nil {
		return err
	}
	return keyring.Set(keyringService, name, value)
}

func (Keychain) Delete(name string) error {
	if err := sessionBus(); err != nil {
		return err
	}
	err := keyring.Delete(keyringService, name)
	if errors.Is(err, keyring.ErrNotFound) {
		return ErrNotFound
	}
	return err
}

func (Keychain) Names() ([]string, error) { return nil, nil }

// Env is the read-only AAYU_SECRET_<NAME> environment backend, for CI and
// headless machines.
type Env struct {
	Getenv  func(string) string
	Environ []string
}

func (Env) Name() string     { return "env" }
func (Env) Available() error { return nil }

func (e Env) Get(name string) (string, error) {
	if v := e.Getenv(EnvVar(name)); v != "" {
		return v, nil
	}
	return "", ErrNotFound
}

func (Env) Set(string, string) error { return ErrReadOnly }
func (Env) Delete(string) error      { return ErrReadOnly }

func (e Env) Names() ([]string, error) {
	var out []string
	for _, kv := range e.Environ {
		k, v, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(k, "AAYU_SECRET_") && v != "" {
			out = append(out, strings.ToLower(strings.TrimPrefix(k, "AAYU_SECRET_")))
		}
	}
	return out, nil
}

// File is the age-encrypted file backend: secrets live as one encrypted JSON
// object in Path, encrypted to the X25519 identity in IdentityPath. Both files
// are created 0600 under the XDG directories.
type File struct {
	Path         string // e.g. $XDG_DATA_HOME/aayudha/secrets.age
	IdentityPath string // e.g. $XDG_CONFIG_HOME/aayudha/age-identity.txt
}

func (File) Name() string { return "file" }

func (f File) Available() error {
	if f.Path == "" || f.IdentityPath == "" {
		return errors.New("no XDG data or config directory")
	}
	return nil
}

func (f File) identity(create bool) (*age.X25519Identity, error) {
	b, err := os.ReadFile(f.IdentityPath)
	if errors.Is(err, os.ErrNotExist) && create {
		id, err := age.GenerateX25519Identity()
		if err != nil {
			return nil, err
		}
		if err := writePrivate(f.IdentityPath, []byte(id.String()+"\n")); err != nil {
			return nil, err
		}
		return id, nil
	}
	if err != nil {
		return nil, err
	}
	return age.ParseX25519Identity(strings.TrimSpace(string(b)))
}

func (f File) load() (map[string]string, error) {
	b, err := os.ReadFile(f.Path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	id, err := f.identity(false)
	if err != nil {
		return nil, fmt.Errorf("age identity %s: %w", f.IdentityPath, err)
	}
	r, err := age.Decrypt(bytes.NewReader(b), id)
	if err != nil {
		return nil, fmt.Errorf("decrypt %s: %w", f.Path, err)
	}
	plain, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	m := map[string]string{}
	return m, json.Unmarshal(plain, &m)
}

func (f File) save(m map[string]string) error {
	id, err := f.identity(true)
	if err != nil {
		return err
	}
	plain, err := json.Marshal(m)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, id.Recipient())
	if err != nil {
		return err
	}
	if _, err := w.Write(plain); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return writePrivate(f.Path, buf.Bytes())
}

func (f File) Get(name string) (string, error) {
	m, err := f.load()
	if err != nil {
		return "", err
	}
	if v, ok := m[name]; ok {
		return v, nil
	}
	return "", ErrNotFound
}

func (f File) Set(name, value string) error {
	m, err := f.load()
	if err != nil {
		return err
	}
	m[name] = value
	return f.save(m)
}

func (f File) Delete(name string) error {
	m, err := f.load()
	if err != nil {
		return err
	}
	if _, ok := m[name]; !ok {
		return ErrNotFound
	}
	delete(m, name)
	return f.save(m)
}

func (f File) Names() ([]string, error) {
	if _, err := os.Stat(f.Path); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	m, err := f.load()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(m))
	for n := range m {
		out = append(out, n)
	}
	sort.Strings(out)
	return out, nil
}

// writePrivate atomically writes data with mode 0600 (directories 0700).
func writePrivate(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// Dirs returns the XDG config and data directories for aayudha from the environment.
func Dirs(getenv func(string) string) (configDir, dataDir string) {
	home := getenv("HOME")
	if home == "" {
		home = getenv("USERPROFILE")
	}
	configDir, dataDir = getenv("XDG_CONFIG_HOME"), getenv("XDG_DATA_HOME")
	if configDir == "" && home != "" {
		configDir = filepath.Join(home, ".config")
	}
	if dataDir == "" && home != "" {
		dataDir = filepath.Join(home, ".local", "share")
	}
	if configDir != "" {
		configDir = filepath.Join(configDir, "aayudha")
	}
	if dataDir != "" {
		dataDir = filepath.Join(dataDir, "aayudha")
	}
	return configDir, dataDir
}

// Default returns the standard store: keychain, then env, then the age file.
func Default(getenv func(string) string, environ []string) *Store {
	cfg, data := Dirs(getenv)
	f := File{}
	if cfg != "" && data != "" {
		f = File{Path: filepath.Join(data, "secrets.age"), IdentityPath: filepath.Join(cfg, "age-identity.txt")}
	}
	return New(Keychain{}, Env{Getenv: getenv, Environ: environ}, f)
}
