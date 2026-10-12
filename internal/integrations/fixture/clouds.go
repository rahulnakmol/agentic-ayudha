package fixture

import (
	"crypto"
	"crypto/hmac"
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
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/proof/sigv4"
)

// Credentials the cloud token server accepts (synthetic; never real).
const (
	AzureTenant       = "8d2f1c3a-0000-4000-8000-00000000aaaa"
	AzureClient       = "5e6f7a8b-0000-4000-8000-00000000bbbb"
	AzureSecret       = "entra-client-secret"
	AWSKeyID          = "TESTAAYUDHAKEYID0001"
	AWSSecret         = "aws-secret-access-key"
	AWSFederatedKeyID = "TESTAAYUDHAFEDERATED"
	AWSFederated      = "aws-federated-secret"
	AWSSession        = "aws-session-token"
	AWSRole           = "arn:aws:iam::123456789012:role/aayudha-read"
	GCPServiceAccount = "aayudha-read@acme-prod.iam.gserviceaccount.com"
	GCPFederation     = "projects/123456789/locations/global/workloadIdentityPools/github/providers/aayudha"
	GitHubRequestTok  = "gh-actions-request-token"
)

// Tokens the server issues: Entra tokens name their scope, so a recording
// can pin which API a bearer was minted for.
const (
	GCPToken     = "gcp-service-account-token"
	GCPFederated = "gcp-federated-token"
)

// EntraToken is the token the server issues for an Entra scope.
func EntraToken(scope string) string { return "entra:" + scope }

// GitHubOIDC is the OIDC token the server issues for an audience.
func GitHubOIDC(aud string) string { return "gh-oidc:" + aud }

// Clouds is an httptest server for the token exchanges the cloud adapters
// make (WP7.2b): Microsoft Entra client credentials and federated
// assertions, Google's JWT bearer grant (the assertion's RS256 signature
// is verified), Google STS and service account impersonation, AWS
// AssumeRoleWithWebIdentity, and the GitHub Actions OIDC endpoint. Each
// exchange checks every parameter and is logged in Exchanges.
type Clouds struct {
	T      testing.TB
	Server *httptest.Server
	Key    *rsa.PrivateKey // the service account key (GCPKeyJSON)
	Now    time.Time
	// GoogleScopes are the scopes Google tokens must be asked for (the
	// read-only BigQuery pair unless a test sets others).
	GoogleScopes string
	mu           sync.Mutex
	exchange     []string
}

// NewClouds starts the token server.
func NewClouds(t testing.TB, now time.Time) *Clouds {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	c := &Clouds{T: t, Key: k, Now: now, GoogleScopes: "https://www.googleapis.com/auth/bigquery.readonly https://www.googleapis.com/auth/cloud-platform.read-only"}
	c.Server = httptest.NewServer(http.HandlerFunc(c.serve))
	t.Cleanup(c.Server.Close)
	return c
}

// Exchanges lists the token exchanges served, in order.
func (c *Clouds) Exchanges() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string{}, c.exchange...)
}

func (c *Clouds) log(s string) {
	c.mu.Lock()
	c.exchange = append(c.exchange, s)
	c.mu.Unlock()
}

// GCPKeyJSON is a service account JSON key for the server's RSA key.
func (c *Clouds) GCPKeyJSON() string {
	der, _ := x509.MarshalPKCS8PrivateKey(c.Key)
	// A key generated for this test server only; never a real credential.
	b, _ := json.Marshal(map[string]string{
		"type":           "service_account", // nosemgrep: generic.secrets.security.detected-google-gcm-service-account.detected-google-gcm-service-account
		"project_id":     "acme-prod",
		"private_key_id": "k1",
		"private_key":    string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})), "client_email": GCPServiceAccount,
		"token_uri": "https://oauth2.googleapis.com/token"})
	return string(b)
}

// Env is the GitHub Actions environment of a job with id-token: write.
func (c *Clouds) Env() map[string]string {
	return map[string]string{"ACTIONS_ID_TOKEN_REQUEST_URL": c.Server.URL + "/_oidc?api-version=2.0", "ACTIONS_ID_TOKEN_REQUEST_TOKEN": GitHubRequestTok}
}

var awsSTS = regexp.MustCompile(`^sts\.[a-z0-9-]+\.amazonaws\.com$`)

// exchangeHost reports whether a request is a token exchange this server answers.
func (c *Clouds) exchangeHost(req *http.Request) bool {
	switch h := req.URL.Host; {
	case h == strings.TrimPrefix(c.Server.URL, "http://"),
		h == "login.microsoftonline.com", h == "oauth2.googleapis.com", h == "sts.googleapis.com", h == "iamcredentials.googleapis.com":
		return true
	case awsSTS.MatchString(h):
		return req.Method == http.MethodPost
	}
	return false
}

// Route sends token exchanges to the server (over real HTTP) and every
// other request to data, which is normally a Replayer.
func (c *Clouds) Route(data http.RoundTripper) http.RoundTripper {
	return route{c, data}
}

type route struct {
	c    *Clouds
	data http.RoundTripper
}

func (r route) RoundTrip(req *http.Request) (*http.Response, error) {
	if !r.c.exchangeHost(req) {
		return r.data.RoundTrip(req)
	}
	out := req.Clone(req.Context())
	u, _ := url.Parse(r.c.Server.URL)
	out.URL.Scheme, out.URL.Host, out.Host = u.Scheme, u.Host, u.Host
	out.Header.Set("X-Fixture-Host", req.URL.Host)
	return http.DefaultTransport.RoundTrip(out)
}

func (c *Clouds) fail(w http.ResponseWriter, format string, a ...any) {
	c.T.Errorf("token exchange: "+format, a...)
	http.Error(w, fmt.Sprintf(format, a...), http.StatusBadRequest)
}

func (c *Clouds) serve(w http.ResponseWriter, r *http.Request) {
	host := r.Header.Get("X-Fixture-Host")
	body, _ := io.ReadAll(r.Body)
	form, _ := url.ParseQuery(string(body))
	reply := func(v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}
	switch {
	case r.URL.Path == "/_oidc":
		if r.Header.Get("Authorization") != "Bearer "+GitHubRequestTok || r.URL.Query().Get("api-version") != "2.0" {
			c.fail(w, "GitHub OIDC request without the request token or api-version")
			return
		}
		aud := r.URL.Query().Get("audience")
		c.log("github-oidc " + aud)
		reply(map[string]string{"value": GitHubOIDC(aud)})

	case host == "login.microsoftonline.com":
		if r.Method != http.MethodPost || r.URL.Path != "/"+AzureTenant+"/oauth2/v2.0/token" || form.Get("grant_type") != "client_credentials" {
			c.fail(w, "entra: %s %s grant %q", r.Method, r.URL.Path, form.Get("grant_type"))
			return
		}
		scope := form.Get("scope")
		if scope != "https://api.loganalytics.io/.default" && scope != "https://management.azure.com/.default" && scope != "https://storage.azure.com/.default" {
			c.fail(w, "entra: scope %q", scope)
			return
		}
		switch {
		case form.Get("client_secret") != "":
			if form.Get("client_id") != AzureClient || form.Get("client_secret") != AzureSecret || form.Has("client_assertion") {
				c.fail(w, "entra: wrong client credentials")
				return
			}
			c.log("entra client_secret " + scope)
		case form.Get("client_assertion") != "":
			if form.Get("client_id") != AzureClient || form.Get("client_assertion_type") != "urn:ietf:params:oauth:client-assertion-type:jwt-bearer" ||
				form.Get("client_assertion") != GitHubOIDC("api://AzureADTokenExchange") {
				c.fail(w, "entra: wrong federated assertion")
				return
			}
			c.log("entra federated " + scope)
		default:
			c.fail(w, "entra: no credential")
			return
		}
		reply(map[string]any{"token_type": "Bearer", "expires_in": 3599, "access_token": EntraToken(scope)})

	case host == "oauth2.googleapis.com":
		if r.URL.Path != "/token" || form.Get("grant_type") != "urn:ietf:params:oauth:grant-type:jwt-bearer" {
			c.fail(w, "google oauth: %s grant %q", r.URL.Path, form.Get("grant_type"))
			return
		}
		if err := c.verifyJWT(form.Get("assertion")); err != nil {
			c.fail(w, "google oauth: %v", err)
			return
		}
		c.log("google jwt-bearer " + GCPServiceAccount)
		reply(map[string]any{"access_token": GCPToken, "expires_in": 3599, "token_type": "Bearer"})

	case host == "sts.googleapis.com":
		var in map[string]string
		_ = json.Unmarshal(body, &in)
		want := map[string]string{"audience": "//iam.googleapis.com/" + GCPFederation, "grantType": "urn:ietf:params:oauth:grant-type:token-exchange",
			"requestedTokenType": "urn:ietf:params:oauth:token-type:access_token", "scope": "https://www.googleapis.com/auth/cloud-platform",
			"subjectTokenType": "urn:ietf:params:oauth:token-type:jwt", "subjectToken": GitHubOIDC("https://iam.googleapis.com/" + GCPFederation)}
		for k, v := range want {
			if in[k] != v {
				c.fail(w, "google sts: %s = %q, want %q", k, in[k], v)
				return
			}
		}
		c.log("google sts token-exchange")
		reply(map[string]any{"access_token": GCPFederated, "issued_token_type": "urn:ietf:params:oauth:token-type:access_token", "token_type": "Bearer", "expires_in": 3599})

	case host == "iamcredentials.googleapis.com":
		var in struct {
			Scope    []string `json:"scope"`
			Lifetime string   `json:"lifetime"`
		}
		_ = json.Unmarshal(body, &in)
		if r.URL.Path != "/v1/projects/-/serviceAccounts/"+GCPServiceAccount+":generateAccessToken" || r.Header.Get("Authorization") != "Bearer "+GCPFederated ||
			strings.Join(in.Scope, " ") != c.GoogleScopes {
			c.fail(w, "generateAccessToken: %s auth %q scopes %v", r.URL.Path, r.Header.Get("Authorization"), in.Scope)
			return
		}
		c.log("google impersonate " + GCPServiceAccount)
		reply(map[string]string{"accessToken": GCPToken, "expireTime": c.Now.Add(time.Hour).UTC().Format(time.RFC3339)})

	case awsSTS.MatchString(host):
		want := url.Values{"Action": {"AssumeRoleWithWebIdentity"}, "Version": {"2011-06-15"}, "RoleArn": {AWSRole}, "RoleSessionName": {"aayudha"},
			"DurationSeconds": {"3600"}, "WebIdentityToken": {GitHubOIDC("sts.amazonaws.com")}}
		if form.Encode() != want.Encode() || r.Header.Get("Authorization") != "" {
			c.fail(w, "aws sts: form %s (signed: %t)", form.Encode(), r.Header.Get("Authorization") != "")
			return
		}
		c.log("aws assume-role-with-web-identity " + AWSRole)
		w.Header().Set("Content-Type", "text/xml")
		fmt.Fprintf(w, `<AssumeRoleWithWebIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/"><AssumeRoleWithWebIdentityResult>`+
			`<Credentials><AccessKeyId>%s</AccessKeyId><SecretAccessKey>%s</SecretAccessKey><SessionToken>%s</SessionToken><Expiration>%s</Expiration></Credentials>`+
			`<AssumedRoleUser><Arn>arn:aws:sts::123456789012:assumed-role/aayudha-read/aayudha</Arn></AssumedRoleUser></AssumeRoleWithWebIdentityResult></AssumeRoleWithWebIdentityResponse>`,
			AWSFederatedKeyID, AWSFederated, AWSSession, c.Now.Add(time.Hour).UTC().Format(time.RFC3339))

	default:
		c.fail(w, "unexpected %s %s%s", r.Method, host, r.URL.Path)
	}
}

// verifyJWT checks a service account assertion: RS256 over the server's
// key, issued by the service account for Google's token endpoint with the
// scopes the server expects, valid for one hour from now.
func (c *Clouds) verifyJWT(jwt string) error {
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		return fmt.Errorf("assertion is not a JWT")
	}
	dec := base64.RawURLEncoding
	sig, err := dec.DecodeString(parts[2])
	if err != nil {
		return err
	}
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(&c.Key.PublicKey, crypto.SHA256, sum[:], sig); err != nil {
		return fmt.Errorf("assertion signature: %w", err)
	}
	var h, cl map[string]any
	hb, _ := dec.DecodeString(parts[0])
	cb, _ := dec.DecodeString(parts[1])
	if json.Unmarshal(hb, &h) != nil || json.Unmarshal(cb, &cl) != nil {
		return fmt.Errorf("assertion is not JSON")
	}
	if h["alg"] != "RS256" || h["kid"] != "k1" || cl["iss"] != GCPServiceAccount || cl["aud"] != "https://oauth2.googleapis.com/token" ||
		cl["scope"] != c.GoogleScopes ||
		cl["iat"] != float64(c.Now.Unix()) || cl["exp"] != float64(c.Now.Add(time.Hour).Unix()) {
		return fmt.Errorf("assertion header %v claims %v", h, cl)
	}
	return nil
}

// SigV4 returns a Replayer check that recomputes every AWS request's
// Signature Version 4 with the secret of its access key (AWSKeyID, or the
// federated AWSFederatedKeyID, which must carry AWSSession), as AWS does.
func SigV4() func(*http.Request, []byte) error {
	secrets := map[string]string{AWSKeyID: AWSSecret, AWSFederatedKeyID: AWSFederated, "TESTAAYUDHACLIKEYID1": "aws-cli-secret"}
	return func(r *http.Request, body []byte) error {
		auth := r.Header.Get("Authorization")
		m := regexp.MustCompile(`^AWS4-HMAC-SHA256 Credential=([^/]+)/[0-9]{8}/([^/]+)/([^/]+)/aws4_request, SignedHeaders=[^,]+, Signature=[0-9a-f]{64}$`).FindStringSubmatch(auth)
		if m == nil {
			return fmt.Errorf("not SigV4: %q", auth)
		}
		secret, ok := secrets[m[1]]
		if !ok {
			return fmt.Errorf("unknown access key %s", m[1])
		}
		if (m[1] == AWSFederatedKeyID) != (r.Header.Get("X-Amz-Security-Token") == AWSSession) {
			return fmt.Errorf("session token %q with key %s", r.Header.Get("X-Amz-Security-Token"), m[1])
		}
		ts, err := time.Parse("20060102T150405Z", r.Header.Get("X-Amz-Date"))
		if err != nil {
			return err
		}
		check, _ := http.NewRequest(r.Method, r.URL.String(), nil)
		for k, v := range r.Header {
			if k != "Authorization" && k != "X-Amz-Date" && k != "X-Amz-Content-Sha256" {
				check.Header[k] = v
			}
		}
		sigv4.Sign(check, body, m[1], secret, m[2], m[3], ts)
		if !hmac.Equal([]byte(check.Header.Get("Authorization")), []byte(auth)) {
			return fmt.Errorf("signature mismatch")
		}
		return nil
	}
}
