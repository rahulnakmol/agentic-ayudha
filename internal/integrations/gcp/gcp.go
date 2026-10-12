// Package gcp is the Google Cloud adapter (WP7.2b): baselines and event
// pulls from the Google Analytics 4 (Firebase) BigQuery export through the
// BigQuery jobs.query API, with an OAuth token from a service account key
// (a signed JWT), GitHub Actions workload identity federation (STS token
// exchange, then service account impersonation), or `gcloud auth
// print-access-token`. Minted tokens carry only read-only OAuth scopes, and
// before any query the adapter asks IAM (testIamPermissions) whether the
// identity holds a write permission, and refuses it if so.
package gcp

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/cloud"
)

// Endpoints.
const (
	TokenURL       = "https://oauth2.googleapis.com/token"
	STSURL         = "https://sts.googleapis.com/v1/token"
	IAMCredentials = "iamcredentials.googleapis.com"
	BigQuery       = "bigquery.googleapis.com"
	ResourceMgr    = "cloudresourcemanager.googleapis.com"
)

// Scopes are the OAuth scopes a minted token carries: read-only, so the
// token cannot write even where the identity could.
var Scopes = []string{"https://www.googleapis.com/auth/bigquery.readonly", "https://www.googleapis.com/auth/cloud-platform.read-only"}

// Metrics this adapter answers: events counts GA4 export events, users the
// distinct users behind them (user_id, else user_pseudo_id).
var Metrics = []string{"events", "users"}

// Need lists the fields [integrations.gcp] must set.
var Need = []integrations.Field{
	{Key: "project", Want: "<project>.<dataset> of the GA4 BigQuery export, e.g. acme-prod.analytics_123456789"},
	{Key: "federation", Want: "the workload identity provider, projects/<number>/locations/global/workloadIdentityPools/<pool>/providers/<provider>", Auth: "oidc"},
	{Key: "identity", Want: "the read-only service account email to impersonate", Auth: "oidc"},
}

// ReadPermissions are what the adapter uses; WritePermissions are what a
// read-only identity must not hold on the project (doctor and pre-query).
var (
	ReadPermissions  = []string{"bigquery.jobs.create", "bigquery.tables.getData", "bigquery.tables.list"}
	WritePermissions = []string{
		"bigquery.datasets.create", "bigquery.datasets.update", "bigquery.datasets.delete",
		"bigquery.tables.create", "bigquery.tables.update", "bigquery.tables.updateData", "bigquery.tables.delete",
		"bigquery.routines.create", "resourcemanager.projects.setIamPolicy", "iam.serviceAccountKeys.create",
		"storage.objects.create", "storage.objects.delete",
	}
	tableWrites = []string{"bigquery.tables.update", "bigquery.tables.updateData", "bigquery.tables.delete"}
)

var (
	projectID  = regexp.MustCompile(`^[a-z][a-z0-9-]{4,28}[a-z0-9]$`)
	datasetID  = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
	saEmail    = regexp.MustCompile(`^[a-z0-9-]+@[a-z0-9-]+\.iam\.gserviceaccount\.com$`)
	federation = regexp.MustCompile(`^projects/[0-9]+/locations/global/workloadIdentityPools/[a-z0-9-]+/providers/[a-z0-9-]+$`)
	location   = regexp.MustCompile(`^[A-Za-z0-9-]{2,30}$`)
)

// key is a service account JSON key.
type key struct {
	Type         string `json:"type"`
	ClientEmail  string `json:"client_email"`
	PrivateKey   string `json:"private_key"`
	PrivateKeyID string `json:"private_key_id"`
	TokenURI     string `json:"token_uri"`
	rsa          *rsa.PrivateKey
}

// Client reads one GA4 BigQuery export dataset.
type Client struct {
	hc       *http.Client
	conn     integrations.Conn
	project  string
	dataset  string
	location string
	key      *key
	env      cloud.Env
	tokens   cloud.Cache
	// Sleep waits between polls of an unfinished query; tests make it instant.
	Sleep func(time.Duration)
}

// New returns a client for a resolved [integrations.gcp].
func New(c integrations.Conn, rt http.RoundTripper, env cloud.Env) (*Client, error) {
	p, d, _ := strings.Cut(c.Project, ".")
	if !projectID.MatchString(p) || !datasetID.MatchString(d) || len(d) > 1024 {
		return nil, fmt.Errorf("%w: [integrations.%s] project = %q: want <project>.<dataset> of the GA4 BigQuery export, e.g. acme-prod.analytics_123456789", integrations.ErrNotConfigured, c.Name, c.Project)
	}
	if c.Region != "" && !location.MatchString(c.Region) {
		return nil, fmt.Errorf("%w: [integrations.%s] region = %q: want a BigQuery location such as europe-west2", integrations.ErrNotConfigured, c.Name, c.Region)
	}
	if c.Auth == "oidc" && !federation.MatchString(c.Federation) {
		return nil, fmt.Errorf("%w: [integrations.%s] federation = %q: want projects/<number>/locations/global/workloadIdentityPools/<pool>/providers/<provider>", integrations.ErrNotConfigured, c.Name, c.Federation)
	}
	if (c.Auth == "oidc" || c.Identity != "") && !saEmail.MatchString(c.Identity) {
		return nil, fmt.Errorf("%w: [integrations.%s] identity = %q: want the service account email (<name>@<project>.iam.gserviceaccount.com)", integrations.ErrNotConfigured, c.Name, c.Identity)
	}
	cl := &Client{conn: c, project: p, dataset: d, location: c.Region, env: env, Sleep: time.Sleep}
	if c.StoredKey() {
		k, err := parseKey(c.Key)
		if err != nil {
			return nil, fmt.Errorf("%w: [integrations.%s] secret %q: %w", integrations.ErrNotConfigured, c.Name, c.Secret, err)
		}
		cl.key = k
	}
	path := regexp.MustCompile
	cl.hc = integrations.Client(rt,
		integrations.Allow{Method: http.MethodPost, Host: "oauth2.googleapis.com", Path: path(`^/token$`)},
		integrations.Allow{Method: http.MethodPost, Host: "sts.googleapis.com", Path: path(`^/v1/token$`)},
		integrations.Allow{Method: http.MethodPost, Host: IAMCredentials, Path: path(`^/v1/projects/-/serviceAccounts/[^/]+:generateAccessToken$`)},
		integrations.Allow{Method: http.MethodPost, Host: ResourceMgr, Path: path(`^/v1/projects/[^/]+:testIamPermissions$`)},
		integrations.Allow{Method: http.MethodPost, Host: BigQuery, Path: path(`^/bigquery/v2/projects/[^/]+/queries$`)},
		integrations.Allow{Method: http.MethodPost, Host: BigQuery, Path: path(`^/bigquery/v2/projects/[^/]+/datasets/[^/]+/tables/[^/]+:testIamPermissions$`)})
	return cl, nil
}

func parseKey(s string) (*key, error) {
	var k key
	if err := json.Unmarshal([]byte(s), &k); err != nil || k.Type != "service_account" || k.ClientEmail == "" {
		return nil, fmt.Errorf("want a service account JSON key (type service_account)")
	}
	if k.TokenURI == "" {
		k.TokenURI = TokenURL
	}
	if k.TokenURI != TokenURL {
		return nil, fmt.Errorf("token_uri %q: only %s is trusted", k.TokenURI, TokenURL)
	}
	block, _ := pem.Decode([]byte(k.PrivateKey))
	if block == nil {
		return nil, fmt.Errorf("private_key is not PEM")
	}
	pk, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		pk1, err1 := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err1 != nil {
			return nil, fmt.Errorf("private_key: %w", err)
		}
		pk = pk1
	}
	r, ok := pk.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private_key is not RSA")
	}
	k.rsa = r
	return &k, nil
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// assertion is the RS256 JWT a service account key signs to ask Google's
// token endpoint for an access token (the JWT bearer grant).
func (k *key) assertion(now time.Time) (string, error) {
	h, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": k.PrivateKeyID})
	c, _ := json.Marshal(map[string]any{"iss": k.ClientEmail, "scope": strings.Join(Scopes, " "), "aud": k.TokenURI,
		"iat": now.Unix(), "exp": now.Add(time.Hour).Unix()})
	signing := b64(h) + "." + b64(c)
	sum := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, k.rsa, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return signing + "." + b64(sig), nil
}

// token returns an access token for the identity.
func (c *Client) token(ctx context.Context) (string, error) {
	return c.tokens.Get("gcp", c.env.Clock(), func() (cloud.Token, error) {
		now := c.env.Clock()
		switch c.conn.Auth {
		case "cli":
			args := []string{"auth", "print-access-token"}
			if c.conn.Identity != "" {
				args = append(args, "--impersonate-service-account="+c.conn.Identity)
			}
			out, err := c.env.CLI(ctx, "gcloud", args...)
			if err != nil {
				return cloud.Token{}, err
			}
			t := strings.TrimSpace(string(out))
			if t == "" || strings.ContainsAny(t, " \n") {
				return cloud.Token{}, fmt.Errorf("%w: `gcloud auth print-access-token` printed no token", integrations.ErrProvider)
			}
			return cloud.Token{Value: t, Expires: now.Add(5 * time.Minute)}, nil
		case "oidc":
			jwt, err := c.env.GitHubOIDC(ctx, c.hc, "https://iam.googleapis.com/"+c.conn.Federation)
			if err != nil {
				return cloud.Token{}, err
			}
			var fed cloud.OAuth
			if err := cloud.PostJSON(ctx, c.hc, STSURL, "", map[string]string{
				"audience":           "//iam.googleapis.com/" + c.conn.Federation,
				"grantType":          "urn:ietf:params:oauth:grant-type:token-exchange",
				"requestedTokenType": "urn:ietf:params:oauth:token-type:access_token",
				"scope":              "https://www.googleapis.com/auth/cloud-platform",
				"subjectTokenType":   "urn:ietf:params:oauth:token-type:jwt",
				"subjectToken":       jwt,
			}, &fed); err != nil {
				return cloud.Token{}, err
			}
			if fed.AccessToken == "" {
				return cloud.Token{}, fmt.Errorf("%w: Google STS returned no access token", integrations.ErrProvider)
			}
			var sa struct {
				AccessToken string `json:"accessToken"`
				ExpireTime  string `json:"expireTime"`
			}
			u := "https://" + IAMCredentials + "/v1/projects/-/serviceAccounts/" + c.conn.Identity + ":generateAccessToken"
			if err := cloud.PostJSON(ctx, c.hc, u, fed.AccessToken, map[string]any{"scope": Scopes, "lifetime": "3600s"}, &sa); err != nil {
				return cloud.Token{}, err
			}
			exp, err := time.Parse(time.RFC3339, sa.ExpireTime)
			if sa.AccessToken == "" || err != nil {
				return cloud.Token{}, fmt.Errorf("%w: generateAccessToken returned no token", integrations.ErrProvider)
			}
			return cloud.Token{Value: sa.AccessToken, Expires: exp}, nil
		default:
			a, err := c.key.assertion(now)
			if err != nil {
				return cloud.Token{}, fmt.Errorf("%w: signing the service account assertion: %w", integrations.ErrNotConfigured, err)
			}
			var res cloud.OAuth
			if err := cloud.PostForm(ctx, c.hc, c.key.TokenURI, url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"}, "assertion": {a}}, &res); err != nil {
				return cloud.Token{}, err
			}
			return res.Token(now, "Google OAuth")
		}
	})
}

func (c *Client) account() string {
	switch {
	case c.conn.Auth == "cli" && c.conn.Identity != "":
		return c.conn.Identity + " (impersonated through gcloud)"
	case c.conn.Auth == "cli":
		return "the gcloud login"
	case c.conn.Auth == "oidc":
		return c.conn.Identity + " (GitHub Actions workload identity federation)"
	}
	return c.key.ClientEmail
}

func (c *Client) post(ctx context.Context, u string, body, out any) error {
	tok, err := c.token(ctx)
	if err != nil {
		return err
	}
	return cloud.PostJSON(ctx, c.hc, u, tok, body, out)
}

func (c *Client) get(ctx context.Context, u string, out any) error {
	tok, err := c.token(ctx)
	if err != nil {
		return err
	}
	req, err := integrations.NewRequest(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	return integrations.DoJSON(c.hc, req, out)
}

// Scope asks IAM which of the read and write permissions the identity holds
// on the project, then which table writes it holds on a table of the export
// dataset (dataset-level grants do not show on the project).
func (c *Client) Scope(ctx context.Context) (integrations.Scope, error) {
	path := "/v1/projects/" + c.project + ":testIamPermissions"
	var res struct {
		Permissions []string `json:"permissions"`
	}
	if err := c.post(ctx, "https://"+ResourceMgr+path, map[string]any{"permissions": append(append([]string{}, ReadPermissions...), WritePermissions...)}, &res); err != nil {
		return integrations.Scope{}, err
	}
	s := integrations.Scope{Verified: true, Account: c.account(), Endpoint: "POST " + path + " and the dataset's first table"}
	for _, p := range res.Permissions {
		s.Scopes = append(s.Scopes, p)
		for _, w := range WritePermissions {
			if p == w {
				s.Write = append(s.Write, p)
			}
		}
	}
	var tables struct {
		Tables []struct {
			Ref struct {
				TableID string `json:"tableId"`
			} `json:"tableReference"`
		} `json:"tables"`
	}
	ds := "https://" + BigQuery + "/bigquery/v2/projects/" + c.project + "/datasets/" + c.dataset
	if err := c.get(ctx, ds+"/tables?maxResults=1", &tables); err != nil {
		return integrations.Scope{}, err
	}
	if len(tables.Tables) == 0 {
		return integrations.Scope{}, fmt.Errorf("%w: dataset %s.%s has no table; is the GA4 export linked?", integrations.ErrProvider, c.project, c.dataset)
	}
	var tres struct {
		Permissions []string `json:"permissions"`
	}
	if err := c.post(ctx, ds+"/tables/"+url.PathEscape(tables.Tables[0].Ref.TableID)+":testIamPermissions", map[string]any{"permissions": tableWrites}, &tres); err != nil {
		return integrations.Scope{}, err
	}
	for _, p := range tres.Permissions {
		s.Write = append(s.Write, p+" (dataset "+c.dataset+")")
	}
	return s, nil
}

// window is the SQL that keeps the GA4 export rows of the window: the daily
// tables are named in the property's time zone, so the suffix range is a
// day wider on each side and event_timestamp (µs, UTC) decides.
func (c *Client) window(w integrations.Window) string {
	return fmt.Sprintf("FROM `%s.%s.events_*`\nWHERE _TABLE_SUFFIX BETWEEN '%s' AND '%s'\nAND event_timestamp >= %d AND event_timestamp < %d",
		c.project, c.dataset, w.From.UTC().AddDate(0, 0, -1).Format("20060102"), w.To.UTC().Format("20060102"), w.From.UnixMicro(), w.To.UnixMicro())
}

// SQL returns the GoogleSQL a baseline runs. The filter is a predicate on
// the export's columns; it ends its own line.
func (c *Client) SQL(q integrations.Query) (string, error) {
	var sel string
	switch q.Metric {
	case "events":
		sel = "COUNT(*)"
	case "users":
		sel = "COUNT(DISTINCT COALESCE(user_id, user_pseudo_id))"
	default:
		return "", fmt.Errorf("%w: gcp metric %q: want one of %s", integrations.ErrQuery, q.Metric, strings.Join(Metrics, ", "))
	}
	if err := q.Window.Check(); err != nil {
		return "", err
	}
	sql := "SELECT " + sel + " AS value\n" + c.window(q.Window)
	if f := strings.TrimSpace(q.Filter); f != "" {
		if err := cloud.CheckFilter("gcp", f, "|>"); err != nil {
			return "", err
		}
		sql += "\nAND (" + f + "\n)"
	}
	return sql, nil
}

// ExportSQL is the query an event pull runs: the columns the GA4 export
// reader parses.
func (c *Client) ExportSQL(w integrations.Window) string {
	return "SELECT event_timestamp, event_name, user_id, user_pseudo_id\n" + c.window(w)
}

type result struct {
	JobComplete bool `json:"jobComplete"`
	JobRef      struct {
		JobID    string `json:"jobId"`
		Location string `json:"location"`
	} `json:"jobReference"`
	Schema struct {
		Fields []struct {
			Name string `json:"name"`
		} `json:"fields"`
	} `json:"schema"`
	Rows []struct {
		F []struct {
			V any `json:"v"`
		} `json:"f"`
	} `json:"rows"`
	PageToken string `json:"pageToken"`
}

// query checks the identity, runs sql with jobs.query, and calls row for
// every result row, polling an unfinished job and paging with
// jobs.getQueryResults. It returns the exact request.
func (c *Client) query(ctx context.Context, sql string, row func(map[string]any) error) (string, error) {
	scope, err := c.Scope(ctx)
	if err != nil {
		return "", err
	}
	if err := scope.Check("gcp"); err != nil {
		return "", err
	}
	body := map[string]any{"query": sql, "useLegacySql": false, "timeoutMs": 60000, "maxResults": 10000}
	if c.location != "" {
		body["location"] = c.location
	}
	u := "https://" + BigQuery + "/bigquery/v2/projects/" + c.project + "/queries"
	var res result
	if err := c.post(ctx, u, body, &res); err != nil {
		return "", err
	}
	schema := res.Schema
	for page := 0; ; page++ {
		if res.JobComplete {
			if len(res.Schema.Fields) > 0 {
				schema = res.Schema
			}
			for _, r := range res.Rows {
				m := map[string]any{}
				for i, f := range schema.Fields {
					if i < len(r.F) {
						m[f.Name] = r.F[i].V
					}
				}
				if err := row(m); err != nil {
					return "", err
				}
			}
			if res.PageToken == "" {
				break
			}
		} else if page > 0 {
			c.Sleep(time.Second)
		}
		if page >= 100000 || res.JobRef.JobID == "" {
			return "", fmt.Errorf("%w: bigquery job did not finish or page (job %q)", integrations.ErrProvider, res.JobRef.JobID)
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}
		v := url.Values{"maxResults": {"10000"}, "timeoutMs": {"60000"}}
		if res.JobRef.Location != "" {
			v.Set("location", res.JobRef.Location)
		}
		if res.JobComplete {
			v.Set("pageToken", res.PageToken)
		}
		ref := res.JobRef
		res = result{}
		if err := c.get(ctx, u+"/"+url.PathEscape(ref.JobID)+"?"+v.Encode(), &res); err != nil {
			return "", err
		}
		if res.JobRef.JobID == "" {
			res.JobRef = ref
		}
	}
	b, _ := cloud.Marshal(body)
	return "POST " + u + " " + string(b), nil
}

// Baseline checks the identity, then runs the metric's query.
func (c *Client) Baseline(ctx context.Context, q integrations.Query) (integrations.Baseline, error) {
	sql, err := c.SQL(q)
	if err != nil {
		return integrations.Baseline{}, err
	}
	var vals []any
	source, err := c.query(ctx, sql, func(m map[string]any) error { vals = append(vals, m["value"]); return nil })
	if err != nil {
		return integrations.Baseline{}, err
	}
	if len(vals) != 1 {
		return integrations.Baseline{}, fmt.Errorf("%w: bigquery returned %d rows; want one value", integrations.ErrProvider, len(vals))
	}
	s, _ := vals[0].(string) // INT64 arrives as a string
	var v float64
	if _, err := fmt.Sscan(s, &v); err != nil {
		return integrations.Baseline{}, fmt.Errorf("%w: bigquery value %v; want a number", integrations.ErrProvider, vals[0])
	}
	who := "GA4 export events"
	if q.Metric == "users" {
		who = "distinct users (user_id, else user_pseudo_id) with GA4 export events"
	}
	if f := strings.TrimSpace(q.Filter); f != "" {
		who += " where " + f
	}
	return integrations.Baseline{
		Metric:      q.Metric,
		Population:  fmt.Sprintf("BigQuery %s.%s: %s", c.project, c.dataset, who),
		Window:      integrations.Window{From: q.Window.From.UTC(), To: q.Window.To.UTC()},
		Value:       v,
		SourceQuery: source,
		FetchedAt:   c.env.Clock().UTC(),
	}, nil
}

// Export pulls the window's GA4 events as JSON lines in the BigQuery export
// format (kind "gcp").
func (c *Client) Export(ctx context.Context, w integrations.Window, out io.Writer) (string, string, error) {
	if err := w.Check(); err != nil {
		return "", "", err
	}
	enc := json.NewEncoder(out)
	source, err := c.query(ctx, c.ExportSQL(w), func(m map[string]any) error { return enc.Encode(m) })
	if err != nil {
		return "", "", err
	}
	return "gcp", source, nil
}
