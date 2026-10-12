package proof

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/cloud"
)

// MediaOptions are what OpenMedia needs besides the [proof] media value.
type MediaOptions struct {
	// Secret returns the value of [proof] media_secret ("" when unset).
	Secret     func() string
	SecretName string // for messages
	HTTP       *http.Client
	Now        func() time.Time
	// Env is the environment for oidc (GitHub Actions) and cli (az,
	// gcloud) credentials.
	Env cloud.Env
}

// ErrMediaSecret means the store needs [proof] media_secret and it is
// missing or malformed.
var ErrMediaSecret = errors.New("proof media credential")

// OpenMedia opens the store a [proof] media value names:
//
//	s3://bucket[/prefix][?region=…&endpoint=…]         secret ACCESS_KEY_ID:SECRET_ACCESS_KEY
//	r2://<account-id>/bucket[/prefix]                    secret ACCESS_KEY_ID:SECRET_ACCESS_KEY
//	azblob://<account>/<container>[/prefix][?auth=sas|key|oidc|cli&tenant=…&client=…&subscription=…&group=…]
//	gs://<bucket>[/prefix][?auth=key|oidc|cli&provider=…&sa=…]
//
// Azure: auth sas (the default) takes a SAS token as the secret; key an
// app registration's <client-id>:<client-secret> with tenant; oidc a
// GitHub Actions federated credential with tenant and client; cli the az
// login. subscription and group (the storage account's resource group) let
// an Entra credential manage the lifecycle rule through Azure Resource
// Manager. GCS: auth key (the default) takes a service account JSON key;
// oidc a workload identity provider and the service account to
// impersonate; cli the gcloud login (sa optional). Both take endpoint= for
// an emulator (Azurite, fake-gcs-server).
func OpenMedia(v string, o MediaOptions) (Media, error) {
	u, err := url.Parse(v)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("[proof] media %q: want s3://, r2://, azblob://, or gs://", v)
	}
	if o.Env.Now == nil {
		o.Env.Now = o.Now
	}
	secret := ""
	if o.Secret != nil {
		secret = strings.TrimSpace(o.Secret())
	}
	needSecret := func(want string) error {
		if secret == "" {
			return fmt.Errorf("%w: [proof] media is %s, but secret %q (%s) is not set: `aayu auth set-secret %s`", ErrMediaSecret, v, o.SecretName, want, o.SecretName)
		}
		return nil
	}
	switch u.Scheme {
	case "s3", "r2":
		b, err := ParseMedia(v)
		if err != nil {
			return nil, err
		}
		if err := needSecret("ACCESS_KEY_ID:SECRET_ACCESS_KEY"); err != nil {
			return nil, err
		}
		ak, sk, ok := strings.Cut(secret, ":")
		if !ok {
			return nil, fmt.Errorf("%w: secret %q must be ACCESS_KEY_ID:SECRET_ACCESS_KEY", ErrMediaSecret, o.SecretName)
		}
		b.AccessKey, b.SecretKey, b.HTTP, b.Now = ak, sk, o.HTTP, o.Now
		return b, nil
	case "azblob":
		a, err := parseAzureBlob(v, u)
		if err != nil {
			return nil, err
		}
		a.HTTP, a.env = o.HTTP, o.Env
		switch a.Auth {
		case "sas":
			if err := needSecret("a SAS token for the container"); err != nil {
				return nil, err
			}
			a.sas = strings.TrimPrefix(secret, "?")
			if q, err := url.ParseQuery(a.sas); err != nil || q.Get("sig") == "" || q.Get("sv") == "" {
				return nil, fmt.Errorf("%w: secret %q is not a SAS token (sv=…&sig=…)", ErrMediaSecret, o.SecretName)
			}
		case "key":
			if err := needSecret("<client-id>:<client-secret> of an app registration"); err != nil {
				return nil, err
			}
			var ok bool
			if a.id.Client, a.id.Secret, ok = strings.Cut(secret, ":"); !ok {
				return nil, fmt.Errorf("%w: secret %q must be <client-id>:<client-secret>", ErrMediaSecret, o.SecretName)
			}
		}
		return a, nil
	case "gs":
		g, err := parseGCS(v, u)
		if err != nil {
			return nil, err
		}
		g.HTTP, g.env = o.HTTP, o.Env
		if g.id.Auth == "key" {
			if err := needSecret("a service account JSON key"); err != nil {
				return nil, err
			}
			if g.id.Key, err = cloud.ParseServiceAccountKey(secret); err != nil {
				return nil, fmt.Errorf("%w: secret %q: %w", ErrMediaSecret, o.SecretName, err)
			}
		}
		return g, nil
	}
	return nil, fmt.Errorf("[proof] media %q: scheme %q is not a bucket (want s3, r2, azblob, or gs)", v, u.Scheme)
}

var (
	azAccount   = regexp.MustCompile(`^[a-z0-9]{3,24}$`)
	azContainer = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9]|-[a-z0-9]){2,62}$`)
	azGUID      = regexp.MustCompile(`^[0-9a-fA-F-]{36}$`)
	azGroup     = regexp.MustCompile(`^[A-Za-z0-9._()-]{1,90}$`)
	gsBucket    = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{1,220}[a-z0-9]$`)
	gsProvider  = regexp.MustCompile(`^projects/[0-9]+/locations/global/workloadIdentityPools/[a-z0-9-]+/providers/[a-z0-9-]+$`)
	gsAccount   = regexp.MustCompile(`^[a-z0-9-]+@[a-z0-9-]+\.iam\.gserviceaccount\.com$`)
)

func endpoint(v string, q url.Values, def string) (string, error) {
	e := q.Get("endpoint")
	if e == "" {
		return def, nil
	}
	if u, err := url.Parse(e); err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return "", fmt.Errorf("[proof] media %q: endpoint %q is not an http(s) URL", v, e)
	}
	return strings.TrimRight(e, "/"), nil
}

// httpDo sends req and returns the body of a 2xx answer; 404 is ErrNoObject.
// Messages never carry the request's query (a SAS token).
func httpDo(hc *http.Client, req *http.Request, what string) ([]byte, error) {
	if hc == nil {
		hc = &http.Client{Timeout: 5 * time.Minute}
	}
	resp, err := hc.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return nil, fmt.Errorf("%s %s %s: %w", what, req.Method, req.URL.Path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	out, err := io.ReadAll(io.LimitReader(resp.Body, 1<<30))
	if err != nil {
		return nil, err
	}
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, ErrNoObject
	case resp.StatusCode >= 300:
		msg := strings.TrimSpace(string(out))
		if len(msg) > 300 {
			msg = msg[:300]
		}
		return nil, fmt.Errorf("%s %s %s: HTTP %d: %s", what, req.Method, req.URL.Path, resp.StatusCode, msg)
	}
	return out, nil
}

// AzureBlob is a proof media store in an Azure Blob Storage container. Its
// lifecycle rule is a rule of the storage account's management policy,
// which only Azure Resource Manager serves.
type AzureBlob struct {
	URL, Endpoint, Account, Container, Prefix string
	Auth                                      string // sas, key, oidc, cli
	Subscription, Group                       string // for the management policy (Entra credentials only)
	HTTP                                      *http.Client

	id     cloud.Entra
	sas    string
	env    cloud.Env
	tokens cloud.Cache
}

// ARMEndpoint serves storage account management policies.
const ARMEndpoint = "https://management.azure.com"

const (
	azStorageScope = "https://storage.azure.com/.default"
	azARMScope     = "https://management.azure.com/.default"
	azVersion      = "2023-11-03"
	azPolicyAPI    = "2023-05-01"
)

func parseAzureBlob(v string, u *url.URL) (*AzureBlob, error) {
	q := u.Query()
	container, prefix, _ := strings.Cut(strings.Trim(u.Path, "/"), "/")
	a := &AzureBlob{URL: v, Account: u.Host, Container: container, Prefix: prefix, Auth: q.Get("auth"),
		Subscription: q.Get("subscription"), Group: q.Get("group")}
	if a.Auth == "" {
		a.Auth = "sas"
	}
	a.id = cloud.Entra{Auth: a.Auth, Tenant: q.Get("tenant"), Client: q.Get("client")}
	switch {
	case !azAccount.MatchString(a.Account):
		return nil, fmt.Errorf("[proof] media %q: want azblob://<storage account>/<container>[/prefix]", v)
	case !azContainer.MatchString(a.Container):
		return nil, fmt.Errorf("[proof] media %q: container %q: want 3-63 lowercase letters, digits, and single hyphens", v, a.Container)
	case !slices.Contains([]string{"sas", "key", "oidc", "cli"}, a.Auth):
		return nil, fmt.Errorf("[proof] media %q: auth %q: want sas, key, oidc, or cli", v, a.Auth)
	case (a.Auth == "key" || a.Auth == "oidc") && a.id.Tenant == "":
		return nil, fmt.Errorf("[proof] media %q: auth %s needs tenant=<Entra tenant ID>", v, a.Auth)
	case a.Auth == "oidc" && !azGUID.MatchString(a.id.Client):
		return nil, fmt.Errorf("[proof] media %q: auth oidc needs client=<application (client) ID>", v)
	case (a.Subscription == "") != (a.Group == ""):
		return nil, fmt.Errorf("[proof] media %q: subscription and group go together (the storage account's subscription and resource group)", v)
	case a.Subscription != "" && (!azGUID.MatchString(a.Subscription) || !azGroup.MatchString(a.Group)):
		return nil, fmt.Errorf("[proof] media %q: subscription=<subscription ID>&group=<resource group>", v)
	}
	var err error
	a.Endpoint, err = endpoint(v, q, "https://"+a.Account+".blob.core.windows.net")
	return a, err
}

// Key implements Media.
func (a *AzureBlob) Key(repo string, story int, sha string) string {
	return objectKey(a.Prefix, repo, story, sha)
}

func (a *AzureBlob) token(ctx context.Context, scope string) (string, error) {
	return a.tokens.Get(scope, a.env.Clock(), func() (cloud.Token, error) { return a.env.Entra(ctx, a.HTTP, a.id, scope) })
}

func (a *AzureBlob) blob(ctx context.Context, method, key string, body []byte) ([]byte, error) {
	u := a.Endpoint + "/" + a.Container + "/" + escapeKey(key)
	if a.Auth == "sas" {
		u += "?" + a.sas
	}
	req, err := http.NewRequestWithContext(ctx, method, u, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.ContentLength = int64(len(body))
	req.Header.Set("x-ms-version", azVersion)
	req.Header.Set("x-ms-date", a.env.Clock().UTC().Format(http.TimeFormat))
	if method == http.MethodPut {
		req.Header.Set("x-ms-blob-type", "BlockBlob")
		req.Header.Set("Content-Type", "application/octet-stream")
	}
	if a.Auth != "sas" {
		tok, err := a.token(ctx, azStorageScope)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	return httpDo(a.HTTP, req, "azure blob "+a.Account)
}

func escapeKey(k string) string {
	parts := strings.Split(k, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

// Put implements Media.
func (a *AzureBlob) Put(ctx context.Context, key string, data []byte) error {
	_, err := a.blob(ctx, http.MethodPut, key, data)
	return err
}

// Get implements Media.
func (a *AzureBlob) Get(ctx context.Context, key string) ([]byte, error) {
	return a.blob(ctx, http.MethodGet, key, nil)
}

// Delete implements Media.
func (a *AzureBlob) Delete(ctx context.Context, key string) error {
	_, err := a.blob(ctx, http.MethodDelete, key, nil)
	if errors.Is(err, ErrNoObject) {
		return nil
	}
	return err
}

// AzureRuleName is aayudha's management policy rule for repo: alphanumeric,
// as Azure requires, and stable per repository.
func AzureRuleName(repo string) string {
	sum := sha256.Sum256([]byte(repo))
	return "aayudhaproofretention" + hex.EncodeToString(sum[:6])
}

func (a *AzureBlob) policyURL() string {
	return ARMEndpoint + "/subscriptions/" + a.Subscription + "/resourceGroups/" + a.Group +
		"/providers/Microsoft.Storage/storageAccounts/" + a.Account + "/managementPolicies/default?api-version=" + azPolicyAPI
}

func (a *AzureBlob) arm(ctx context.Context, method string, body []byte) ([]byte, error) {
	if a.Auth == "sas" || a.Subscription == "" {
		why := "a SAS token cannot reach Azure Resource Manager"
		if a.Auth != "sas" {
			why = "[proof] media has no subscription= and group="
		}
		return nil, fmt.Errorf("%w: %s, which holds the storage account's lifecycle policy; apply the rule in .aayudha/setup/azure/ (delete blobs under %s after [proof] retention_days), or use an Entra credential with subscription= and group=",
			ErrLifecycleUnmanaged, why, a.Container+"/"+repoPrefix(a.Prefix, "<owner>/<repo>"))
	}
	tok, err := a.token(ctx, azARMScope)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, a.policyURL(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.ContentLength = int64(len(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	return httpDo(a.HTTP, req, "azure management policy "+a.Account)
}

type azRule struct {
	Name string          `json:"name"`
	Raw  json.RawMessage `json:"-"`
}

func (a *AzureBlob) rules(ctx context.Context) ([]json.RawMessage, error) {
	raw, err := a.arm(ctx, http.MethodGet, nil)
	if errors.Is(err, ErrNoObject) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var p struct {
		Properties struct {
			Policy struct {
				Rules []json.RawMessage `json:"rules"`
			} `json:"policy"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("azure management policy: %w", err)
	}
	return p.Properties.Policy.Rules, nil
}

func (a *AzureBlob) rule(repo string, days int) json.RawMessage {
	r, _ := json.Marshal(map[string]any{"enabled": true, "name": AzureRuleName(repo), "type": "Lifecycle", "definition": map[string]any{
		"actions": map[string]any{"baseBlob": map[string]any{"delete": map[string]int{"daysAfterCreationGreaterThan": days}}},
		"filters": map[string]any{"blobTypes": []string{"blockBlob"}, "prefixMatch": []string{a.Container + "/" + repoPrefix(a.Prefix, repo)}},
	}})
	return r
}

func ruleNamed(r json.RawMessage) string {
	var x azRule
	_ = json.Unmarshal(r, &x)
	return x.Name
}

func sameJSON(a, b []byte) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	xa, _ := json.Marshal(x)
	ya, _ := json.Marshal(y)
	return bytes.Equal(xa, ya)
}

// EnsureLifecycle implements Media: one rule in the storage account's
// management policy, merged with the others (a PUT replaces the policy).
func (a *AzureBlob) EnsureLifecycle(ctx context.Context, repo string, days int) (bool, error) {
	cur, err := a.rules(ctx)
	if err != nil {
		return false, err
	}
	name, want := AzureRuleName(repo), a.rule(repo, days)
	var rules []json.RawMessage
	found := false
	for _, r := range cur {
		if ruleNamed(r) == name {
			found = true
			if days > 0 && sameJSON(r, want) {
				return false, nil
			}
			continue
		}
		rules = append(rules, r)
	}
	if days == 0 && !found {
		return false, nil
	}
	if days > 0 {
		rules = append(rules, want)
	}
	if len(rules) == 0 {
		_, err = a.arm(ctx, http.MethodDelete, nil)
		return err == nil, err
	}
	body, _ := json.Marshal(map[string]any{"properties": map[string]any{"policy": map[string]any{"rules": rules}}})
	_, err = a.arm(ctx, http.MethodPut, body)
	return err == nil, err
}

// Lifecycle implements Media.
func (a *AzureBlob) Lifecycle(ctx context.Context, repo string) (string, error) {
	cur, err := a.rules(ctx)
	if err != nil {
		return "", err
	}
	for _, r := range cur {
		if ruleNamed(r) == AzureRuleName(repo) {
			return string(r), nil
		}
	}
	return "", nil
}

// GCS is a proof media store in a Google Cloud Storage bucket, through the
// JSON API with an OAuth token. Its lifecycle rule is a Delete rule on the
// bucket matching the repository's prefix.
type GCS struct {
	URL, Endpoint, Bucket, Prefix string
	HTTP                          *http.Client

	id     cloud.Google
	env    cloud.Env
	tokens cloud.Cache
}

// GCSScopes are the scopes a minted token carries: objects and the
// bucket's lifecycle (buckets.patch needs full control).
var GCSScopes = []string{"https://www.googleapis.com/auth/devstorage.full_control"}

func parseGCS(v string, u *url.URL) (*GCS, error) {
	q := u.Query()
	g := &GCS{URL: v, Bucket: u.Host, Prefix: strings.Trim(u.Path, "/"),
		id: cloud.Google{Auth: q.Get("auth"), Federation: q.Get("provider"), ServiceAccount: q.Get("sa")}}
	if g.id.Auth == "" {
		g.id.Auth = "key"
	}
	switch {
	case !gsBucket.MatchString(g.Bucket):
		return nil, fmt.Errorf("[proof] media %q: want gs://<bucket>[/prefix]", v)
	case !slices.Contains([]string{"key", "oidc", "cli"}, g.id.Auth):
		return nil, fmt.Errorf("[proof] media %q: auth %q: want key, oidc, or cli", v, g.id.Auth)
	case g.id.Auth == "oidc" && !gsProvider.MatchString(g.id.Federation):
		return nil, fmt.Errorf("[proof] media %q: auth oidc needs provider=projects/<number>/locations/global/workloadIdentityPools/<pool>/providers/<provider>", v)
	case (g.id.Auth == "oidc" || g.id.ServiceAccount != "") && !gsAccount.MatchString(g.id.ServiceAccount):
		return nil, fmt.Errorf("[proof] media %q: sa=<service account email> is needed with auth oidc", v)
	}
	var err error
	g.Endpoint, err = endpoint(v, q, "https://storage.googleapis.com")
	return g, err
}

// Key implements Media.
func (g *GCS) Key(repo string, story int, sha string) string {
	return objectKey(g.Prefix, repo, story, sha)
}

func (g *GCS) do(ctx context.Context, method, u string, body []byte, ctype string) ([]byte, error) {
	tok, err := g.tokens.Get("gcs", g.env.Clock(), func() (cloud.Token, error) { return g.env.Google(ctx, g.HTTP, g.id, GCSScopes) })
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, u, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.ContentLength = int64(len(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	return httpDo(g.HTTP, req, "gcs "+g.Bucket)
}

func (g *GCS) object(key string) string {
	return g.Endpoint + "/storage/v1/b/" + g.Bucket + "/o/" + url.PathEscape(key)
}

// Put implements Media (a simple media upload).
func (g *GCS) Put(ctx context.Context, key string, data []byte) error {
	u := g.Endpoint + "/upload/storage/v1/b/" + g.Bucket + "/o?" + url.Values{"uploadType": {"media"}, "name": {key}}.Encode()
	_, err := g.do(ctx, http.MethodPost, u, data, "application/octet-stream")
	return err
}

// Get implements Media.
func (g *GCS) Get(ctx context.Context, key string) ([]byte, error) {
	return g.do(ctx, http.MethodGet, g.object(key)+"?alt=media", nil, "")
}

// Delete implements Media.
func (g *GCS) Delete(ctx context.Context, key string) error {
	_, err := g.do(ctx, http.MethodDelete, g.object(key), nil, "")
	if errors.Is(err, ErrNoObject) {
		return nil
	}
	return err
}

func (g *GCS) rules(ctx context.Context) ([]json.RawMessage, error) {
	raw, err := g.do(ctx, http.MethodGet, g.Endpoint+"/storage/v1/b/"+g.Bucket+"?fields=lifecycle", nil, "")
	if err != nil {
		return nil, err
	}
	var b struct {
		Lifecycle struct {
			Rule []json.RawMessage `json:"rule"`
		} `json:"lifecycle"`
	}
	if err := json.Unmarshal(raw, &b); err != nil {
		return nil, fmt.Errorf("gcs lifecycle: %w", err)
	}
	return b.Lifecycle.Rule, nil
}

func (g *GCS) rule(repo string, days int) json.RawMessage {
	r, _ := json.Marshal(map[string]any{"action": map[string]string{"type": "Delete"},
		"condition": map[string]any{"age": days, "matchesPrefix": []string{repoPrefix(g.Prefix, repo)}}})
	return r
}

// ours reports whether a GCS rule is aayudha's for repo: a Delete matching
// exactly the repository's prefix (GCS rules carry no ID).
func (g *GCS) ours(r json.RawMessage, repo string) bool {
	var x struct {
		Action struct {
			Type string `json:"type"`
		} `json:"action"`
		Condition struct {
			MatchesPrefix []string `json:"matchesPrefix"`
		} `json:"condition"`
	}
	_ = json.Unmarshal(r, &x)
	return x.Action.Type == "Delete" && slices.Equal(x.Condition.MatchesPrefix, []string{repoPrefix(g.Prefix, repo)})
}

// EnsureLifecycle implements Media: one Delete rule on the bucket, merged
// with the others (a patch replaces the lifecycle).
func (g *GCS) EnsureLifecycle(ctx context.Context, repo string, days int) (bool, error) {
	cur, err := g.rules(ctx)
	if err != nil {
		return false, err
	}
	want := g.rule(repo, days)
	rules := []json.RawMessage{}
	found := false
	for _, r := range cur {
		if g.ours(r, repo) {
			found = true
			if days > 0 && sameJSON(r, want) {
				return false, nil
			}
			continue
		}
		rules = append(rules, r)
	}
	if days == 0 && !found {
		return false, nil
	}
	if days > 0 {
		rules = append(rules, want)
	}
	body, _ := json.Marshal(map[string]any{"lifecycle": map[string]any{"rule": rules}})
	_, err = g.do(ctx, http.MethodPatch, g.Endpoint+"/storage/v1/b/"+g.Bucket+"?fields=lifecycle", body, "application/json")
	return err == nil, err
}

// Lifecycle implements Media.
func (g *GCS) Lifecycle(ctx context.Context, repo string) (string, error) {
	cur, err := g.rules(ctx)
	if err != nil {
		return "", err
	}
	for _, r := range cur {
		if g.ours(r, repo) {
			return string(r), nil
		}
	}
	return "", nil
}
