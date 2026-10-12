package providers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
	"github.com/rahulnakmol/agentic-ayudha/internal/secrets"

	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/fixture"
)

const azureResource = "/subscriptions/0b1c2d3e-0000-4000-8000-000000000001/resourceGroups/rg-benefits/providers/Microsoft.Insights/components/benefits-qa"

// cloudSets configures azure, aws, and gcp with the given auth.
func cloudSets(auth string) []string {
	sets := []string{
		"integrations.azure.scope=read", "integrations.azure.project=" + azureResource,
		"integrations.aws.scope=read", "integrations.aws.region=eu-west-1", "integrations.aws.project=benefits-qa",
		"integrations.gcp.scope=read", "integrations.gcp.project=acme-prod.analytics_123456789",
	}
	switch auth {
	case "key":
		sets = append(sets, "integrations.azure.secret=azure", "integrations.azure.org="+fixture.AzureTenant,
			"integrations.aws.secret=aws", "integrations.gcp.secret=gcp")
	case "oidc":
		sets = append(sets, "integrations.azure.auth=oidc", "integrations.azure.org="+fixture.AzureTenant, "integrations.azure.identity="+fixture.AzureClient,
			"integrations.aws.auth=oidc", "integrations.aws.identity="+fixture.AWSRole,
			"integrations.gcp.auth=oidc", "integrations.gcp.federation="+fixture.GCPFederation, "integrations.gcp.identity="+fixture.GCPServiceAccount)
	case "cli":
		sets = append(sets, "integrations.azure.auth=cli", "integrations.aws.auth=cli",
			"integrations.gcp.auth=cli", "integrations.gcp.identity="+fixture.GCPServiceAccount)
	}
	return sets
}

// cloudDeps wires the cloud adapters to the token server (exchanges) and
// r (data plane), with SigV4 verified on every AWS request.
func cloudDeps(t *testing.T, r *fixture.Replayer, auth string) Deps {
	d, _ := cloudDepsWith(t, r, auth)
	return d
}

func cloudDepsWith(t *testing.T, r *fixture.Replayer, auth string) (Deps, *fixture.Clouds) {
	t.Helper()
	clouds := fixture.NewClouds(t, fetched)
	r.Check = func(req *http.Request, body []byte) error {
		if strings.HasSuffix(req.URL.Host, ".amazonaws.com") {
			return fixture.SigV4()(req, body)
		}
		return nil
	}
	env := map[string]string{
		"AAYU_SECRET_AZURE": fixture.AzureClient + ":" + fixture.AzureSecret,
		"AAYU_SECRET_AWS":   fixture.AWSKeyID + ":" + fixture.AWSSecret,
		"AAYU_SECRET_GCP":   clouds.GCPKeyJSON(),
	}
	d := deps(t, clouds.Route(r), env, cloudSets(auth)...)
	d.Getenv = func(k string) string { return clouds.Env()[k] }
	d.Sleep = func(time.Duration) {}
	if auth != "oidc" {
		d.Getenv = func(string) string { return "" }
	}
	return d, clouds
}

// recording loads the first n interactions of a golden cloud recording.
func recording(t *testing.T, source string, n int) *fixture.Recording {
	t.Helper()
	r := fixture.Load(t, "testdata/"+source+".json")
	r.Interactions = r.Interactions[:n]
	return r
}

// TestCloudAuthModes: each cloud reaches its scope check with each kind of
// credential (WP7.2b): a stored key, GitHub Actions OIDC federation (no
// stored key; the exchanges are checked parameter by parameter by the token
// server), and the local CLI login, which runs with a scrubbed environment.
func TestCloudAuthModes(t *testing.T) {
	userARN := url.QueryEscape("arn:aws:iam::123456789012:user/aayudha-read")
	assumed := `<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/"><GetCallerIdentityResult><Arn>arn:aws:sts::123456789012:assumed-role/aayudha-read/aayudha</Arn><UserId>AROAAAYUDHAEXAMPLE01:aayudha</UserId><Account>123456789012</Account></GetCallerIdentityResult></GetCallerIdentityResponse>`
	for _, auth := range []string{"key", "oidc", "cli"} {
		for _, tt := range []struct {
			source    string
			n         int
			bearer    string // the data-plane bearer, per auth
			exchanges []string
			cli       string // the CLI call for auth = "cli"
		}{
			{"azure", 1, map[string]string{"key": "Bearer entra:https://management.azure.com/.default", "oidc": "Bearer entra:https://management.azure.com/.default", "cli": "Bearer az-cli-token"}[auth],
				map[string][]string{"key": {"entra client_secret https://management.azure.com/.default"},
					"oidc": {"github-oidc api://AzureADTokenExchange", "entra federated https://management.azure.com/.default"}}[auth],
				"az account get-access-token --resource https://management.azure.com/ --output json"},
			{"aws", 2, "", map[string][]string{"oidc": {"github-oidc sts.amazonaws.com", "aws assume-role-with-web-identity " + fixture.AWSRole}}[auth],
				"aws configure export-credentials --format process"},
			{"gcp", 3, map[string]string{"key": "Bearer " + fixture.GCPToken, "oidc": "Bearer " + fixture.GCPToken, "cli": "Bearer gcloud-token"}[auth],
				map[string][]string{"key": {"google jwt-bearer " + fixture.GCPServiceAccount},
					"oidc": {"github-oidc https://iam.googleapis.com/" + fixture.GCPFederation, "google sts token-exchange", "google impersonate " + fixture.GCPServiceAccount}}[auth],
				"gcloud auth print-access-token --impersonate-service-account=" + fixture.GCPServiceAccount},
		} {
			t.Run(tt.source+"/"+auth, func(t *testing.T) {
				rec := recording(t, tt.source, tt.n)
				for i := range rec.Interactions {
					if tt.bearer != "" {
						rec.Interactions[i].Auth = tt.bearer
					}
				}
				if tt.source == "aws" && auth != "key" {
					// A role session: the simulation runs on the role.
					rec.Interactions[0].Text = assumed
					rec.Interactions[1].URL = strings.Replace(rec.Interactions[1].URL, userARN, url.QueryEscape(fixture.AWSRole), 1)
				}
				r := &fixture.Replayer{T: t, Rec: rec}
				d, clouds := cloudDepsWith(t, r, auth)
				var calls []string
				d.Exec = func(_ context.Context, name string, args, env []string) ([]byte, error) {
					calls = append(calls, strings.Join(append([]string{name}, args...), " "))
					if !slices.Equal(env, []string{"PATH=", "HOME="}) {
						t.Errorf("CLI environment %v: want only PATH and HOME", env)
					}
					return []byte(map[string]string{
						"az":     `{"accessToken": "az-cli-token", "expiresOn": "2026-10-12 10:29:00.000000", "expires_on": 1791797340, "tokenType": "Bearer"}`,
						"aws":    `{"Version": 1, "AccessKeyId": "TESTAAYUDHACLIKEYID1", "SecretAccessKey": "aws-cli-secret", "SessionToken": "cli-session", "Expiration": "2026-10-12T10:30:00+00:00"}`,
						"gcloud": "gcloud-token\n",
					}[name]), nil
				}
				sc, err := Check(context.Background(), d, tt.source)
				if err != nil {
					t.Fatal(err)
				}
				if !sc.Verified || len(sc.Write) != 0 || len(sc.Scopes) == 0 {
					t.Errorf("scope %+v: want verified, read-only", sc)
				}
				r.Done()
				if got := clouds.Exchanges(); !slices.Equal(got, tt.exchanges) {
					t.Errorf("token exchanges %q, want %q", got, tt.exchanges)
				}
				want := []string(nil)
				if auth == "cli" {
					want = []string{tt.cli}
				}
				if !slices.Equal(calls, want) {
					t.Errorf("CLI calls %q, want %q", calls, want)
				}
			})
		}
	}
}

// TestCloudWriteCapableRefused: a credential the cloud reports as able to
// write is refused after the scope requests and before any data request
// (WP1.3), whether the write comes from a role on the resource (Azure), an
// allowed policy simulation (AWS), or a dataset-level grant that the
// project does not show (GCP).
func TestCloudWriteCapableRefused(t *testing.T) {
	for _, tt := range []struct {
		source string
		n      int
		edit   func(*fixture.Recording)
		q      integrations.Query
		naming string
	}{
		{"azure", 1, func(r *fixture.Recording) {
			r.Interactions[0].Response = json.RawMessage(`{"value":[{"actions":["*/read","Microsoft.Insights/components/*"],"notActions":[],"dataActions":[],"notDataActions":[]}]}`)
		}, integrations.Query{Metric: "events", Window: sept}, "Microsoft.Insights/components/*"},
		{"aws", 2, func(r *fixture.Recording) {
			r.Interactions[1].Text = strings.Replace(r.Interactions[1].Text,
				"<EvalActionName>logs:PutLogEvents</EvalActionName><EvalResourceName>*</EvalResourceName><EvalDecision>implicitDeny</EvalDecision>",
				"<EvalActionName>logs:PutLogEvents</EvalActionName><EvalResourceName>*</EvalResourceName><EvalDecision>allowed</EvalDecision>", 1)
		}, integrations.Query{Metric: "events", Window: sept}, "logs:PutLogEvents"},
		{"gcp", 3, func(r *fixture.Recording) {
			r.Interactions[2].Response = json.RawMessage(`{"permissions":["bigquery.tables.updateData"]}`)
		}, integrations.Query{Metric: "events", Window: sept}, "bigquery.tables.updateData (dataset analytics_123456789)"},
	} {
		t.Run(tt.source, func(t *testing.T) {
			rec := recording(t, tt.source, tt.n)
			tt.edit(rec)
			r := &fixture.Replayer{T: t, Rec: rec}
			_, err := Baseline(context.Background(), cloudDeps(t, r, "key"), tt.source, tt.q)
			if !errors.Is(err, secrets.ErrWriteScoped) || !strings.Contains(err.Error(), tt.naming) {
				t.Fatalf("err = %v, want ErrWriteScoped naming %s", err, tt.naming)
			}
			r.Done()
		})
	}
}

// TestCloudRefusedBeforeAnyCall: scope = "write", an unsafe filter (it
// comes from an issue body, §0.1), an unknown metric, and a RUM window older than 30 days are
// refused with zero requests; a missing section names what to set.
func TestCloudRefusedBeforeAnyCall(t *testing.T) {
	for _, tt := range []struct {
		name, source string
		sets         []string
		q            integrations.Query
		want         error
		naming       string
	}{
		{"azure write scope", "azure", []string{"integrations.azure.scope=write"}, integrations.Query{Metric: "events", Window: sept}, secrets.ErrWriteScoped, "write"},
		{"aws write scope", "aws", []string{"integrations.aws.scope=write"}, integrations.Query{Metric: "events", Window: sept}, secrets.ErrWriteScoped, "write"},
		{"gcp write scope", "gcp", []string{"integrations.gcp.scope=write"}, integrations.Query{Metric: "events", Window: sept}, secrets.ErrWriteScoped, "write"},
		{"kql pipe", "azure", nil, integrations.Query{Metric: "events", Filter: "Name == 'x' | union AppTraces", Window: sept}, integrations.ErrQuery, `"|" is not allowed`},
		{"insights pipe", "aws", nil, integrations.Query{Metric: "users", Filter: "event_type = 'x' | stats count(*)", Window: sept}, integrations.ErrQuery, `"|" is not allowed`},
		{"sql statement", "gcp", nil, integrations.Query{Metric: "users", Filter: "TRUE; DROP TABLE x", Window: sept}, integrations.ErrQuery, `";" is not allowed`},
		{"unknown metric", "gcp", nil, integrations.Query{Metric: "revenue", Window: sept}, integrations.ErrQuery, "want one of events, users"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rt := &refuse{t: t}
			// The CLI login: nothing is stored, and the refusals come before it is asked.
			d := deps(t, rt, nil, append(cloudSets("cli"), tt.sets...)...)
			d.Exec = func(context.Context, string, []string, []string) ([]byte, error) {
				t.Error("a cloud CLI was run")
				return nil, errors.New("no CLI")
			}
			_, err := Baseline(context.Background(), d, tt.source, tt.q)
			if !errors.Is(err, tt.want) || !strings.Contains(err.Error(), tt.naming) {
				t.Fatalf("err = %v, want %v naming %q", err, tt.want, tt.naming)
			}
			if rt.n != 0 {
				t.Fatalf("%d requests sent", rt.n)
			}
		})
	}
	t.Run("rum window older than 30 days", func(t *testing.T) {
		rt := &refuse{t: t}
		d := deps(t, rt, map[string]string{"AAYU_SECRET_AWS": fixture.AWSKeyID + ":" + fixture.AWSSecret}, cloudSets("key")...)
		_, _, err := Export(context.Background(), d, "aws", sept, io.Discard)
		if !errors.Is(err, integrations.ErrQuery) || !strings.Contains(err.Error(), "last 30 days") || rt.n != 0 {
			t.Fatalf("err = %v after %d requests", err, rt.n)
		}
	})
	t.Run("azure absent", func(t *testing.T) {
		_, err := Baseline(context.Background(), deps(t, &refuse{t: t}, nil), "azure", integrations.Query{Metric: "events", Window: sept})
		for _, w := range []string{"[integrations.azure] is not set", `project = "<the Azure resource ID`, `org = "<the Microsoft Entra tenant ID>"`} {
			if !errors.Is(err, integrations.ErrNotConfigured) || !strings.Contains(err.Error(), w) {
				t.Errorf("err = %v, want ErrNotConfigured naming %s", err, w)
			}
		}
	})
}

// TestAWSSimulationDenied: a principal without iam:SimulatePrincipalPolicy
// is reachable but unverified, so doctor says the configured scope is
// trusted.
func TestAWSSimulationDenied(t *testing.T) {
	rec := recording(t, "aws", 2)
	rec.Interactions[1].Status = http.StatusForbidden
	rec.Interactions[1].Text = `<ErrorResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/"><Error><Type>Sender</Type><Code>AccessDenied</Code><Message>User is not authorized to perform: iam:SimulatePrincipalPolicy</Message></Error></ErrorResponse>`
	r := &fixture.Replayer{T: t, Rec: rec}
	sc, err := Check(context.Background(), cloudDeps(t, r, "key"), "aws")
	if err != nil || sc.Verified || sc.Account != "arn:aws:iam::123456789012:user/aayudha-read" {
		t.Fatalf("scope %+v, err %v: want reachable, unverified", sc, err)
	}
	r.Done()
}
