// Package secrets stores and resolves named secrets (WP1.3). Config files name
// secrets; values live only in a backend: the OS keychain, AAYU_SECRET_<NAME>
// environment variables, or an age-encrypted file under the XDG data directory.
package secrets

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// ErrNotFound means no backend holds the secret.
var ErrNotFound = errors.New("secret not found")

// ErrReadOnly means the backend cannot store secrets (the env backend).
var ErrReadOnly = errors.New("backend is read-only")

// Backend is one place secrets can live.
type Backend interface {
	Name() string
	// Available returns nil when the backend can be used on this machine.
	Available() error
	Get(name string) (string, error)
	Set(name, value string) error
	Delete(name string) error
	// Names lists the secrets the backend can enumerate; the keychain cannot.
	Names() ([]string, error)
}

var namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// ValidName reports whether name is a valid secret name: lowercase letters,
// digits, '-' and '_'. Config validation uses the same rule (WP1.2).
func ValidName(name string) error {
	if !namePattern.MatchString(name) {
		return fmt.Errorf("secret name %q: want lowercase letters, digits, '-' and '_'", name)
	}
	return nil
}

// EnvVar returns the environment variable that holds a secret: AAYU_SECRET_
// followed by the name upper-cased with '-' as '_' (ai-gateway → AAYU_SECRET_AI_GATEWAY).
func EnvVar(name string) string {
	return "AAYU_SECRET_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
}

// Store resolves secrets across backends in precedence order.
type Store struct {
	backends []Backend
}

// New returns a store over backends in precedence order (keychain, env, file).
func New(backends ...Backend) *Store { return &Store{backends: backends} }

// Status is a backend's availability, for `aayu auth status` and `doctor`.
type Status struct {
	Backend   string `json:"backend"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

// Status reports every backend's availability.
func (s *Store) Status() []Status {
	out := make([]Status, len(s.backends))
	for i, b := range s.backends {
		out[i] = Status{Backend: b.Name(), Available: true}
		if err := b.Available(); err != nil {
			out[i] = Status{Backend: b.Name(), Reason: err.Error()}
		}
	}
	return out
}

// Get returns the value and the backend it came from. The first available
// backend that holds the secret wins.
func (s *Store) Get(name string) (value, backend string, err error) {
	if err := ValidName(name); err != nil {
		return "", "", err
	}
	for _, b := range s.backends {
		if b.Available() != nil {
			continue
		}
		v, err := b.Get(name)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return "", "", fmt.Errorf("%s: %w", b.Name(), err)
		}
		return v, b.Name(), nil
	}
	return "", "", fmt.Errorf("%w: %s (set it with `aayu auth set-secret %s`, or export %s)", ErrNotFound, name, name, EnvVar(name))
}

// Set stores a secret in the first available writable backend and returns its name.
func (s *Store) Set(name, value string) (string, error) {
	if err := ValidName(name); err != nil {
		return "", err
	}
	if value == "" {
		return "", errors.New("refusing to store an empty secret")
	}
	var errs []error
	for _, b := range s.backends {
		if err := b.Available(); err != nil {
			continue
		}
		err := b.Set(name, value)
		if errors.Is(err, ErrReadOnly) {
			continue
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", b.Name(), err))
			continue
		}
		return b.Name(), nil
	}
	if len(errs) == 0 {
		errs = append(errs, errors.New("no writable secrets backend is available"))
	}
	return "", errors.Join(errs...)
}

// Delete removes a secret from every writable backend that holds it and
// returns the backends it was removed from.
func (s *Store) Delete(name string) ([]string, error) {
	if err := ValidName(name); err != nil {
		return nil, err
	}
	var removed []string
	for _, b := range s.backends {
		if b.Available() != nil {
			continue
		}
		err := b.Delete(name)
		switch {
		case err == nil:
			removed = append(removed, b.Name())
		case errors.Is(err, ErrNotFound), errors.Is(err, ErrReadOnly):
		default:
			return removed, fmt.Errorf("%s: %w", b.Name(), err)
		}
	}
	return removed, nil
}

// Entry says where a secret name resolves. It never carries the value.
type Entry struct {
	Name    string `json:"name"`
	Backend string `json:"backend,omitempty"` // empty when unset
}

// List reports where each secret resolves: the names the backends can
// enumerate plus the names config references (the keychain cannot enumerate).
func (s *Store) List(referenced []string) ([]Entry, error) {
	// Names are keyed by their env variable so that "ai-gateway" (config) and
	// "ai_gateway" (enumerated from AAYU_SECRET_AI_GATEWAY) are one entry.
	names := map[string]string{}
	add := func(n string) {
		if _, ok := names[EnvVar(n)]; !ok && ValidName(n) == nil {
			names[EnvVar(n)] = n
		}
	}
	for _, n := range referenced {
		add(n)
	}
	for _, b := range s.backends {
		if b.Available() != nil {
			continue
		}
		ns, err := b.Names()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", b.Name(), err)
		}
		for _, n := range ns {
			add(n)
		}
	}
	var out []Entry
	for _, n := range names {
		_, backend, err := s.Get(n)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return nil, err
		}
		out = append(out, Entry{Name: n, Backend: backend})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// ErrWriteScoped means a write-capable key was offered to a read-only verb.
var ErrWriteScoped = errors.New("write-capable key refused for a read verb")

// CheckScope enforces [integrations.<name>] scope before any call: a verb that
// only reads refuses an integration whose key is configured as write-capable.
func CheckScope(integration, scope string, needWrite bool) error {
	switch {
	case scope != "read" && scope != "write":
		return fmt.Errorf("[integrations.%s] scope = %q: want read or write", integration, scope)
	case needWrite && scope == "read":
		return fmt.Errorf("[integrations.%s] has a read-only key; this verb writes", integration)
	case !needWrite && scope == "write":
		return fmt.Errorf("%w: [integrations.%s] scope = \"write\"; configure a read-only key for read verbs", ErrWriteScoped, integration)
	}
	return nil
}
