package initrepo

import (
	"bytes"
	"context"
	"embed"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"text/template"

	"github.com/shurcooL/githubv4"
	"golang.org/x/crypto/nacl/box"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/doctor"
	"github.com/rahulnakmol/agentic-ayudha/internal/hosts"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/posthog"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/sentry"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/seats"
)

// The Ready guard (WP2.7) template init --gates installs.
//
//go:embed aayudha-ready-guard.yml
var readyGuardTemplate string

//go:embed aayudha-streak.yml
var streakTemplate string

//go:embed aayudha-merge-gate.yml
var mergeGateTemplate string

//go:embed aayudha-ops.yml
var opsTemplate string

//go:embed aayudha-approve.yml
var approveTemplate string

// Gate workflows `init --gates` installs: exactly the ones whose WP this binary
// contains (rule 1a): the WP1.4 broker, the WP2.7 Ready guard, the WP4.8 merge gate, the WP5.6 ops executor, the
// WP5.7 streak, and the Story #481 approvals. None calls a model: the WP2.9 triage workflow is retired (doctor.RetiredWorkflows, Appendix B S-2.9b). The WP7.1 loops workflow is rendered from the checkout's loops/*.toml (Extras.Loops).
const (
	TokenWorkflow      = "aayudha-token.yml"
	ReadyGuardWorkflow = "aayudha-ready-guard.yml"
	StreakWorkflow     = "aayudha-streak.yml"
	MergeGateWorkflow  = "aayudha-merge-gate.yml"
	OpsWorkflow        = "aayudha-ops.yml"
	ApproveWorkflow    = "aayudha-approve.yml"
)

// GateWorkflows renders the workflows init --gates installs, by file name.
// version is the aayudha version the guard installs (a tag, or dev).
func GateWorkflows(version string) map[string][]byte {
	return map[string][]byte{
		TokenWorkflow:      identity.BrokerWorkflow,
		ReadyGuardWorkflow: []byte(strings.ReplaceAll(readyGuardTemplate, "{{VERSION}}", GuardVersion(version))),
		StreakWorkflow:     []byte(strings.ReplaceAll(streakTemplate, "{{VERSION}}", GuardVersion(version))),
		MergeGateWorkflow:  []byte(strings.ReplaceAll(mergeGateTemplate, "{{VERSION}}", GuardVersion(version))),
		OpsWorkflow:        []byte(strings.ReplaceAll(opsTemplate, "{{VERSION}}", GuardVersion(version))),
		ApproveWorkflow:    []byte(strings.ReplaceAll(approveTemplate, "{{VERSION}}", GuardVersion(version))),
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
	Projects, Gates, Identity, Integrations, VerifySkill bool
	Version                                              string
	// Assets are the embedded asset trees (hosts/verify-skill/ for --verify-skill).
	Assets fs.FS
	// Maintainer is the agent the human chose to maintain the feature maps
	// (--verify-skill, D-4.5b.8), written as [verify] maintainer; "" writes none.
	Maintainer string
	// Loops are the rendered loops/*.toml of the checkout (WP7.1), by path:
	// the Actions fallback and the Amp and Cursor files; --gates writes them.
	Loops map[string][]byte
	// Confirm asks the human a yes/no question after showing a cost and scope
	// summary; --yes answers it for a human, never for an agent.
	Confirm func(summary string) bool
	// Ask reads one line from the human (bucket URL); "" skips.
	Ask func(prompt string) string
	// Print shows the human a cloud's setup steps (.aayudha/setup/<cloud>/README.md).
	Print func(text string)
	// ManifestCode runs the GitHub App manifest flow in the human's browser and
	// returns the code GitHub redirects back with.
	ManifestCode func(ctx context.Context, postURL string, manifest func(redirect string) string) (string, error)
}

// RunExtras performs the requested extras after the core steps. Each is
// idempotent: a second run finds what the first created and changes nothing.
func (in *Init) RunExtras(ctx context.Context, x Extras) error {
	if err := in.retired(x); err != nil {
		return err
	}
	for _, s := range []struct {
		on   bool
		name string
		run  func(context.Context, Extras) error
	}{{x.Gates, "gates", in.gates}, {x.Projects, "projects", in.projects}, {x.Identity, "identity-app", in.app}, {x.Integrations, "integrations", in.integrations}, {x.VerifySkill, "verify-skill", in.verifySkill}} {
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

// retired reports each workflow an earlier init --gates installed that this
// version does not (doctor.RetiredWorkflows), and removes it when the human
// confirms. It runs on every init, with or without --gates.
func (in *Init) retired(x Extras) error {
	for _, f := range doctor.RetiredAt(in.Root) {
		path := ".github/workflows/" + f.File
		switch {
		case in.dry():
			in.note("retired", "file", path, "planned", "remove: "+f.Why)
			continue
		case x.Confirm == nil || !x.Confirm(fmt.Sprintf("Retired workflow: %s %s. Remove it?", path, f.Why)):
			in.note("retired", "file", path, "kept", "retired: "+f.Why+"; delete it, or rerun init and confirm")
			continue
		}
		if err := os.Remove(filepath.Join(in.Root, filepath.FromSlash(path))); err != nil {
			return err
		}
		in.forgetFile(path)
		in.note("retired", "file", path, "deleted", f.Why)
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
	for _, p := range slices.Sorted(maps.Keys(x.Loops)) {
		if err := in.writeFile("gates", p, x.Loops[p]); err != nil {
			return err
		}
	}
	if err := in.hostConfigOwners(); err != nil {
		return err
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

// ledgerBypassApp adds the bot App to the ledger ruleset's bypass actors, for
// agent lanes, beside the write role: `aayu ship`, run by any human with
// write access, lands the merge gate's pack and the ship record under that
// human's identity (S-4.2b, D-4.2b.h; before 0.2 the App replaced the admin
// role, Appendix B D1).
func (in *Init) ledgerBypassApp(ctx context.Context, appID int64) error {
	o, ok := in.State.Object("ruleset", LedgerRuleset)
	if !ok {
		return nil
	}
	path := "/repos/" + in.Repo.FullName + "/rulesets/" + o.ID
	body := map[string]any{"bypass_actors": []map[string]any{writeRole, {"actor_id": appID, "actor_type": "Integration", "bypass_mode": "always"}}}
	_, err := in.GH.Write(ctx, actions.Action{Kind: "github.ruleset.update", Target: in.Repo.FullName, IdempotencyKey: in.key("ruleset-bypass"),
		Request: &actions.Request{Method: http.MethodPut, Path: path}, Payload: body},
		func(ctx context.Context) (any, error) { return nil, in.rest(ctx, http.MethodPut, path, body, nil) })
	if err == nil {
		in.note("identity", "ruleset", o.Name+" bypass", "written", "the bot App joins the write role as a ledger writer")
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
	{"bucket", "Proof media bucket (S3, R2, Azure Blob, or Cloud Storage; private): required on public repositories (V3). Storage and egress are billed by your provider; aayudha sets lifecycle rules from [proof] retention_days (WP4.2, WP7.2b.2)."},
	// The scopes are the ones aayu doctor checks, so the two never disagree.
	{"posthog", "PostHog: product analytics read for baselines and measure. Read-only personal API key (" + strings.Join(posthog.Scopes(), ", ") + "); usage-based pricing on PostHog's side."},
	{"sentry", "Sentry: error and performance read for baselines, investigate, slo, and runbook. Read-only auth token (" + strings.Join(sentry.Scopes(), ", ") + "); no extra cost."},
	{"growthbook", "GrowthBook: experiment and flag read for experiment and readout. Read-only key; no extra cost on the open-source plan."},
	{"azure", "Azure: Application Insights read (Log Analytics query API) for grill baselines and measure --live. A read-only app registration (Monitoring Reader, Log Analytics Reader) that GitHub Actions reaches through OIDC federation, no stored key; Log Analytics bills per GB scanned, usually cents per run (WP7.2b)."},
	{"aws", "AWS: CloudWatch RUM read (Logs Insights and GetAppMonitorData) for grill baselines and measure --live. A read-only IAM role that GitHub Actions assumes through OIDC, no stored key; Logs Insights bills per GB scanned (WP7.2b)."},
	{"gcp", "Google Cloud: GA4 BigQuery export read for grill baselines and measure --live. A read-only service account (bigquery.dataViewer on the dataset, bigquery.jobUser) that GitHub Actions impersonates through workload identity federation, no stored key; BigQuery bills per TB scanned, with a free monthly tier (WP7.2b)."},
}

// cloudConfig is the [integrations.<cloud>] section init writes for a cloud:
// GitHub Actions OIDC federation, with the identifiers the founder fills in
// from .aayudha/setup/<cloud>/README.md.
var cloudConfig = map[string]string{
	"azure": `[integrations.azure]
scope = "read"
auth = "oidc"   # GitHub Actions federation; locally AAYU_INTEGRATIONS__AZURE__AUTH=cli (see .aayudha/setup/azure/README.md)
org = ""        # Microsoft Entra tenant ID
identity = ""   # application (client) ID of the read-only app registration
project = ""    # resource ID of the Application Insights component
`,
	"aws": `[integrations.aws]
scope = "read"
auth = "oidc"   # GitHub Actions federation; locally AAYU_INTEGRATIONS__AWS__AUTH=cli (see .aayudha/setup/aws/README.md)
identity = ""   # arn:aws:iam::<account-id>:role/<read-only role>
region = ""     # region of the CloudWatch RUM app monitor
project = ""    # CloudWatch RUM app monitor name
`,
	"gcp": `[integrations.gcp]
scope = "read"
auth = "oidc"   # GitHub Actions federation; locally AAYU_INTEGRATIONS__GCP__AUTH=cli (see .aayudha/setup/gcp/README.md)
federation = "" # projects/<number>/locations/global/workloadIdentityPools/<pool>/providers/<provider>
identity = ""   # read-only service account email
project = ""    # <project>.<dataset> of the GA4 BigQuery export
`,
}

//go:embed setup
var setupFS embed.FS

// SetupFiles renders the least-privilege setup for a cloud (azure, aws,
// gcp) for repo r: the founder's steps, the role or policy documents, the
// GitHub Actions OIDC federation, and the workflow permissions, keyed by
// path under .aayudha/setup/<cloud>/.
func SetupFiles(cloud string, r Repo, retentionDays int) (map[string][]byte, error) {
	if retentionDays <= 0 {
		retentionDays = 90
	}
	branch := r.Default
	if branch == "" {
		branch = "main"
	}
	data := map[string]any{"Repo": r.FullName, "Name": r.Name, "Branch": branch, "Cloud": cloud, "Retention": retentionDays,
		"AzureRule": proof.AzureRuleName(r.FullName),
		"Provider":  map[string]string{"azure": "Microsoft Entra", "aws": "AWS STS", "gcp": "Google STS"}[cloud]}
	out := map[string][]byte{}
	render := func(src, name string) error {
		b, err := setupFS.ReadFile(src)
		if err != nil {
			return err
		}
		t, err := template.New(name).Option("missingkey=error").Parse(string(b))
		if err != nil {
			return err
		}
		var buf bytes.Buffer
		if err := t.Execute(&buf, data); err != nil {
			return err
		}
		out[path.Join(".aayudha", "setup", cloud, name)] = buf.Bytes()
		return nil
	}
	entries, err := setupFS.ReadDir("setup/" + cloud)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if err := render("setup/"+cloud+"/"+e.Name(), strings.TrimSuffix(e.Name(), ".tmpl")); err != nil {
			return nil, err
		}
	}
	return out, render("setup/workflow.yml.tmpl", "workflow.yml")
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
			url := strings.TrimSpace(x.Ask("Bucket URL for proof media (s3://…, r2://…, azblob://<account>/<container>, or gs://…; empty to skip): "))
			if url == "" {
				in.note("integrations", "integration", "bucket", "kept", "no bucket given; set [proof] media yourself")
				continue
			}
			if !regexp.MustCompile(`^(s3|r2|azblob|gs)://[a-z0-9][a-z0-9._-]{1,61}[a-z0-9]([/?].*)?$`).MatchString(url) || strings.ContainsAny(url, "\"\n") {
				return fmt.Errorf("bucket %q: want s3://<bucket>, r2://<account>/<bucket>, azblob://<account>/<container>, or gs://<bucket>", url)
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
		section := fmt.Sprintf("[integrations.%s]\nsecret = %q # set the value with: aayu auth set-secret %s\nscope = \"read\"\n", ig.Name, ig.Name, ig.Name)
		if c, ok := cloudConfig[ig.Name]; ok {
			section = c
			if err := in.cloudSetup(ig.Name, cur, x); err != nil {
				return err
			}
		}
		if err := in.appendConfig("integrations."+ig.Name, section); err != nil {
			return err
		}
		in.State.Put(Object{Kind: "integration", Name: ig.Name, ID: ig.Name, External: true})
	}
	return nil
}

// cloudSetup writes .aayudha/setup/<cloud>/ (files already there are left
// alone, so the founder's edits survive a rerun) and prints its README: the
// founder's steps to create the read-only identity.
func (in *Init) cloudSetup(cloud string, cfg []byte, x Extras) error {
	days := 90
	if m := regexp.MustCompile(`(?m)^retention_days\s*=\s*([0-9]+)`).FindSubmatch(cfg); m != nil {
		_, _ = fmt.Sscan(string(m[1]), &days)
	}
	files, err := SetupFiles(cloud, in.Repo, days)
	if err != nil {
		return err
	}
	for _, p := range slices.Sorted(maps.Keys(files)) {
		if _, err := os.Stat(filepath.Join(in.Root, filepath.FromSlash(p))); err == nil {
			in.note("integrations", "file", p, "unchanged", "already present")
			continue
		}
		if err := in.updateFile("integrations", p, files[p]); err != nil {
			return err
		}
	}
	if x.Print != nil {
		x.Print(string(files[path.Join(".aayudha", "setup", cloud, "README.md")]))
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

// The CODEOWNERS block init --gates owns: the files a host runs from the
// checkout (hosts.HostConfigOwners). Lines outside it are never changed, and
// it never loosens one: each path keeps the owners a hand-written line gives it.
const (
	hostConfigSection = "host-config" // its File.Section, so uninit removes only the block
	HostConfigBegin   = "# BEGIN aayudha:host-config (written by `aayu init --gates`: the hooks, MCP servers, and plugins a host runs from this repository)"
	HostConfigEnd     = "# END aayudha:host-config"
)

// hostConfigOwners makes a change to any host's executable config need a code
// owner: a branch can change a hook or MCP command, and a host runs it from
// the checkout. Each pattern is owned by the person who ran init and by every
// owner a hand-written line already gives that path, since the block's line
// is the last match. The block goes before aayu seat's, whose lines win.
func (in *Init) hostConfigOwners() error {
	if !seats.ValidLogin(in.Repo.HumanLogin) {
		in.note("gates", "file", "CODEOWNERS", "kept", "no human login to own the host config")
		return nil
	}
	path := seats.CodeownersPath(in.Root)
	full := filepath.Join(in.Root, filepath.FromSlash(path))
	old, err := os.ReadFile(full)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	next := []byte(RenderHostConfigOwners(string(old), in.Repo.HumanLogin))
	switch {
	case bytes.Equal(old, next):
		in.note("gates", "file", path, "unchanged", "")
		return nil
	case in.dry():
		in.note("gates", "file", path, "planned", "block aayudha:host-config")
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(full, next, 0o644); err != nil {
		return err
	}
	in.State.PutFile(File{Path: path, SHA256: hash(next), Created: len(old) == 0, Section: hostConfigSection})
	in.note("gates", "file", path, "written", "block aayudha:host-config: changes to host hooks and MCP servers need a code owner")
	return nil
}

// RenderHostConfigOwners returns existing CODEOWNERS text with the
// host-config block rebuilt for login, or removed when login is "".
func RenderHostConfigOwners(existing, login string) string {
	var hand, seatsBlock []string
	inHost, inSeats := false, false
	for _, l := range strings.Split(strings.TrimRight(existing, "\n"), "\n") {
		switch {
		case strings.HasPrefix(l, "# BEGIN aayudha:host-config"):
			inHost = true
		case inHost:
			inHost = strings.TrimSpace(l) != HostConfigEnd
		case strings.HasPrefix(l, "# BEGIN aayudha:seats"):
			inSeats = true
			seatsBlock = append(seatsBlock, l)
		case inSeats:
			seatsBlock = append(seatsBlock, l)
			inSeats = strings.TrimSpace(l) != seats.BlockEnd
		default:
			hand = append(hand, l)
		}
	}
	rules := identity.ParseCodeowners(strings.Join(hand, "\n"))
	block := []string{HostConfigBegin}
	for _, pat := range hosts.HostConfigOwners {
		path := strings.TrimPrefix(pat, "/")
		// A path a hand-written line already protects keeps exactly its
		// owners: adding the login would let it alone approve.
		owners := []string{login}
		if o, _, ok := identity.Owners(rules, path); ok && len(o) > 0 {
			owners = o
		}
		block = append(block, pat+"  @"+strings.Join(owners, " @"))
		// A narrower hand-written line under pat would lose to the block's
		// line, the later match: restate it after, with its owners.
		var seen []string
		for _, r := range rules {
			sub := strings.TrimPrefix(r.Pattern, "/")
			if !strings.HasSuffix(path, "/") || sub == path || !strings.HasPrefix(sub, path) || slices.Contains(seen, sub) {
				continue
			}
			seen = append(seen, sub)
			if o, _, ok := identity.Owners(rules, sub); ok && len(o) > 0 {
				block = append(block, r.Pattern+"  @"+strings.Join(o, " @"))
			}
		}
	}
	block = append(block, HostConfigEnd)
	text := strings.TrimRight(strings.Join(hand, "\n"), "\n")
	parts := []string{}
	if strings.TrimSpace(text) != "" {
		parts = append(parts, text)
	}
	if login != "" {
		parts = append(parts, strings.Join(block, "\n"))
	}
	if len(seatsBlock) > 0 {
		parts = append(parts, strings.Join(seatsBlock, "\n"))
	}
	return strings.Join(parts, "\n\n") + "\n"
}
