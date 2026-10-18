package initrepo

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestAppHookURL: aayu reads the new App's webhook as the App, with a JWT
// signed by the App's key (RS256, issuer the App's ID), and fails closed on a
// key it cannot sign with or an answer it cannot read, so the key is never
// stored for an App whose webhook is unknown (#578).
func TestAppHookURL(t *testing.T) {
	rk, _ := rsa.GenerateKey(rand.Reader, 2048)
	ek, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	pkcs1 := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(rk)}))
	p8, _ := x509.MarshalPKCS8PrivateKey(rk)
	pkcs8 := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: p8}))
	e8, _ := x509.MarshalPKCS8PrivateKey(ek)
	ecKey := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: e8}))

	answer, body := http.StatusOK, `{"url":"https://example.invalid/aayudha-no-webhook"}`
	var iss string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), ".")
		if len(parts) != 3 || r.URL.Path != "/app/hook/config" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		sig, _ := base64.RawURLEncoding.DecodeString(parts[2])
		sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
		if rsa.VerifyPKCS1v15(&rk.PublicKey, crypto.SHA256, sum[:], sig) != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		c, _ := base64.RawURLEncoding.DecodeString(parts[1])
		iss = string(c)
		w.WriteHeader(answer)
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()
	ctx := context.Background()

	for _, k := range []string{pkcs1, pkcs8} {
		if got, err := appHookURL(ctx, srv.URL, 4242, k); err != nil || got != noWebhook || !strings.Contains(iss, `"iss":"4242"`) {
			t.Fatalf("hook = %q, %v; claims %s", got, err, iss)
		}
	}
	for name, tt := range map[string]struct {
		base, key, want string
		code            int
		body            string
	}{
		"not PEM":            {srv.URL, "nope", "not a PEM key", 200, body},
		"not an RSA key":     {srv.URL, ecKey, "not an RSA key", 200, body},
		"refused":            {srv.URL, pkcs1, "HTTP 404", 404, body},
		"not JSON":           {srv.URL, pkcs1, "invalid character", 200, "<html>"},
		"bad base":           {"http://bad\x7f", pkcs1, "invalid control character", 200, body},
		"GitHub unreachable": {"http://127.0.0.1:1", pkcs1, "connect", 200, body},
	} {
		answer, body = tt.code, tt.body
		if got, err := appHookURL(ctx, tt.base, 4242, tt.key); err == nil || got != "" || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: %q, %v; want an error with %q", name, got, err, tt.want)
		}
	}
}
