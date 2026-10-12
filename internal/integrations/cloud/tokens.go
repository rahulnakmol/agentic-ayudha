package cloud

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
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
)

// Token endpoints.
const (
	EntraHost      = "login.microsoftonline.com"
	GoogleTokenURL = "https://oauth2.googleapis.com/token"
	GoogleSTSURL   = "https://sts.googleapis.com/v1/token"
	IAMCredentials = "iamcredentials.googleapis.com"
	// EntraAudience is the audience Entra expects on a federated GitHub token.
	EntraAudience = "api://AzureADTokenExchange"
)

// Entra is a Microsoft Entra identity: an app registration's client
// secret (auth "key"), a GitHub Actions federated credential ("oidc"), or
// the az CLI login ("cli").
type Entra struct {
	Auth, Tenant, Client, Secret string
}

// Token mints an Entra access token for scope (an Azure resource's
// "<resource>/.default").
func (e Env) Entra(ctx context.Context, hc *http.Client, id Entra, scope string) (Token, error) {
	now := e.Clock()
	form := url.Values{"grant_type": {"client_credentials"}, "scope": {scope}}
	switch id.Auth {
	case "cli":
		args := []string{"account", "get-access-token", "--resource", strings.TrimSuffix(scope, ".default"), "--output", "json"}
		if id.Tenant != "" {
			args = append(args, "--tenant", id.Tenant)
		}
		out, err := e.CLI(ctx, "az", args...)
		if err != nil {
			return Token{}, err
		}
		var res struct {
			AccessToken string `json:"accessToken"`
			ExpiresOn   int64  `json:"expires_on"`
		}
		if json.Unmarshal(out, &res) != nil || res.AccessToken == "" {
			return Token{}, fmt.Errorf("%w: `az account get-access-token` printed no accessToken", integrations.ErrProvider)
		}
		exp := time.Unix(res.ExpiresOn, 0)
		if res.ExpiresOn == 0 {
			exp = now.Add(5 * time.Minute) // older az prints only a local expiresOn
		}
		return Token{Value: res.AccessToken, Expires: exp}, nil
	case "oidc":
		assertion, err := e.GitHubOIDC(ctx, hc, EntraAudience)
		if err != nil {
			return Token{}, err
		}
		form.Set("client_id", id.Client)
		form.Set("client_assertion_type", "urn:ietf:params:oauth:client-assertion-type:jwt-bearer")
		form.Set("client_assertion", assertion)
	default:
		form.Set("client_id", id.Client)
		form.Set("client_secret", id.Secret)
	}
	var res OAuth
	if err := PostForm(ctx, hc, "https://"+EntraHost+"/"+url.PathEscape(id.Tenant)+"/oauth2/v2.0/token", form, &res); err != nil {
		return Token{}, err
	}
	return res.Token(now, "Microsoft Entra")
}

// ServiceAccountKey is a Google service account JSON key.
type ServiceAccountKey struct {
	Type         string `json:"type"`
	ClientEmail  string `json:"client_email"`
	PrivateKey   string `json:"private_key"`
	PrivateKeyID string `json:"private_key_id"`
	TokenURI     string `json:"token_uri"`
	rsa          *rsa.PrivateKey
}

// ParseServiceAccountKey reads a service account JSON key. Only Google's own
// token endpoint is trusted as its token_uri.
func ParseServiceAccountKey(s string) (*ServiceAccountKey, error) {
	var k ServiceAccountKey
	if err := json.Unmarshal([]byte(s), &k); err != nil || k.Type != "service_account" || k.ClientEmail == "" {
		return nil, fmt.Errorf("want a service account JSON key (type service_account)")
	}
	if k.TokenURI == "" {
		k.TokenURI = GoogleTokenURL
	}
	if k.TokenURI != GoogleTokenURL {
		return nil, fmt.Errorf("token_uri %q: only %s is trusted", k.TokenURI, GoogleTokenURL)
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
func (k *ServiceAccountKey) assertion(now time.Time, scopes []string) (string, error) {
	h, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": k.PrivateKeyID})
	c, _ := json.Marshal(map[string]any{"iss": k.ClientEmail, "scope": strings.Join(scopes, " "), "aud": k.TokenURI,
		"iat": now.Unix(), "exp": now.Add(time.Hour).Unix()})
	signing := b64(h) + "." + b64(c)
	sum := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, k.rsa, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return signing + "." + b64(sig), nil
}

// Google is a Google Cloud identity: a service account key (auth "key"),
// GitHub Actions workload identity federation through a provider, then
// impersonation of ServiceAccount ("oidc"), or the gcloud login, optionally
// impersonating ServiceAccount ("cli").
type Google struct {
	Auth           string
	Key            *ServiceAccountKey
	Federation     string // projects/<number>/locations/global/workloadIdentityPools/<pool>/providers/<provider>
	ServiceAccount string
}

// Account names the identity for messages.
func (g Google) Account() string {
	switch {
	case g.Auth == "cli" && g.ServiceAccount != "":
		return g.ServiceAccount + " (impersonated through gcloud)"
	case g.Auth == "cli":
		return "the gcloud login"
	case g.Auth == "oidc":
		return g.ServiceAccount + " (GitHub Actions workload identity federation)"
	}
	return g.Key.ClientEmail
}

// Google mints a Google access token carrying scopes (the gcloud login
// carries its own).
func (e Env) Google(ctx context.Context, hc *http.Client, id Google, scopes []string) (Token, error) {
	now := e.Clock()
	switch id.Auth {
	case "cli":
		args := []string{"auth", "print-access-token"}
		if id.ServiceAccount != "" {
			args = append(args, "--impersonate-service-account="+id.ServiceAccount)
		}
		out, err := e.CLI(ctx, "gcloud", args...)
		if err != nil {
			return Token{}, err
		}
		t := strings.TrimSpace(string(out))
		if t == "" || strings.ContainsAny(t, " \n") {
			return Token{}, fmt.Errorf("%w: `gcloud auth print-access-token` printed no token", integrations.ErrProvider)
		}
		return Token{Value: t, Expires: now.Add(5 * time.Minute)}, nil
	case "oidc":
		jwt, err := e.GitHubOIDC(ctx, hc, "https://iam.googleapis.com/"+id.Federation)
		if err != nil {
			return Token{}, err
		}
		var fed OAuth
		if err := PostJSON(ctx, hc, GoogleSTSURL, "", map[string]string{
			"audience":           "//iam.googleapis.com/" + id.Federation,
			"grantType":          "urn:ietf:params:oauth:grant-type:token-exchange",
			"requestedTokenType": "urn:ietf:params:oauth:token-type:access_token",
			"scope":              "https://www.googleapis.com/auth/cloud-platform",
			"subjectTokenType":   "urn:ietf:params:oauth:token-type:jwt",
			"subjectToken":       jwt,
		}, &fed); err != nil {
			return Token{}, err
		}
		if fed.AccessToken == "" {
			return Token{}, fmt.Errorf("%w: Google STS returned no access token", integrations.ErrProvider)
		}
		var sa struct {
			AccessToken string `json:"accessToken"`
			ExpireTime  string `json:"expireTime"`
		}
		u := "https://" + IAMCredentials + "/v1/projects/-/serviceAccounts/" + id.ServiceAccount + ":generateAccessToken"
		if err := PostJSON(ctx, hc, u, fed.AccessToken, map[string]any{"scope": scopes, "lifetime": "3600s"}, &sa); err != nil {
			return Token{}, err
		}
		exp, err := time.Parse(time.RFC3339, sa.ExpireTime)
		if sa.AccessToken == "" || err != nil {
			return Token{}, fmt.Errorf("%w: generateAccessToken returned no token", integrations.ErrProvider)
		}
		return Token{Value: sa.AccessToken, Expires: exp}, nil
	default:
		a, err := id.Key.assertion(now, scopes)
		if err != nil {
			return Token{}, fmt.Errorf("%w: signing the service account assertion: %w", integrations.ErrNotConfigured, err)
		}
		var res OAuth
		if err := PostForm(ctx, hc, id.Key.TokenURI, url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"}, "assertion": {a}}, &res); err != nil {
			return Token{}, err
		}
		return res.Token(now, "Google OAuth")
	}
}
