package initrepo

import (
	"context"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/shurcooL/githubv4"
	"golang.org/x/crypto/nacl/box"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
)

// The Ready guard (WP2.7) and triage (WP2.9) templates init --gates installs.
//
//go:embed aayudha-ready-guard.yml
var readyGuardTemplate string

//go:embed aayudha-triage.yml
var triageTemplate string

//go:embed aayudha-streak.yml
var streakTemplate string

// Gate workflows `init --gates` installs: exactly the ones whose WP this binary
// contains (rule 1a): the WP1.4 broker, the WP2.7 Ready guard, the WP2.9 triage, and the WP5.7 streak. The merge gate (WP4.8) and loop
// workflows (WP7.1) join this list in their own WPs.
const (
	TokenWorkflow      = "aayudha-token.yml"
	ReadyGuardWorkflow = "aayudha-ready-guard.yml"
	TriageWorkflow     = "aayudha-triage.yml"
	StreakWorkflow     = "aayudha-streak.yml"
)

// GateWorkflows renders the workflows init --gates installs, by file name.
// version is the aayudha version the guard installs (a tag, or dev).
func GateWorkflows(version string) map[string][]byte {
	return map[string][]byte{
		TokenWorkflow:      identity.BrokerWorkflow,
		ReadyGuardWorkflow: []byte(strings.ReplaceAll(readyGuardTemplate, "{{VERSION}}", GuardVersion(version))),
		TriageWorkflow:     []byte(strings.ReplaceAll(triageTemplate, "{{VERSION}}", GuardVersion(version))),
		StreakWorkflow:     []byte(strings.ReplaceAll(streakTemplate, "{{VERSION}}", GuardVersion(version))),
	}
}

var releaseVersion = regexp.MustCompile(`^v?[0-9]+\.[0-9]+\.[0-9]+$`)

// GuardVersion is the module version a workflow installs: the release tag of a
// release binary, else the dev branch (development builds have no tag).
func GuardVersion(v string) string {
	if releaseVersion.MatchString(v) {
		return "v" + strings.TrimPrefix(v, "v")
	}
	return "dev"
}

// Extras are the opt-in init flags (WP2.6).
type Extras struct {
	Projects, Gates, Identity, Integrations bool
	Version                                 string
	// Confirm asks the human a yes/no question after showing a cost and scope
	// summary; --yes answers it for a human, never for an agent.
	Confirm func(summary string) bool
	// Ask reads one line from the human (bucket URL); "" skips.
	Ask func(prompt string) string
	// ManifestCode runs the GitHub App manifest flow in the human's browser and
	// returns the code GitHub redirects back with.
	ManifestCode func(ctx context.Context, postURL string, manifest func(redirect string) string) (string, error)
}

// RunExtras performs the requested extras after the core steps. Each is
// idempotent: a second run finds what the first created and changes nothing.
func (in *Init) RunExtras(ctx context.Context, x Extras) error {
	for _, s := range []struct {
		on   bool
		name string
		run  func(context.Context, Extras) error
	}{{x.Gates, "gates", in.gates}, {x.Projects, "projects", in.projects}, {x.Identity, "identity-app", in.app}, {x.Integrations, "integrations", in.integrations}} {
		if !s.on {
			continue
		}
		if err := s.run(ctx, x); err != nil {
			_ = in.save()
			return fmt.Errorf("init --%s: %w", strings.TrimSuffix(s.name, "-app"), err)
		}
		in.State.MarkDone(s.name)
		if err := in.save(); err != nil {
			return err
		}
	}
	return nil
}

func (in *Init) gates(_ context.Context, x Extras) error {
	wfs := GateWorkflows(x.Version)
	for _, name := range slices.Sorted(func(y func(string) bool) {
		for k := range wfs {
			if !y(k) {
				return
			}
		}
	}) {
		if err := in.writeFile("gates", ".github/workflows/"+name, wfs[name]); err != nil {
			return err
		}
	}
	in.note("gates", "secrets", "AAYUDHA_APP_CLIENT_ID, AAYUDHA_APP_PRIVATE_KEY", "unchanged",
		"the broker needs the bot App's client ID (repository variable) and private key (repository secret); init --identity sets both. Model keys for CI lanes go in repository or environment secrets")
	return nil
}

type projectNode struct {
	ID    githubv4.ID
	Title string
}

// Project views init --projects creates. Projects v2 views cannot be created
// through the API, so each view is its own project (Appendix B D-2.6a).
var ProjectViews = []string{"Portfolio", "Delivery", "Ops"}

func (in *Init) projects(ctx context.Context, x Extras) error {
	r := in.Repo
	var q struct {
		RepositoryOwner struct {
			ID    githubv4.ID
			Owner struct {
				ProjectsV2 struct{ Nodes []projectNode } `graphql:"projectsV2(first: 100)"`
			} `graphql:"... on ProjectV2Owner"`
		} `graphql:"repositoryOwner(login: $owner)"`
		Repository struct{ ID githubv4.ID } `graphql:"repository(owner: $owner, name: $name)"`
	}
	if err := in.GH.GraphQL.Query(ctx, &q, map[string]any{"owner": githubv4.String(r.Owner), "name": githubv4.String(r.Name)}); err != nil {
		return err
	}
	nodes := q.RepositoryOwner.Owner.ProjectsV2.Nodes
	missing := 0
	for _, view := range ProjectViews {
		if !slices.ContainsFunc(nodes, func(n projectNode) bool { return n.Title == r.Name+" "+view }) {
			missing++
		}
	}
	if missing > 0 && !x.Confirm(fmt.Sprintf("Projects: creates %d GitHub Projects (%s) owned by %s and links them to %s. Free; needs a token with the project scope. Views only: no field is written as truth.",
		missing, strings.Join(ProjectViews, ", "), r.Owner, r.FullName)) {
		in.note("projects", "project", "*", "kept", "not confirmed")
		return nil
	}
	for _, view := range ProjectViews {
		title := r.Name + " " + view
		if o, ok := in.State.Object("project", title); ok && slices.ContainsFunc(nodes, func(n projectNode) bool { return fmt.Sprint(n.ID) == o.ID }) {
			in.note("projects", "project", title, "unchanged", "")
			continue
		}
		if i := slices.IndexFunc(nodes, func(n projectNode) bool { return n.Title == title }); i >= 0 {
			in.State.Put(Object{Kind: "project", Name: title, ID: fmt.Sprint(nodes[i].ID), Adopted: true, External: true})
			in.note("projects", "project", title, "adopted", "")
			continue
		}
		var m struct {
			CreateProjectV2 struct {
				ProjectV2 struct{ ID githubv4.ID }
			} `graphql:"createProjectV2(input: $input)"`
		}
		rec, err := in.GH.Write(ctx, actions.Action{Kind: "github.project.create", Target: r.Owner, IdempotencyKey: in.key("project", view),
			Request: &actions.Request{Method: http.MethodPost, Path: "/graphql"}, Payload: map[string]any{"title": title}},
			func(ctx context.Context) (any, error) {
				input := githubv4.CreateProjectV2Input{OwnerID: q.RepositoryOwner.ID, Title: githubv4.String(title), RepositoryID: &q.Repository.ID}
				err := in.GH.GraphQL.Mutate(ctx, &m, input, nil)
				return fmt.Sprint(m.CreateProjectV2.ProjectV2.ID), err
			})
		if err != nil {
			return err
		}
		if in.recordCreated(rec, Object{Kind: "project", Name: title, ID: fmt.Sprint(m.CreateProjectV2.ProjectV2.ID), External: true}, "projects") {
			if err := in.save(); err != nil {
				return err
			}
		}
	}
	return nil
}

// AppManifest is the GitHub App manifest init --identity registers: the
// permissions agent lanes and CI need, no webhook.
func AppManifest(r Repo, redirect string) string {
	name := "aayudha-" + r.Name
	if len(name) > 34 {
		name = name[:34]
	}
	return fmt.Sprintf(`{"name":%q,"url":"https://github.com/rahulnakmol/agentic-ayudha","redirect_url":%q,"public":false,`+
		`"hook_attributes":{"url":"https://example.invalid/aayudha-no-webhook","active":false},`+
		`"default_permissions":{"contents":"write","issues":"write","pull_requests":"write","actions":"read","checks":"read","metadata":"read"}}`, name, redirect)
}

func (in *Init) app(ctx context.Context, x Extras) error {
	r := in.Repo
	if o, ok := in.State.Object("app", "bot"); ok {
		in.note("identity", "app", o.Name, "unchanged", "")
		return nil
	}
	if in.Repo.BotKind != "" {
		in.note("identity", "app", in.Repo.BotKind, "unchanged", "a bot identity is already configured")
		return nil
	}
	if !x.Confirm("Bot identity: registers a private GitHub App owned by " + r.Owner + " with contents, issues, and pull requests write, actions and checks read. " +
		"Free. Its private key goes straight into the repository secret AAYUDHA_APP_PRIVATE_KEY and never touches disk; you then install the App on " + r.FullName + ".") {
		in.note("identity", "app", "*", "kept", "not confirmed")
		return nil
	}
	if in.dry() {
		in.note("identity", "app", "aayudha-"+r.Name, "planned", "manifest flow in the browser")
		return nil
	}
	post := "https://github.com/settings/apps/new"
	if r.Org {
		post = "https://github.com/organizations/" + r.Owner + "/settings/apps/new"
	}
	code, err := x.ManifestCode(ctx, post, func(redirect string) string { return AppManifest(r, redirect) })
	if err != nil {
		return err
	}
	var conv struct {
		ID       int64  `json:"id"`
		Slug     string `json:"slug"`
		ClientID string `json:"client_id"`
		PEM      string `json:"pem"`
		HTMLURL  string `json:"html_url"`
	}
	_, err = in.GH.Write(ctx, actions.Action{Kind: "github.app.create", Target: r.Owner, IdempotencyKey: in.key("app"),
		Request: &actions.Request{Method: http.MethodPost, Path: "/app-manifests/" + code + "/conversions"}},
		func(ctx context.Context) (any, error) {
			err := in.rest(ctx, http.MethodPost, "/app-manifests/"+code+"/conversions", nil, &conv)
			return map[string]any{"id": conv.ID, "slug": conv.Slug}, err // the PEM is never logged
		})
	if err != nil {
		return err
	}
	in.State.Put(Object{Kind: "app", Name: "bot", ID: strconv.FormatInt(conv.ID, 10), External: true})
	in.State.Identity = identity.KindApp
	if err := in.save(); err != nil {
		return err
	}
	if err := in.putSecret(ctx, "AAYUDHA_APP_PRIVATE_KEY", conv.PEM); err != nil {
		return err
	}
	conv.PEM = ""
	if err := in.putVariable(ctx, "AAYUDHA_APP_CLIENT_ID", conv.ClientID); err != nil {
		return err
	}
	if err := in.appendConfig("identity", fmt.Sprintf("[identity]\napp_id = %d\napp_slug = %q\nbroker = %q\n", conv.ID, conv.Slug, TokenWorkflow)); err != nil {
		return err
	}
	in.note("identity", "app", conv.Slug, "created", "install it on "+r.FullName+": https://github.com/apps/"+conv.Slug+"/installations/new")
	return in.ledgerBypassApp(ctx, conv.ID)
}

// ledgerBypassApp makes the bot App the ledger ruleset's bypass actor in place
// of the admin role (Appendix B D1: "init --identity replaces the admin bypass").
func (in *Init) ledgerBypassApp(ctx context.Context, appID int64) error {
	o, ok := in.State.Object("ruleset", LedgerRuleset)
	if !ok {
		return nil
	}
	path := "/repos/" + in.Repo.FullName + "/rulesets/" + o.ID
	body := map[string]any{"bypass_actors": []map[string]any{{"actor_id": appID, "actor_type": "Integration", "bypass_mode": "always"}}}
	_, err := in.GH.Write(ctx, actions.Action{Kind: "github.ruleset.update", Target: in.Repo.FullName, IdempotencyKey: in.key("ruleset-bypass"),
		Request: &actions.Request{Method: http.MethodPut, Path: path}, Payload: body},
		func(ctx context.Context) (any, error) { return nil, in.rest(ctx, http.MethodPut, path, body, nil) })
	if err == nil {
		in.note("identity", "ruleset", o.Name+" bypass", "written", "the bot App replaces the admin role as the ledger writer")
	}
	return err
}

// putSecret stores a repository Actions secret, sealed to the repository's
// public key (libsodium sealed box); the value is never logged.
func (in *Init) putSecret(ctx context.Context, name, value string) error {
	r := in.Repo
	var pk struct {
		KeyID string `json:"key_id"`
		Key   string `json:"key"`
	}
	if err := in.rest(ctx, http.MethodGet, "/repos/"+r.FullName+"/actions/secrets/public-key", nil, &pk); err != nil {
		return err
	}
	raw, err := base64.StdEncoding.DecodeString(pk.Key)
	if err != nil {
		return fmt.Errorf("repository public key: %w", err)
	}
	if len(raw) != 32 {
		return fmt.Errorf("repository public key: %d bytes, want 32", len(raw))
	}
	var key [32]byte
	copy(key[:], raw)
	sealed, err := box.SealAnonymous(nil, []byte(value), &key, nil)
	if err != nil {
		return err
	}
	path := "/repos/" + r.FullName + "/actions/secrets/" + name
	body := map[string]any{"encrypted_value": base64.StdEncoding.EncodeToString(sealed), "key_id": pk.KeyID}
	_, err = in.GH.Write(ctx, actions.Action{Kind: "github.secret.put", Target: r.FullName, Request: &actions.Request{Method: http.MethodPut, Path: path}, Payload: map[string]any{"name": name}},
		func(ctx context.Context) (any, error) { return nil, in.rest(ctx, http.MethodPut, path, body, nil) })
	if err == nil {
		in.State.Put(Object{Kind: "secret", Name: name, ID: name})
	}
	return err
}

func (in *Init) putVariable(ctx context.Context, name, value string) error {
	r := in.Repo
	path := "/repos/" + r.FullName + "/actions/variables"
	body := map[string]any{"name": name, "value": value}
	_, err := in.GH.Write(ctx, actions.Action{Kind: "github.variable.create", Target: r.FullName, Request: &actions.Request{Method: http.MethodPost, Path: path}, Payload: body},
		func(ctx context.Context) (any, error) { return nil, in.rest(ctx, http.MethodPost, path, body, nil) })
	if err == nil {
		in.State.Put(Object{Kind: "variable", Name: name, ID: name})
	}
	return err
}

// appendConfig adds a section to .aayudha/config.toml unless it is there already.
func (in *Init) appendConfig(section, text string) error {
	full := filepath.Join(in.Root, ConfigPath)
	cur, err := os.ReadFile(full)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if regexp.MustCompile(`(?m)^\[` + regexp.QuoteMeta(section) + `\]`).Match(cur) {
		in.note("config", "file", ConfigPath+" ["+section+"]", "unchanged", "")
		return nil
	}
	next := strings.TrimRight(string(cur), "\n") + "\n\n" + text
	if len(cur) == 0 {
		next = "version = 1\n\n" + text
	}
	return in.updateFile("config", ConfigPath, []byte(next))
}

// Integration is an external service init --integrations configures.
type Integration struct {
	Name    string
	Summary string // cost and scope
}

// Integrations lists the external services init --integrations offers.
var Integrations = []Integration{
	{"bucket", "Proof media bucket (S3 or R2, private): required on public repositories (V3). Storage and egress are billed by your provider; aayudha sets lifecycle rules from [proof] retention_days (WP4.2)."},
	{"posthog", "PostHog: product analytics read for baselines and measure. Read-only project API key; usage-based pricing on PostHog's side."},
	{"sentry", "Sentry: error and performance read for baselines and respond. Read-only auth token (event:read, project:read); no extra cost."},
	{"growthbook", "GrowthBook: experiment and flag read for experiment and readout. Read-only key; no extra cost on the open-source plan."},
}

func (in *Init) integrations(_ context.Context, x Extras) error {
	full := filepath.Join(in.Root, ConfigPath)
	for _, ig := range Integrations {
		cur, _ := os.ReadFile(full)
		if ig.Name == "bucket" {
			if regexp.MustCompile(`(?m)^media = "[^"]+`).Match(cur) {
				in.note("integrations", "integration", "bucket", "unchanged", "[proof] media is set")
				continue
			}
			if !x.Confirm(ig.Summary) {
				in.note("integrations", "integration", "bucket", "kept", "not confirmed")
				continue
			}
			url := strings.TrimSpace(x.Ask("Bucket URL for proof media (s3://… or r2://…; empty to skip): "))
			if url == "" {
				in.note("integrations", "integration", "bucket", "kept", "no bucket given; set [proof] media yourself")
				continue
			}
			if !regexp.MustCompile(`^(s3|r2)://[a-z0-9][a-z0-9.-]{1,61}[a-z0-9](/.*)?$`).MatchString(url) {
				return fmt.Errorf("bucket %q: want s3://<bucket> or r2://<bucket>", url)
			}
			next := regexp.MustCompile(`(?m)^media = "".*$`).ReplaceAll(cur, []byte(fmt.Sprintf("media = %q # private bucket (V3), set by init --integrations", url)))
			if string(next) == string(cur) {
				next = []byte(strings.TrimRight(string(cur), "\n") + fmt.Sprintf("\n\n[proof]\nmedia = %q # private bucket (V3), set by init --integrations\n", url))
			}
			if err := in.updateFile("integrations", ConfigPath, next); err != nil {
				return err
			}
			in.State.Put(Object{Kind: "integration", Name: "bucket", ID: url, External: true})
			continue
		}
		if regexp.MustCompile(`(?m)^\[integrations\.` + ig.Name + `\]`).Match(cur) {
			in.note("integrations", "integration", ig.Name, "unchanged", "")
			continue
		}
		if !x.Confirm(ig.Summary) {
			in.note("integrations", "integration", ig.Name, "kept", "not confirmed")
			continue
		}
		if err := in.appendConfig("integrations."+ig.Name, fmt.Sprintf("[integrations.%s]\nsecret = %q # set the value with: aayu auth set-secret %s\nscope = \"read\"\n", ig.Name, ig.Name, ig.Name)); err != nil {
			return err
		}
		in.State.Put(Object{Kind: "integration", Name: ig.Name, ID: ig.Name, External: true})
	}
	return nil
}

// updateFile writes a file init edits in place (config sections it adds),
// whoever created it.
func (in *Init) updateFile(step, path string, content []byte) error {
	if in.dry() {
		in.note(step, "file", path, "planned", "")
		return nil
	}
	full := filepath.Join(in.Root, filepath.FromSlash(path))
	_, statErr := os.Stat(full)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(full, content, 0o644); err != nil {
		return err
	}
	in.State.PutFile(File{Path: path, SHA256: hash(content), Created: errors.Is(statErr, os.ErrNotExist)})
	in.note(step, "file", path, "written", "")
	return nil
}
