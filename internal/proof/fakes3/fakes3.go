// Package fakes3 is an in-memory S3-compatible store for tests of the proof
// media bucket (WP4.2): objects and lifecycle configuration, with every
// request's SigV4 signature recomputed and checked.
package fakes3

import (
	"crypto/hmac"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/proof/sigv4"
)

// S3 is an S3-compatible store for tests: objects and lifecycle
// configuration in memory, every request's SigV4 signature recomputed.
type S3 struct {
	Mu        sync.Mutex
	Objects   map[string][]byte // "bucket/key"
	Lifecycle map[string][]byte // bucket -> XML
	Secret    string
	Server    *httptest.Server
	Rejected  int
}

// New starts a fake whose every key pair has secret key secret.
func New(t testing.TB, secret string) *S3 {
	f := &S3{Objects: map[string][]byte{}, Lifecycle: map[string][]byte{}, Secret: secret}
	f.Server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.Server.Close)
	return f
}

func (f *S3) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	auth := r.Header.Get("Authorization")
	check, _ := http.NewRequest(r.Method, "http://"+r.Host+r.URL.RequestURI(), nil)
	for k, v := range r.Header {
		if k != "Authorization" && k != "X-Amz-Date" && k != "X-Amz-Content-Sha256" {
			check.Header[k] = v
		}
	}
	ts, _ := time.Parse("20060102T150405Z", r.Header.Get("X-Amz-Date"))
	access := ""
	if i := strings.Index(auth, "Credential="); i >= 0 {
		access, _, _ = strings.Cut(auth[i+len("Credential="):], "/")
	}
	region := strings.Split(auth, "/")
	reg := "us-east-1"
	if len(region) > 2 {
		reg = region[2]
	}
	sigv4.Sign(check, body, access, f.Secret, reg, "s3", ts)
	f.Mu.Lock()
	defer f.Mu.Unlock()
	if auth == "" || !hmac.Equal([]byte(check.Header.Get("Authorization")), []byte(auth)) {
		f.Rejected++
		http.Error(w, "<Error><Code>SignatureDoesNotMatch</Code></Error>", http.StatusForbidden)
		return
	}
	bucket, key, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if _, ok := r.URL.Query()["lifecycle"]; ok {
		switch r.Method {
		case http.MethodGet:
			if b, ok := f.Lifecycle[bucket]; ok {
				_, _ = w.Write(b)
				return
			}
			http.Error(w, "<Error><Code>NoSuchLifecycleConfiguration</Code></Error>", http.StatusNotFound)
		case http.MethodPut:
			if r.Header.Get("Content-MD5") == "" && r.Header.Get("X-Amz-Checksum-Sha256") == "" {
				http.Error(w, "<Error><Code>InvalidRequest</Code><Message>Missing required header for this request: Content-MD5 or x-amz-checksum-*</Message></Error>", http.StatusBadRequest)
				return
			}
			f.Lifecycle[bucket] = body
		case http.MethodDelete:
			delete(f.Lifecycle, bucket)
			w.WriteHeader(http.StatusNoContent)
		}
		return
	}
	k := bucket + "/" + key
	switch r.Method {
	case http.MethodPut:
		f.Objects[k] = body
	case http.MethodGet:
		b, ok := f.Objects[k]
		if !ok {
			http.Error(w, "<Error><Code>NoSuchKey</Code></Error>", http.StatusNotFound)
			return
		}
		_, _ = w.Write(b)
	case http.MethodDelete:
		delete(f.Objects, k)
		w.WriteHeader(http.StatusNoContent)
	}
}
