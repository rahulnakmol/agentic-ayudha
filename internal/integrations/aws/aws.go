// Package aws is the AWS adapter (WP7.2b): baselines from CloudWatch Logs
// Insights over a CloudWatch RUM app monitor's log group, and event pulls
// from CloudWatch RUM (GetAppMonitorData), signed with Signature Version 4
// (the WP4.2 signer). Credentials are an access key pair, a role assumed
// with a GitHub Actions OIDC token (AssumeRoleWithWebIdentity), or `aws
// configure export-credentials`. Before any query it simulates the
// principal's policies (iam:SimulatePrincipalPolicy) and refuses a
// credential that may write.
package aws

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/cloud"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof/sigv4"
)

// OIDCAudience is the audience AWS's GitHub OIDC provider is created with.
const OIDCAudience = "sts.amazonaws.com"

// RUMRetention is how far back GetAppMonitorData reads: a window starting
// earlier is refused before any call (use an export file instead).
const RUMRetention = 30 * 24 * time.Hour

// Metrics this adapter answers: events counts RUM events, users the
// distinct user_details.userId behind them.
var Metrics = []string{"events", "users"}

// Need lists the fields [integrations.aws] must set.
var Need = []integrations.Field{
	{Key: "project", Want: "the CloudWatch RUM app monitor name"},
	{Key: "region", Want: "the AWS region of the app monitor, e.g. eu-west-1"},
	{Key: "identity", Want: "the IAM role ARN trusted for GitHub Actions", Auth: "oidc"},
}

// ReadActions are what the adapter calls; WriteActions are what a read-only
// principal must not be allowed (the doctor and pre-query check).
var (
	ReadActions  = []string{"logs:StartQuery", "logs:GetQueryResults", "rum:GetAppMonitor", "rum:GetAppMonitorData"}
	WriteActions = []string{
		"logs:PutLogEvents", "logs:CreateLogGroup", "logs:DeleteLogGroup", "logs:PutRetentionPolicy", "logs:PutSubscriptionFilter", "logs:DeleteLogStream",
		"rum:PutRumEvents", "rum:CreateAppMonitor", "rum:UpdateAppMonitor", "rum:DeleteAppMonitor", "rum:PutRumMetricsDestination",
		"iam:CreateAccessKey", "iam:PutRolePolicy", "iam:AttachRolePolicy", "iam:PassRole", "iam:CreateUser",
		"s3:PutObject", "s3:DeleteObject", "sts:AssumeRole",
	}
)

var (
	monitorName = regexp.MustCompile(`^[A-Za-z0-9_-][A-Za-z0-9._-]{0,254}$`)
	regionName  = regexp.MustCompile(`^[a-z]{2}(-[a-z]+)+-[0-9]$`)
	roleARN     = regexp.MustCompile(`^arn:aws[a-z-]*:iam::[0-9]{12}:role/[A-Za-z0-9+=,.@_/-]+$`)
	keyID       = regexp.MustCompile(`^[A-Z0-9]{16,128}$`)
)

type creds struct {
	access, secret, session string
	expires                 time.Time
}

// Client reads one CloudWatch RUM app monitor.
type Client struct {
	hc      *http.Client
	conn    integrations.Conn
	region  string
	monitor string
	env     cloud.Env
	// Sleep waits between Logs Insights polls; tests make it instant.
	Sleep func(time.Duration)

	mu   sync.Mutex
	cred *creds
}

func (c *Client) host(service string) string { return service + "." + c.region + ".amazonaws.com" }

// New returns a client for a resolved [integrations.aws].
func New(c integrations.Conn, rt http.RoundTripper, env cloud.Env) (*Client, error) {
	if !monitorName.MatchString(c.Project) {
		return nil, fmt.Errorf("%w: [integrations.%s] project = %q: want the CloudWatch RUM app monitor name (letters, digits, ., _ and -)", integrations.ErrNotConfigured, c.Name, c.Project)
	}
	if !regionName.MatchString(c.Region) {
		return nil, fmt.Errorf("%w: [integrations.%s] region = %q: want an AWS region such as eu-west-1", integrations.ErrNotConfigured, c.Name, c.Region)
	}
	if c.Auth == "oidc" && !roleARN.MatchString(c.Identity) {
		return nil, fmt.Errorf("%w: [integrations.%s] identity = %q: want the IAM role ARN, arn:aws:iam::<account>:role/<name>", integrations.ErrNotConfigured, c.Name, c.Identity)
	}
	if c.StoredKey() {
		ak, _, ok := strings.Cut(c.Key, ":")
		if !ok || !keyID.MatchString(ak) {
			return nil, fmt.Errorf("%w: [integrations.%s] secret %q: want ACCESS_KEY_ID:SECRET_ACCESS_KEY", integrations.ErrNotConfigured, c.Name, c.Secret)
		}
	}
	cl := &Client{conn: c, region: c.Region, monitor: c.Project, env: env, Sleep: time.Sleep}
	root := regexp.MustCompile(`^/$`)
	cl.hc = integrations.Client(rt,
		integrations.Allow{Method: http.MethodPost, Host: cl.host("sts"), Path: root},
		integrations.Allow{Method: http.MethodPost, Host: cl.host("logs"), Path: root, Header: "X-Amz-Target", Value: "Logs_20140328.StartQuery"},
		integrations.Allow{Method: http.MethodPost, Host: cl.host("logs"), Path: root, Header: "X-Amz-Target", Value: "Logs_20140328.GetQueryResults"},
		integrations.Allow{Method: http.MethodPost, Host: cl.host("rum"), Path: regexp.MustCompile(`^/appmonitor/[^/]+/data$`)})
	return cl, nil
}

// credentials returns the access key pair to sign with.
func (c *Client) credentials(ctx context.Context) (creds, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.env.Clock()
	if c.cred != nil && (c.cred.expires.IsZero() || now.Add(time.Minute).Before(c.cred.expires)) {
		return *c.cred, nil
	}
	var cr creds
	switch c.conn.Auth {
	case "cli":
		out, err := c.env.CLI(ctx, "aws", "configure", "export-credentials", "--format", "process")
		if err != nil {
			return creds{}, err
		}
		var res struct {
			AccessKeyID     string `json:"AccessKeyId"`
			SecretAccessKey string `json:"SecretAccessKey"`
			SessionToken    string `json:"SessionToken"`
			Expiration      string `json:"Expiration"`
		}
		if json.Unmarshal(out, &res) != nil || res.AccessKeyID == "" || res.SecretAccessKey == "" {
			return creds{}, fmt.Errorf("%w: `aws configure export-credentials` printed no credentials", integrations.ErrProvider)
		}
		exp, _ := time.Parse(time.RFC3339, res.Expiration)
		cr = creds{res.AccessKeyID, res.SecretAccessKey, res.SessionToken, exp}
	case "oidc":
		jwt, err := c.env.GitHubOIDC(ctx, c.hc, OIDCAudience)
		if err != nil {
			return creds{}, err
		}
		form := url.Values{"Action": {"AssumeRoleWithWebIdentity"}, "Version": {"2011-06-15"}, "RoleArn": {c.conn.Identity},
			"RoleSessionName": {"aayudha"}, "DurationSeconds": {"3600"}, "WebIdentityToken": {jwt}}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+c.host("sts")+"/", strings.NewReader(form.Encode()))
		if err != nil {
			return creds{}, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		var res struct {
			Credentials struct {
				AccessKeyID     string `xml:"AccessKeyId"`
				SecretAccessKey string `xml:"SecretAccessKey"`
				SessionToken    string `xml:"SessionToken"`
				Expiration      string `xml:"Expiration"`
			} `xml:"AssumeRoleWithWebIdentityResult>Credentials"`
		}
		if err := c.doXML(req, &res); err != nil {
			return creds{}, err
		}
		if res.Credentials.AccessKeyID == "" {
			return creds{}, fmt.Errorf("%w: AssumeRoleWithWebIdentity returned no credentials", integrations.ErrProvider)
		}
		exp, _ := time.Parse(time.RFC3339, res.Credentials.Expiration)
		cr = creds{res.Credentials.AccessKeyID, res.Credentials.SecretAccessKey, res.Credentials.SessionToken, exp}
	default:
		ak, sk, _ := strings.Cut(c.conn.Key, ":")
		cr = creds{access: ak, secret: sk}
	}
	c.cred = &cr
	return cr, nil
}

// signed sends a SigV4-signed request and returns the response body.
func (c *Client) signed(ctx context.Context, method, rawURL, service, region string, body []byte, hdr map[string]string) ([]byte, error) {
	cr, err := c.credentials(ctx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", integrations.ErrQuery, err)
	}
	req.ContentLength = int64(len(body))
	req.Header.Set("User-Agent", "aayudha")
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	if cr.session != "" {
		req.Header.Set("X-Amz-Security-Token", cr.session)
	}
	sigv4.Sign(req, body, cr.access, cr.secret, region, service, c.env.Clock())
	return c.send(req)
}

// apiError is an AWS error answer.
type apiError struct {
	status int
	code   string
	body   string
}

func (e *apiError) Error() string { return e.body }

func (c *Client) send(req *http.Request) ([]byte, error) {
	resp, err := c.hc.Do(req)
	if err != nil {
		if errors.Is(err, actions.ErrCapability) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %s %s: %w", integrations.ErrProvider, req.Method, req.URL.Redacted(), err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: %s %s: %w", integrations.ErrProvider, req.Method, req.URL.Redacted(), err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		s := strings.Join(strings.Fields(string(b)), " ")
		if len(s) > 300 {
			s = s[:300] + "…"
		}
		code := resp.Header.Get("X-Amzn-Errortype")
		if m := regexp.MustCompile(`<Code>([^<]+)</Code>`).FindStringSubmatch(s); m != nil {
			code = m[1]
		}
		return nil, fmt.Errorf("%w: %s %s%s: %s: %w", integrations.ErrProvider, req.Method, req.URL.Host, req.URL.Path, resp.Status,
			&apiError{resp.StatusCode, code, actions.RedactString(s)})
	}
	return b, nil
}

func (c *Client) doXML(req *http.Request, out any) error {
	b, err := c.send(req)
	if err != nil {
		return err
	}
	if err := xml.Unmarshal(b, out); err != nil {
		return fmt.Errorf("%w: %s %s: undecodable response: %w", integrations.ErrProvider, req.Method, req.URL.Host, err)
	}
	return nil
}

// signedXML sends a signed GET to an AWS query API (STS, IAM).
func (c *Client) signedXML(ctx context.Context, host, service, region string, q url.Values, out any) error {
	b, err := c.signed(ctx, http.MethodGet, "https://"+host+"/?"+sigv4.CanonicalQuery(q), service, region, nil, nil)
	if err != nil {
		return err
	}
	if err := xml.Unmarshal(b, out); err != nil {
		return fmt.Errorf("%w: %s %s: undecodable response: %w", integrations.ErrProvider, service, q.Get("Action"), err)
	}
	return nil
}

// signedJSON posts a JSON body to an AWS JSON or REST-JSON API.
func (c *Client) signedJSON(ctx context.Context, host, path, service, target string, in, out any) error {
	body, err := cloud.Marshal(in)
	if err != nil {
		return err
	}
	hdr := map[string]string{"Content-Type": "application/json"}
	if target != "" {
		hdr = map[string]string{"Content-Type": "application/x-amz-json-1.1", "X-Amz-Target": target}
	}
	b, err := c.signed(ctx, http.MethodPost, "https://"+host+path, service, c.region, body, hdr)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("%w: %s %s: undecodable response: %w", integrations.ErrProvider, service, path, err)
	}
	return nil
}

// principal turns a caller ARN into the IAM principal its policies attach
// to: an assumed role's session ARN becomes the role ARN (the configured
// role when it is known, since only it carries the role's path).
func (c *Client) principal(arn string) string {
	if c.conn.Auth == "oidc" {
		return c.conn.Identity
	}
	if rest, ok := strings.CutPrefix(arn, "arn:aws:sts::"); ok {
		acct, role, ok := strings.Cut(rest, ":assumed-role/")
		if ok {
			role, _, _ = strings.Cut(role, "/")
			return "arn:aws:iam::" + acct + ":role/" + role
		}
	}
	return arn
}

// Scope finds the caller (sts:GetCallerIdentity) and simulates its
// policies for the actions the adapter needs and the writes it must not
// have (iam:SimulatePrincipalPolicy, which the least-privilege policy
// grants on the principal itself). When the simulation is denied, the
// configured scope is trusted and doctor says so.
func (c *Client) Scope(ctx context.Context) (integrations.Scope, error) {
	var who struct {
		Arn     string `xml:"GetCallerIdentityResult>Arn"`
		Account string `xml:"GetCallerIdentityResult>Account"`
	}
	if err := c.signedXML(ctx, c.host("sts"), "sts", c.region, url.Values{"Action": {"GetCallerIdentity"}, "Version": {"2011-06-15"}}, &who); err != nil {
		return integrations.Scope{}, err
	}
	s := integrations.Scope{Account: who.Arn}
	if strings.HasSuffix(who.Arn, ":root") {
		s.Verified, s.Write, s.Endpoint = true, []string{"account root credentials"}, "sts:GetCallerIdentity"
		return s, nil
	}
	principal := c.principal(who.Arn)
	q := url.Values{"Action": {"SimulatePrincipalPolicy"}, "Version": {"2010-05-08"}, "PolicySourceArn": {principal}}
	for i, a := range append(append([]string{}, ReadActions...), WriteActions...) {
		q.Set("ActionNames.member."+strconv.Itoa(i+1), a)
	}
	q.Set("ResourceArns.member.1", "*")
	q.Set("ResourceArns.member.2", "arn:aws:rum:"+c.region+":"+who.Account+":appmonitor/"+c.monitor)
	var sim struct {
		Results []struct {
			Action   string `xml:"EvalActionName"`
			Decision string `xml:"EvalDecision"`
		} `xml:"SimulatePrincipalPolicyResult>EvaluationResults>member"`
	}
	err := c.signedXML(ctx, "iam.amazonaws.com", "iam", "us-east-1", q, &sim)
	var ae *apiError
	if errors.As(err, &ae) && ae.code == "AccessDenied" {
		s.Endpoint = "iam:SimulatePrincipalPolicy (denied: grant it on " + principal + " to verify scope)"
		return s, nil
	}
	if err != nil {
		return integrations.Scope{}, err
	}
	s.Verified, s.Endpoint = true, "iam:SimulatePrincipalPolicy on "+principal
	allowed := map[string]bool{}
	for _, r := range sim.Results {
		if r.Decision == "allowed" {
			allowed[r.Action] = true
		}
	}
	for _, a := range ReadActions {
		if allowed[a] {
			s.Scopes = append(s.Scopes, a)
		}
	}
	for _, a := range WriteActions {
		if allowed[a] {
			s.Write = append(s.Write, a)
		}
	}
	return s, nil
}

func (c *Client) checked(ctx context.Context) error {
	s, err := c.Scope(ctx)
	if err != nil {
		return err
	}
	return s.Check("aws")
}

// logGroup reads the app monitor's CloudWatch Logs group (rum:GetAppMonitor).
func (c *Client) logGroup(ctx context.Context) (string, error) {
	b, err := c.signed(ctx, http.MethodGet, "https://"+c.host("rum")+"/appmonitor/"+c.monitor, "rum", c.region, nil, nil)
	if err != nil {
		return "", err
	}
	var res struct {
		AppMonitor struct {
			DataStorage struct {
				CwLog struct {
					CwLogEnabled bool   `json:"CwLogEnabled"`
					CwLogGroup   string `json:"CwLogGroup"`
				} `json:"CwLog"`
			} `json:"DataStorage"`
		} `json:"AppMonitor"`
	}
	if err := json.Unmarshal(b, &res); err != nil {
		return "", fmt.Errorf("%w: rum GetAppMonitor: undecodable response: %w", integrations.ErrProvider, err)
	}
	cw := res.AppMonitor.DataStorage.CwLog
	if !cw.CwLogEnabled || cw.CwLogGroup == "" {
		return "", fmt.Errorf("%w: app monitor %s does not copy its events to CloudWatch Logs; turn on \"Data storage\" (CwLogEnabled) so Logs Insights can count them", integrations.ErrProvider, c.monitor)
	}
	return cw.CwLogGroup, nil
}

// Insights returns the Logs Insights query a baseline runs. The window is
// applied to the RUM event's own event_timestamp (milliseconds), half-open.
func Insights(q integrations.Query) (string, error) {
	if q.Metric != "events" && q.Metric != "users" {
		return "", fmt.Errorf("%w: aws metric %q: want one of %s", integrations.ErrQuery, q.Metric, strings.Join(Metrics, ", "))
	}
	if err := q.Window.Check(); err != nil {
		return "", err
	}
	lines := []string{fmt.Sprintf("filter event_timestamp >= %d and event_timestamp < %d", q.Window.From.UnixMilli(), q.Window.To.UnixMilli())}
	if f := strings.TrimSpace(q.Filter); f != "" {
		if err := cloud.CheckFilter("aws", f, "|"); err != nil {
			return "", err
		}
		lines = append(lines, "| filter "+f)
	}
	if q.Metric == "users" {
		lines = append(lines, "| filter ispresent(user_details.userId)", "| stats count(*) by user_details.userId", "| stats count(*) as value")
	} else {
		lines = append(lines, "| stats count(*) as value")
	}
	return strings.Join(lines, "\n"), nil
}

// Baseline checks the principal, then counts the metric with Logs Insights
// over the app monitor's log group.
func (c *Client) Baseline(ctx context.Context, q integrations.Query) (integrations.Baseline, error) {
	query, err := Insights(q)
	if err != nil {
		return integrations.Baseline{}, err
	}
	if err := c.checked(ctx); err != nil {
		return integrations.Baseline{}, err
	}
	lg, err := c.logGroup(ctx)
	if err != nil {
		return integrations.Baseline{}, err
	}
	// @timestamp is when RUM wrote the event, at or after event_timestamp:
	// the scan runs an hour past the window, and the filter decides.
	start := map[string]any{"logGroupNames": []string{lg}, "startTime": q.Window.From.Unix(), "endTime": q.Window.To.Add(time.Hour).Unix(), "queryString": query}
	var started struct {
		QueryID string `json:"queryId"`
	}
	logs := c.host("logs")
	if err := c.signedJSON(ctx, logs, "/", "logs", "Logs_20140328.StartQuery", start, &started); err != nil {
		return integrations.Baseline{}, err
	}
	if started.QueryID == "" {
		return integrations.Baseline{}, fmt.Errorf("%w: logs StartQuery returned no queryId", integrations.ErrProvider)
	}
	var res struct {
		Status  string `json:"status"`
		Results [][]struct {
			Field string `json:"field"`
			Value string `json:"value"`
		} `json:"results"`
	}
	for i := 0; ; i++ {
		if err := c.signedJSON(ctx, logs, "/", "logs", "Logs_20140328.GetQueryResults", map[string]string{"queryId": started.QueryID}, &res); err != nil {
			return integrations.Baseline{}, err
		}
		if res.Status == "Complete" {
			break
		}
		if (res.Status != "Scheduled" && res.Status != "Running") || i >= 300 {
			return integrations.Baseline{}, fmt.Errorf("%w: logs insights query %s ended %s", integrations.ErrProvider, started.QueryID, res.Status)
		}
		if err := ctx.Err(); err != nil {
			return integrations.Baseline{}, err
		}
		c.Sleep(time.Second)
	}
	var v float64
	switch {
	case len(res.Results) == 0 && q.Metric == "users":
		// No user in the window: the second stats has no row to count.
	case len(res.Results) == 1 && len(res.Results[0]) == 1 && res.Results[0][0].Field == "value":
		if v, err = strconv.ParseFloat(res.Results[0][0].Value, 64); err != nil {
			return integrations.Baseline{}, fmt.Errorf("%w: logs insights value %q", integrations.ErrProvider, res.Results[0][0].Value)
		}
	default:
		return integrations.Baseline{}, fmt.Errorf("%w: logs insights returned %d rows; want one value", integrations.ErrProvider, len(res.Results))
	}
	who := "RUM events"
	if q.Metric == "users" {
		who = "distinct user_details.userId with RUM events"
	}
	if f := strings.TrimSpace(q.Filter); f != "" {
		who += " where " + f
	}
	b, _ := cloud.Marshal(start)
	return integrations.Baseline{
		Metric:      q.Metric,
		Population:  fmt.Sprintf("CloudWatch RUM %s (%s, log group %s): %s", c.monitor, c.region, lg, who),
		Window:      integrations.Window{From: q.Window.From.UTC(), To: q.Window.To.UTC()},
		Value:       v,
		SourceQuery: "POST https://" + logs + "/ Logs_20140328.StartQuery " + string(b),
		FetchedAt:   c.env.Clock().UTC(),
	}, nil
}

// Export pulls the window's raw RUM events (rum:GetAppMonitorData, 100 per
// page) as JSON lines in the CloudWatch RUM export format (kind "aws").
// RUM serves only the last 30 days.
func (c *Client) Export(ctx context.Context, w integrations.Window, out io.Writer) (string, string, error) {
	if err := w.Check(); err != nil {
		return "", "", err
	}
	if w.From.Before(c.env.Clock().Add(-RUMRetention)) {
		return "", "", fmt.Errorf("%w: CloudWatch RUM serves raw events from the last 30 days only; the window starts %s (pass an export with --aws for older data)", integrations.ErrQuery, w.From.UTC().Format(time.DateOnly))
	}
	if err := c.checked(ctx); err != nil {
		return "", "", err
	}
	path := "/appmonitor/" + c.monitor + "/data"
	body := map[string]any{"TimeRange": map[string]int64{"After": w.From.UnixMilli(), "Before": w.To.UnixMilli()}, "MaxResults": 100}
	b, _ := cloud.Marshal(body)
	source := "POST https://" + c.host("rum") + path + " " + string(b)
	seen := map[string]bool{}
	for page := 0; ; page++ {
		var res struct {
			Events    []string `json:"Events"`
			NextToken string   `json:"NextToken"`
		}
		if err := c.signedJSON(ctx, c.host("rum"), path, "rum", "", body, &res); err != nil {
			return "", "", err
		}
		for _, e := range res.Events {
			var line bytes.Buffer
			if json.Compact(&line, []byte(e)) != nil {
				line.Reset()
				line.WriteString(`{"unparsable":true}`) // counted as skipped by the reader
			}
			line.WriteByte('\n')
			if _, err := out.Write(line.Bytes()); err != nil {
				return "", "", err
			}
		}
		if res.NextToken == "" {
			return "aws", source, nil
		}
		if seen[res.NextToken] || page >= 50000 {
			return "", "", fmt.Errorf("%w: rum GetAppMonitorData paging did not end", integrations.ErrProvider)
		}
		seen[res.NextToken] = true
		body["NextToken"] = res.NextToken
	}
}
