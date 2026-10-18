package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/v2"
)

// Layer names, lowest precedence first.
const (
	LayerDefault = "default"
	LayerUser    = "user"
	LayerRepo    = "repo"
	LayerLocal   = "local"
	LayerEnv     = "env"
	LayerFlag    = "flag"
)

// Sources says where each layer comes from.
type Sources struct {
	UserDir string   // directory of the user config, e.g. ~/.config/aayudha; "" skips the layer
	RepoDir string   // repository root; "" skips the repo and local layers
	Environ []string // KEY=value pairs; AAYU_<SECTION>__<KEY> entries form the env layer
	Sets    []string // --set key=value flags, in order
}

// DefaultSources resolves the user config directory from the environment
// (XDG_CONFIG_HOME, then HOME or USERPROFILE) and the repo from dir.
func DefaultSources(getenv func(string) string, dir string, environ, sets []string) Sources {
	s := Sources{Environ: environ, Sets: sets, RepoDir: RepoRoot(dir)}
	switch {
	case getenv("XDG_CONFIG_HOME") != "":
		s.UserDir = filepath.Join(getenv("XDG_CONFIG_HOME"), "aayudha")
	case getenv("HOME") != "":
		s.UserDir = filepath.Join(getenv("HOME"), ".config", "aayudha")
	case getenv("USERPROFILE") != "":
		s.UserDir = filepath.Join(getenv("USERPROFILE"), ".config", "aayudha")
	}
	return s
}

// RepoRoot returns the nearest directory at or above dir that contains .git, or "".
func RepoRoot(dir string) string {
	for d := dir; d != ""; {
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return ""
}

type layer struct {
	name   string
	source string // file path, variable names, or flags
	k      *koanf.Koanf
}

// Loaded is a merged configuration with the layers it came from.
type Loaded struct {
	Config  Config
	Notices []string // e.g. files migrated in memory from an older schema
	layers  []layer
	merged  *koanf.Koanf
}

// Load reads, migrates, checks, and merges every layer. A key unknown to the
// schema is an error that names the file or variable it came from.
func Load(src Sources) (*Loaded, error) {
	l := &Loaded{merged: koanf.New(".")}
	var errs []error
	add := func(name, source string, m map[string]any) {
		if len(m) == 0 {
			return
		}
		from := Version
		if name != LayerEnv && name != LayerFlag { // only files carry a schema version
			var err error
			if from, err = migrate(m); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", source, err))
				return
			}
		}
		if from < Version {
			l.Notices = append(l.Notices, fmt.Sprintf("%s is at config schema v%d; migrated to v%d in memory (run `aayu upgrade` to rewrite it)", source, from, Version))
		}
		k := koanf.New(".")
		if err := k.Load(confmap.Provider(m, "."), nil); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", source, err))
			return
		}
		if err := decode(k, &Config{}); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", source, err))
			return
		}
		if name != LayerUser && k.Exists("fleet") {
			errs = append(errs, fmt.Errorf("%s: [fleet] is user-level only; move it to the user config", source))
			return
		}
		// Where model keys are sent is the person's choice, never the
		// checkout's: a PR's .aayudha/config.toml (or a committed
		// config.local.toml) could otherwise point the gateway at its own
		// server and name any local secret as the Bearer key.
		if (name == LayerRepo || name == LayerLocal) && (k.Exists("models.gateway.base_url") || k.Exists("models.gateway.secret")) {
			errs = append(errs, fmt.Errorf("%s: [models.gateway] base_url and secret are read only from the user config, AAYU_MODELS__GATEWAY__* variables, or --set, never from the checkout", source))
			return
		}
		// The Cursor plan is the person's subscription, and "legacy" picks
		// models a new plan charges at list price, so no checkout sets it.
		if (name == LayerRepo || name == LayerLocal) && k.Exists("hosts.cursor.plan") {
			errs = append(errs, fmt.Errorf("%s: [hosts.cursor] plan is read only from the user config, AAYU_HOSTS__CURSOR__PLAN, or --set, never from the checkout", source))
			return
		}
		// Likewise the ui-review lane's Copilot route: it spends the person's
		// Copilot credits and sends their captures to Copilot, so only the
		// person picks its model or turns it off.
		if (name == LayerRepo || name == LayerLocal) && k.Exists("models.ui-review.copilot") {
			errs = append(errs, fmt.Errorf("%s: [models.ui-review.copilot] (the Copilot route for UI review) is read only from the user config or --set, never from the checkout", source))
			return
		}
		// Like [autonomy], the solo window is the protected repository
		// config's alone (Story #432): no person's or agent's own layer sets it.
		if name != LayerDefault && name != LayerRepo && k.Exists("decisions.solo_provisional_after_hours") {
			errs = append(errs, fmt.Errorf("%s: [decisions] solo_provisional_after_hours is read only from .aayudha/config.toml, a protected path", source))
			return
		}
		// Personal data from a host's PostHog or Sentry MCP is opt-in for the
		// repository (Decision #449), so only its protected config turns it on.
		if name != LayerDefault && name != LayerRepo {
			for _, p := range []string{"posthog", "sentry"} {
				if k.Exists("integrations." + p + ".allow_pii") {
					errs = append(errs, fmt.Errorf("%s: [integrations.%s] allow_pii is read only from .aayudha/config.toml, a protected path", source, p))
					return
				}
			}
		}
		l.layers = append(l.layers, layer{name, source, k})
		if err := l.merged.Merge(k); err != nil {
			errs = append(errs, err)
		}
	}

	m, err := toml.Parser().Unmarshal(DefaultsTOML) // the file CONSTRAINTS.md is generated from (WP0.5)
	if err != nil {
		return nil, fmt.Errorf("embedded defaults.toml: %w", err)
	}
	m["version"] = int64(Version)
	add(LayerDefault, "defaults.toml (embedded)", m)

	if src.UserDir != "" {
		for _, f := range readDir(src.UserDir, "config", &errs) {
			add(LayerUser, f.path, f.m)
		}
	}
	if src.RepoDir != "" {
		dir := filepath.Join(src.RepoDir, ".aayudha")
		for _, f := range readDir(dir, "config", &errs) {
			add(LayerRepo, f.path, f.m)
		}
		for _, f := range readDir(dir, "config.local", &errs) {
			add(LayerLocal, f.path, f.m)
		}
	}
	if m, names := envMap(src.Environ); len(m) > 0 {
		add(LayerEnv, strings.Join(names, ", "), m)
	}
	if len(src.Sets) > 0 {
		m, err := setMap(src.Sets)
		if err != nil {
			errs = append(errs, err)
		} else {
			add(LayerFlag, "--set "+strings.Join(src.Sets, " --set "), m)
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	if err := decode(l.merged, &l.Config); err != nil {
		return nil, err
	}
	if err := l.Config.Validate(); err != nil {
		return nil, err
	}
	return l, nil
}

func decode(k *koanf.Koanf, out *Config) error {
	return k.UnmarshalWithConf("", out, koanf.UnmarshalConf{
		Tag: "koanf",
		DecoderConfig: &mapstructure.DecoderConfig{
			Result:           out,
			TagName:          "koanf",
			ErrorUnused:      true,
			WeaklyTypedInput: true, // env and --set values arrive as strings
			DecodeHook:       mapstructure.StringToSliceHookFunc(","),
		},
	})
}

type file struct {
	path string
	m    map[string]any
}

// readDir returns base.json then base.toml from dir, so TOML wins when both exist.
func readDir(dir, base string, errs *[]error) []file {
	var out []file
	for _, ext := range []string{".json", ".toml"} {
		path := filepath.Join(dir, base+ext)
		b, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			*errs = append(*errs, err)
			continue
		}
		var m map[string]any
		if ext == ".json" {
			m, err = json.Parser().Unmarshal(b)
		} else {
			m, err = toml.Parser().Unmarshal(b)
		}
		if err != nil {
			*errs = append(*errs, fmt.Errorf("%s: %w", path, err))
			continue
		}
		if m == nil {
			m = map[string]any{}
		}
		out = append(out, file{path, m})
	}
	return out
}

// envMap turns AAYU_<SECTION>__<KEY>=value variables into a nested map.
// Double underscores separate levels; other AAYU_* variables (AAYU_AGENT,
// AAYU_LIVE, AAYU_SECRET_*, AAYU_GITHUB_TOKEN) are not configuration.
func envMap(environ []string) (map[string]any, []string) {
	flat := map[string]any{}
	var names []string
	for _, kv := range environ {
		name, value, ok := strings.Cut(kv, "=")
		if !ok || !strings.HasPrefix(name, "AAYU_") || !strings.Contains(name, "__") || strings.HasPrefix(name, "AAYU_SECRET_") {
			continue
		}
		key := strings.ToLower(strings.ReplaceAll(strings.TrimPrefix(name, "AAYU_"), "__", "."))
		flat[key] = value
		names = append(names, name)
	}
	sort.Strings(names)
	return unflatten(flat), names
}

func setMap(sets []string) (map[string]any, error) {
	flat := map[string]any{}
	for _, s := range sets {
		key, value, ok := strings.Cut(s, "=")
		if !ok || key == "" {
			return nil, fmt.Errorf("--set %q: want key=value, e.g. budget.per_day_usd=20", s)
		}
		flat[key] = value
	}
	return unflatten(flat), nil
}

func unflatten(flat map[string]any) map[string]any {
	k := koanf.New(".")
	_ = k.Load(confmap.Provider(flat, "."), nil)
	return k.Raw()
}

// Keys returns every leaf key of the merged configuration, sorted.
func (l *Loaded) Keys() []string {
	keys := l.merged.Keys()
	slices.Sort(keys)
	return keys
}

// Get returns the merged value at key.
func (l *Loaded) Get(key string) any { return l.merged.Get(key) }

// Raw returns the merged configuration as nested maps.
func (l *Loaded) Raw() map[string]any { return l.merged.Raw() }

// Origin returns the layer and source (file, variable, or flag) that set key.
func (l *Loaded) Origin(key string) (name, source string) {
	for i := len(l.layers) - 1; i >= 0; i-- {
		if l.layers[i].k.Exists(key) {
			return l.layers[i].name, l.layers[i].source
		}
	}
	return "", ""
}
