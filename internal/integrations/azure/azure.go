// Package azure is the Azure Monitor adapter (WP7.2b): baselines and event
// pulls from Application Insights through the Log Analytics query API (KQL,
// resource-centric), with a Microsoft Entra token minted by client
// credentials, GitHub Actions workload identity federation, or `az account
// get-access-token`. Before any query it asks Azure Resource Manager which
// actions the identity has on the resource and refuses one that can write.
package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/cloud"
)

// Endpoints (Azure public cloud).
const (
	LoginHost = cloud.EntraHost
	QueryHost = "api.loganalytics.azure.com"
	ARMHost   = "management.azure.com"

	queryScope = "https://api.loganalytics.io/.default"
	armScope   = "https://management.azure.com/.default"
	// PermissionsAPI is the Microsoft.Authorization API version doctor reads.
	PermissionsAPI = "2022-04-01"
)

// Metrics this adapter answers: events counts AppEvents rows, users the
// distinct users behind them (UserAuthenticatedId, else UserId), as the
// export reader joins them.
var Metrics = []string{"events", "users"}

// Need lists the fields [integrations.azure] must set.
var Need = []integrations.Field{
	{Key: "project", Want: "the Azure resource ID of the Log Analytics workspace or Application Insights component"},
	{Key: "org", Want: "the Microsoft Entra tenant ID", Auth: "key"},
	{Key: "org", Want: "the Microsoft Entra tenant ID", Auth: "oidc"},
	{Key: "identity", Want: "the Entra application (client) ID trusted for GitHub Actions", Auth: "oidc"},
}

var (
	resourceID = regexp.MustCompile(`(?i)^/subscriptions/[0-9a-f-]{36}/resourceGroups/[A-Za-z0-9._()-]{1,90}/providers/Microsoft\.(OperationalInsights/workspaces|Insights/components)/[A-Za-z0-9_-]{1,260}$`)
	tenantID   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]{0,252}$`)
	clientID   = regexp.MustCompile(`^[0-9a-fA-F-]{36}$`)
)

// Client reads one Application Insights component or Log Analytics workspace.
type Client struct {
	hc       *http.Client
	conn     integrations.Conn
	resource string
	env      cloud.Env
	tokens   cloud.Cache
}

// New returns a client for a resolved [integrations.azure]. Only the Entra
// token endpoint and the Log Analytics query endpoint may receive a POST.
func New(c integrations.Conn, rt http.RoundTripper, env cloud.Env) (*Client, error) {
	if !resourceID.MatchString(c.Project) {
		return nil, fmt.Errorf("%w: [integrations.%s] project = %q: want the resource ID /subscriptions/<id>/resourceGroups/<group>/providers/Microsoft.OperationalInsights/workspaces/<name> (or Microsoft.Insights/components/<name>)", integrations.ErrNotConfigured, c.Name, c.Project)
	}
	if c.StoredKey() || c.Auth == "oidc" {
		if !tenantID.MatchString(c.Org) {
			return nil, fmt.Errorf("%w: [integrations.%s] org = %q: want the Microsoft Entra tenant ID", integrations.ErrNotConfigured, c.Name, c.Org)
		}
	}
	if c.Auth == "oidc" && !clientID.MatchString(c.Identity) {
		return nil, fmt.Errorf("%w: [integrations.%s] identity = %q: want the Entra application (client) ID", integrations.ErrNotConfigured, c.Name, c.Identity)
	}
	if c.StoredKey() {
		id, _, ok := strings.Cut(c.Key, ":")
		if !ok || !clientID.MatchString(id) {
			return nil, fmt.Errorf("%w: [integrations.%s] secret %q: want <client-id>:<client-secret> of an app registration", integrations.ErrNotConfigured, c.Name, c.Secret)
		}
	}
	hc := integrations.Client(rt,
		integrations.Allow{Method: http.MethodPost, Host: LoginHost, Path: regexp.MustCompile(`^/[^/]+/oauth2/v2\.0/token$`)},
		integrations.Allow{Method: http.MethodPost, Host: QueryHost, Path: regexp.MustCompile(`^/v1/subscriptions/[^?]+/query$`)})
	return &Client{hc: hc, conn: c, resource: c.Project, env: env}, nil
}

// token returns an Entra access token for scope.
func (c *Client) token(ctx context.Context, scope string) (string, error) {
	return c.tokens.Get(scope, c.env.Clock(), func() (cloud.Token, error) {
		id := cloud.Entra{Auth: c.conn.Auth, Tenant: c.conn.Org, Client: c.conn.Identity}
		if c.conn.StoredKey() {
			id.Client, id.Secret, _ = strings.Cut(c.conn.Key, ":")
		}
		return c.env.Entra(ctx, c.hc, id, scope)
	})
}

// account names the identity for doctor.
func (c *Client) account() string {
	switch c.conn.Auth {
	case "cli":
		return "the az CLI login"
	case "oidc":
		return "application " + c.conn.Identity + " (GitHub Actions OIDC)"
	}
	id, _, _ := strings.Cut(c.conn.Key, ":")
	return "application " + id
}

// readOnly reports whether an Azure RBAC action only reads. Query actions
// that POST a query are reads; Microsoft.Support/* (support requests) is
// carried by every built-in reader role and changes no resource.
func readOnly(action string) bool {
	a := strings.ToLower(action)
	switch {
	case strings.HasSuffix(a, "/read"),
		a == "microsoft.operationalinsights/workspaces/analytics/query/action",
		a == "microsoft.operationalinsights/workspaces/search/action",
		strings.HasPrefix(a, "microsoft.support/"):
		return true
	}
	return false
}

// Scope asks Azure Resource Manager for the identity's effective actions on
// the resource (Microsoft.Authorization/permissions). Any action or data
// action that is not a read makes the credential write-capable.
func (c *Client) Scope(ctx context.Context) (integrations.Scope, error) {
	tok, err := c.token(ctx, armScope)
	if err != nil {
		return integrations.Scope{}, err
	}
	path := c.resource + "/providers/Microsoft.Authorization/permissions"
	next := "https://" + ARMHost + path + "?api-version=" + PermissionsAPI
	s := integrations.Scope{Verified: true, Account: c.account(), Endpoint: "GET " + path}
	for pages := 0; next != "" && pages < 20; pages++ {
		req, err := integrations.NewRequest(ctx, http.MethodGet, next, nil)
		if err != nil {
			return integrations.Scope{}, err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		var res struct {
			Value []struct {
				Actions     []string `json:"actions"`
				DataActions []string `json:"dataActions"`
			} `json:"value"`
			NextLink string `json:"nextLink"`
		}
		if err := integrations.DoJSON(c.hc, req, &res); err != nil {
			return integrations.Scope{}, err
		}
		for _, p := range res.Value {
			for _, a := range append(append([]string{}, p.Actions...), p.DataActions...) {
				s.Scopes = append(s.Scopes, a)
				if !readOnly(a) {
					s.Write = append(s.Write, a)
				}
			}
		}
		next = ""
		if u, err := url.Parse(res.NextLink); err == nil && u.Scheme == "https" && u.Host == ARMHost {
			next = res.NextLink
		}
	}
	if len(s.Scopes) == 0 {
		return integrations.Scope{}, fmt.Errorf("%w: GET %s: the identity has no role on %s", integrations.ErrProvider, path, c.resource)
	}
	return s, nil
}

func span(w integrations.Window) string {
	const f = "2006-01-02T15:04:05Z"
	return fmt.Sprintf("| where TimeGenerated >= datetime(%s) and TimeGenerated < datetime(%s)", w.From.UTC().Format(f), w.To.UTC().Format(f))
}

// KQL returns the query a baseline runs. The filter is a KQL predicate on
// AppEvents; it ends its own line, so a trailing comment cannot hide the rest.
func KQL(q integrations.Query) (string, error) {
	if q.Metric != "events" && q.Metric != "users" {
		return "", fmt.Errorf("%w: azure metric %q: want one of %s", integrations.ErrQuery, q.Metric, strings.Join(Metrics, ", "))
	}
	if err := q.Window.Check(); err != nil {
		return "", err
	}
	lines := []string{"AppEvents", span(q.Window)}
	if f := strings.TrimSpace(q.Filter); f != "" {
		if err := cloud.CheckFilter("azure", f, "|"); err != nil {
			return "", err
		}
		lines = append(lines, "| where "+f)
	}
	if q.Metric == "users" {
		lines = append(lines, "| extend User = iff(isnotempty(UserAuthenticatedId), UserAuthenticatedId, UserId)", "| where isnotempty(User)", "| summarize by User")
	}
	return strings.Join(append(lines, "| count"), "\n"), nil
}

// ExportKQL is the query an event pull runs: the columns the Application
// Insights export reader parses.
func ExportKQL(w integrations.Window) string {
	return strings.Join([]string{"AppEvents", span(w), "| project TimeGenerated, Name, UserAuthenticatedId, UserId"}, "\n")
}

type table struct {
	Columns []struct {
		Name string `json:"name"`
	} `json:"columns"`
	Rows [][]any `json:"rows"`
}

// query checks the identity's actions, then runs one KQL query over the
// window and returns its first table and the exact request.
func (c *Client) query(ctx context.Context, kql string, w integrations.Window) (table, string, error) {
	scope, err := c.Scope(ctx)
	if err != nil {
		return table{}, "", err
	}
	if err := scope.Check("azure"); err != nil {
		return table{}, "", err
	}
	tok, err := c.token(ctx, queryScope)
	if err != nil {
		return table{}, "", err
	}
	const f = "2006-01-02T15:04:05Z"
	body := map[string]string{"query": kql, "timespan": w.From.UTC().Format(f) + "/" + w.To.UTC().Format(f)}
	u := "https://" + QueryHost + "/v1" + c.resource + "/query"
	var res struct {
		Tables []table `json:"tables"`
		Error  *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := cloud.PostJSON(ctx, c.hc, u, tok, body, &res); err != nil {
		return table{}, "", err
	}
	// A partial result (row or size limit) carries an error beside the
	// rows; a truncated pull would undercount, so it is refused.
	if res.Error != nil {
		return table{}, "", fmt.Errorf("%w: log analytics: %s: %s (narrow the window)", integrations.ErrProvider, res.Error.Code, res.Error.Message)
	}
	if len(res.Tables) == 0 {
		return table{}, "", fmt.Errorf("%w: log analytics returned no table", integrations.ErrProvider)
	}
	b, _ := cloud.Marshal(body)
	return res.Tables[0], "POST " + u + " " + string(b), nil
}

func (c *Client) name() string { return c.resource[strings.LastIndex(c.resource, "/")+1:] }

// Baseline checks the identity's actions, then runs the metric's KQL query.
func (c *Client) Baseline(ctx context.Context, q integrations.Query) (integrations.Baseline, error) {
	kql, err := KQL(q)
	if err != nil {
		return integrations.Baseline{}, err
	}
	t, source, err := c.query(ctx, kql, q.Window)
	if err != nil {
		return integrations.Baseline{}, err
	}
	if len(t.Rows) != 1 || len(t.Rows[0]) != 1 {
		return integrations.Baseline{}, fmt.Errorf("%w: log analytics returned %d rows; want one count", integrations.ErrProvider, len(t.Rows))
	}
	v, err := number(t.Rows[0][0])
	if err != nil {
		return integrations.Baseline{}, err
	}
	who := "AppEvents events"
	if q.Metric == "users" {
		who = "distinct users (UserAuthenticatedId, else UserId) with AppEvents events"
	}
	if f := strings.TrimSpace(q.Filter); f != "" {
		who += " where " + f
	}
	return integrations.Baseline{
		Metric:      q.Metric,
		Population:  fmt.Sprintf("Azure Monitor %s: %s", c.name(), who),
		Window:      integrations.Window{From: q.Window.From.UTC(), To: q.Window.To.UTC()},
		Value:       v,
		SourceQuery: source,
		FetchedAt:   c.env.Clock().UTC(),
	}, nil
}

// number reads a count, which Log Analytics sends as a JSON number or, in
// some responses, a string.
func number(v any) (float64, error) {
	switch x := v.(type) {
	case float64:
		return x, nil
	case string:
		var f float64
		if _, err := fmt.Sscan(x, &f); err == nil {
			return f, nil
		}
	}
	return 0, fmt.Errorf("%w: log analytics count is %v; want a number", integrations.ErrProvider, v)
}

// Export pulls the window's AppEvents as JSON lines in the Application
// Insights export format (kind "appinsights"), so measure computes cohorts
// from them exactly as from an export file.
func (c *Client) Export(ctx context.Context, w integrations.Window, out io.Writer) (string, string, error) {
	if err := w.Check(); err != nil {
		return "", "", err
	}
	t, source, err := c.query(ctx, ExportKQL(w), w)
	if err != nil {
		return "", "", err
	}
	enc := json.NewEncoder(out)
	for _, r := range t.Rows {
		m := map[string]any{}
		for i, col := range t.Columns {
			if i < len(r) {
				m[col.Name] = r[i]
			}
		}
		if err := enc.Encode(m); err != nil {
			return "", "", err
		}
	}
	return "appinsights", source, nil
}
