// Package ops is the ops action catalog `respond` executes from (WP5.6):
// ops/actions.toml, read only from the protected default branch at a pinned
// commit, never from a checkout or a pull request. Each action is an argv
// template with typed parameters (no shell), an approval mode, the GitHub
// Environments it may run in, an hourly cap, a rollback, and a blast radius.
// An action can be `auto` only once its definition has merged before as
// `human`: a definition whose first appearance on the default branch says
// `auto` is treated as `human`.
package ops

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Path is the catalog's path in the repository; a protected path (CODEOWNERS).
const Path = "ops/actions.toml"

// Version is the catalog schema version.
const Version = 1

// Approval modes.
const (
	Auto  = "auto"
	Human = "human"
)

// Blast radii, smallest first. A global action is never auto.
var BlastRadii = []string{"single", "service", "global"}

// EnvironmentParam is the reserved parameter that picks one of an action's
// environments when it lists more than one.
const EnvironmentParam = "environment"

// Errors.
var (
	ErrNotPinned = errors.New("the catalog commit is not on the protected default branch")
	ErrNoCatalog = errors.New("no ops catalog (" + Path + ") on the default branch")
	ErrUnknown   = errors.New("not a catalog action")
	ErrParams    = errors.New("invalid action parameters")
)

// Param is one typed parameter.
type Param struct {
	Type     string   `toml:"type" json:"type"`                           // string, int, enum, bool
	Pattern  string   `toml:"pattern,omitempty" json:"pattern,omitempty"` // string: anchored regular expression the whole value must match
	Values   []string `toml:"values,omitempty" json:"values,omitempty"`   // enum
	Min      *int     `toml:"min,omitempty" json:"min,omitempty"`         // int
	Max      *int     `toml:"max,omitempty" json:"max,omitempty"`         // int
	Required bool     `toml:"required,omitempty" json:"required,omitempty"`
	Summary  string   `toml:"summary,omitempty" json:"summary,omitempty"`
}

// Action is one catalog entry.
type Action struct {
	Name         string           `toml:"name" json:"name"`
	Summary      string           `toml:"summary,omitempty" json:"summary,omitempty"`
	Argv         []string         `toml:"argv_template" json:"argv_template"`
	Params       map[string]Param `toml:"params,omitempty" json:"params,omitempty"`
	Approval     string           `toml:"approval" json:"approval"`
	Environments []string         `toml:"environments" json:"environments"`
	MaxPerHour   int              `toml:"max_per_hour" json:"max_per_hour"`
	Rollback     string           `toml:"rollback" json:"rollback"`
	BlastRadius  string           `toml:"blast_radius" json:"blast_radius"`

	// Effective is the approval respond enforces: Approval, except that an
	// auto action whose definition first merged as auto is human.
	Effective string `toml:"-" json:"effective_approval"`
	Note      string `toml:"-" json:"note,omitempty"`
}

// Catalog is ops/actions.toml at one commit.
type Catalog struct {
	Version int      `toml:"version" json:"version"`
	Actions []Action `toml:"action" json:"actions"`
	SHA     string   `toml:"-" json:"sha"` // the pinned commit it was read at
}

var (
	nameRE  = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)
	envRE   = regexp.MustCompile(`^[A-Za-z0-9._-]{1,255}$`)
	holeRE  = regexp.MustCompile(`\{([^{}]*)\}`)
	shells  = []string{"sh", "bash", "zsh", "dash", "ksh", "fish", "cmd", "cmd.exe", "powershell", "powershell.exe", "pwsh", "pwsh.exe"}
	inlines = []string{"-c", "/c", "/C", "-Command", "-command", "-EncodedCommand", "-e"}
)

// Parse reads and validates a catalog. Unknown keys are errors.
func Parse(data []byte) (*Catalog, error) {
	var c Catalog
	dec := toml.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		var sm *toml.StrictMissingError
		if errors.As(err, &sm) {
			return nil, fmt.Errorf("%s: unknown keys:\n%s", Path, sm.String())
		}
		return nil, fmt.Errorf("%s: %w", Path, err)
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	for i := range c.Actions {
		c.Actions[i].Effective = c.Actions[i].Approval
	}
	return &c, nil
}

// Validate checks the catalog's rules.
func (c *Catalog) Validate() error {
	var errs []error
	bad := func(format string, args ...any) { errs = append(errs, fmt.Errorf(Path+": "+format, args...)) }
	if c.Version != Version {
		bad("version = %d: want %d", c.Version, Version)
	}
	names := map[string]bool{}
	for _, a := range c.Actions {
		if !nameRE.MatchString(a.Name) {
			bad("action %q: name must match %s", a.Name, nameRE)
		}
		if names[a.Name] {
			bad("action %q: defined twice", a.Name)
		}
		names[a.Name] = true
	}
	for _, a := range c.Actions {
		at := func(format string, args ...any) { bad("action %q: "+format, append([]any{a.Name}, args...)...) }
		if len(a.Argv) == 0 || strings.TrimSpace(a.Argv[0]) == "" {
			at("argv_template needs a program")
		} else {
			if strings.ContainsAny(a.Argv[0], "{}") {
				at("the program (argv_template[0]) cannot be a parameter")
			}
			if slices.Contains(shells, path.Base(strings.ReplaceAll(a.Argv[0], `\`, "/"))) && slices.ContainsFunc(a.Argv[1:], func(s string) bool { return slices.Contains(inlines, s) }) {
				at("an inline shell command (%s -c …) is not an argv template; put the script in the repository and run it", a.Argv[0])
			}
		}
		for _, arg := range a.Argv {
			for _, m := range holeRE.FindAllStringSubmatch(arg, -1) {
				if _, ok := a.Params[m[1]]; !ok {
					at("argv_template uses {%s}, which is not a declared parameter", m[1])
				}
			}
		}
		for name, p := range a.Params {
			switch {
			case name == EnvironmentParam:
				at("parameter %q is reserved: it picks one of environments", name)
			case !nameRE.MatchString(strings.ReplaceAll(name, "_", "-")):
				at("parameter %q: want lowercase letters, digits, - and _", name)
			}
			switch p.Type {
			case "string":
				if !strings.HasPrefix(p.Pattern, "^") || !strings.HasSuffix(p.Pattern, "$") {
					at("string parameter %q needs an anchored pattern (^…$)", name)
				} else if _, err := regexp.Compile(p.Pattern); err != nil {
					at("parameter %q: pattern: %v", name, err)
				}
			case "enum":
				if len(p.Values) == 0 {
					at("enum parameter %q needs values", name)
				}
			case "int":
				if p.Min != nil && p.Max != nil && *p.Min > *p.Max {
					at("int parameter %q: min > max", name)
				}
			case "bool":
			default:
				at("parameter %q: type %q: want string, int, enum, or bool", name, p.Type)
			}
		}
		if a.Approval != Auto && a.Approval != Human {
			at("approval = %q: want auto or human", a.Approval)
		}
		if len(a.Environments) == 0 {
			at("environments: name at least one GitHub Environment")
		}
		for _, e := range a.Environments {
			if !envRE.MatchString(e) {
				at("environment %q is not a GitHub Environment name", e)
			}
		}
		if a.MaxPerHour < 1 {
			at("max_per_hour must be at least 1")
		}
		switch {
		case a.Rollback == "none":
		case a.Rollback == a.Name:
			at("rollback cannot be the action itself")
		case !names[a.Rollback]:
			at("rollback = %q: want \"none\" or another action's name", a.Rollback)
		}
		if !slices.Contains(BlastRadii, a.BlastRadius) {
			at("blast_radius = %q: want one of %s", a.BlastRadius, strings.Join(BlastRadii, ", "))
		}
		if a.BlastRadius == "global" && a.Approval == Auto {
			at("a global blast radius is never auto")
		}
	}
	return errors.Join(errs...)
}

// Find returns the action named name.
func (c *Catalog) Find(name string) (Action, bool) {
	for _, a := range c.Actions {
		if a.Name == name {
			return a, true
		}
	}
	return Action{}, false
}

// Source reads the repository through the GitHub API.
type Source interface {
	// File returns path at ref; ok is false when it does not exist there.
	File(ctx context.Context, ref, path string) (data []byte, ok bool, err error)
	// OnDefault reports whether sha is reachable from the default branch.
	OnDefault(ctx context.Context, sha string) (bool, error)
	// History lists up to max commits reachable from sha that changed path, newest first.
	History(ctx context.Context, sha, path string, max int) ([]string, error)
}

// HistoryDepth bounds the provenance walk; a definition older than this is
// trusted only as human (fail closed).
const HistoryDepth = 30

// Load reads the catalog at sha, which must be on the default branch, and
// resolves each action's effective approval from its merge history.
func Load(ctx context.Context, src Source, sha string) (*Catalog, error) {
	ok, err := src.OnDefault(ctx, sha)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotPinned, sha)
	}
	data, found, err := src.File(ctx, sha, Path)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrNoCatalog
	}
	c, err := Parse(data)
	if err != nil {
		return nil, err
	}
	c.SHA = sha
	var auto []int
	for i, a := range c.Actions {
		if a.Approval == Auto {
			auto = append(auto, i)
		}
	}
	if len(auto) == 0 {
		return c, nil
	}
	hist, err := src.History(ctx, sha, Path, HistoryDepth+1)
	if err != nil {
		return nil, err
	}
	versions := make([]*Catalog, 0, len(hist))
	for _, h := range hist {
		data, found, err := src.File(ctx, h, Path)
		if err != nil {
			return nil, err
		}
		var v *Catalog
		if found {
			v, _ = Parse(data) // an invalid old version ends the walk
		}
		versions = append(versions, v)
	}
	for _, i := range auto {
		a := &c.Actions[i]
		first, complete := firstApproval(*a, versions)
		switch {
		case !complete:
			a.Effective, a.Note = Human, fmt.Sprintf("definition older than the last %d catalog commits; treated as human", HistoryDepth)
		case first != Human:
			a.Effective, a.Note = Human, "this definition first merged as auto; it must merge as human before auto applies"
		}
	}
	return c, nil
}

// firstApproval walks catalog versions newest first while a's definition
// (everything but approval) is unchanged and returns the approval of the
// oldest such version; complete is false when the history ran out first.
func firstApproval(a Action, versions []*Catalog) (approval string, complete bool) {
	want := definition(a)
	approval = a.Approval
	for i, v := range versions {
		var prev Action
		ok := v != nil
		if ok {
			prev, ok = v.Find(a.Name)
		}
		if !ok || definition(prev) != want {
			return approval, i > 0 || len(versions) == 0
		}
		approval = prev.Approval
		if i == HistoryDepth {
			return approval, false
		}
	}
	return approval, true // the file's first commit
}

func definition(a Action) string {
	a.Approval, a.Effective, a.Note, a.Summary = "", "", "", ""
	b, _ := json.Marshal(a)
	return string(b)
}

// ParseParams reads "key=value,key=value".
func ParseParams(s string) (map[string]string, error) {
	out := map[string]string{}
	if strings.TrimSpace(s) == "" {
		return out, nil
	}
	for _, kv := range strings.Split(s, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(kv), "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("%w: %q: want key=value", ErrParams, kv)
		}
		if _, dup := out[k]; dup {
			return nil, fmt.Errorf("%w: %s given twice", ErrParams, k)
		}
		out[k] = v
	}
	return out, nil
}

// Environment picks the environment the action runs in.
func (a Action) Environment(params map[string]string) (string, error) {
	if e, ok := params[EnvironmentParam]; ok {
		if !slices.Contains(a.Environments, e) {
			return "", fmt.Errorf("%w: environment %q: %s may run in %s", ErrParams, e, a.Name, strings.Join(a.Environments, ", "))
		}
		return e, nil
	}
	if len(a.Environments) == 1 {
		return a.Environments[0], nil
	}
	return "", fmt.Errorf("%w: %s runs in %s; pick one with environment=<name>", ErrParams, a.Name, strings.Join(a.Environments, ", "))
}

// Render checks params against the action's types and fills the argv
// template. Values become whole arguments; nothing passes through a shell.
func (a Action) Render(params map[string]string) ([]string, error) {
	var errs []error
	bad := func(format string, args ...any) {
		errs = append(errs, fmt.Errorf("%w: "+format, append([]any{ErrParams}, args...)...))
	}
	for k := range params {
		if _, ok := a.Params[k]; !ok && k != EnvironmentParam {
			bad("%s takes no parameter %q", a.Name, k)
		}
	}
	used := map[string]bool{}
	for _, arg := range a.Argv {
		for _, m := range holeRE.FindAllStringSubmatch(arg, -1) {
			used[m[1]] = true
		}
	}
	for name, p := range a.Params {
		v, ok := params[name]
		if !ok {
			if p.Required || used[name] {
				bad("%s is required", name)
			}
			continue
		}
		switch p.Type {
		case "string":
			if !regexp.MustCompile(p.Pattern).MatchString(v) {
				bad("%s = %q does not match %s", name, v, p.Pattern)
			} else if strings.HasPrefix(v, "-") {
				bad("%s = %q: a value cannot start with -", name, v)
			}
		case "enum":
			if !slices.Contains(p.Values, v) {
				bad("%s = %q: want one of %s", name, v, strings.Join(p.Values, ", "))
			}
		case "int":
			n, err := strconv.Atoi(v)
			switch {
			case err != nil || strconv.Itoa(n) != v:
				bad("%s = %q: want an integer", name, v)
			case p.Min != nil && n < *p.Min, p.Max != nil && n > *p.Max:
				bad("%s = %d is out of range", name, n)
			}
		case "bool":
			if v != "true" && v != "false" {
				bad("%s = %q: want true or false", name, v)
			}
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	argv := make([]string, len(a.Argv))
	for i, arg := range a.Argv {
		argv[i] = holeRE.ReplaceAllStringFunc(arg, func(h string) string { return params[h[1:len(h)-1]] })
	}
	return argv, nil
}
