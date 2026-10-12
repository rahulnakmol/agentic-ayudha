package proof

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/proof/sigv4"
)

// Bucket is the private proof media store named by [proof] media (V3): any
// S3-compatible service (AWS S3, Cloudflare R2, MinIO), reached with SigV4.
// No S3 SDK is on the plan's dependency list (§2), so the four calls the
// store needs are signed here (Appendix B D-4.2a).
type Bucket struct {
	URL       string // the [proof] media value, for messages
	Endpoint  string // scheme://host[:port]; path-style requests
	Region    string
	Name      string
	Prefix    string // object key prefix, no leading or trailing "/"
	AccessKey string
	SecretKey string
	HTTP      *http.Client
	Now       func() time.Time
}

// ParseMedia parses a [proof] media value:
//
//	s3://bucket[/prefix][?region=eu-west-2][&endpoint=https://minio.example:9000]
//	r2://<account-id>/bucket[/prefix]
//
// Without an endpoint, s3:// uses AWS (https://s3.<region>.amazonaws.com).
func ParseMedia(v string) (*Bucket, error) {
	u, err := url.Parse(v)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("[proof] media %q: want s3://bucket[/prefix] or r2://<account>/bucket[/prefix]", v)
	}
	b := &Bucket{URL: v, Region: u.Query().Get("region")}
	rest := strings.Trim(u.Path, "/")
	switch u.Scheme {
	case "s3":
		b.Name = u.Host
		if b.Region == "" {
			b.Region = "us-east-1"
		}
		b.Endpoint = u.Query().Get("endpoint")
		if b.Endpoint == "" {
			b.Endpoint = "https://s3." + b.Region + ".amazonaws.com"
		}
	case "r2":
		name, prefix, _ := strings.Cut(rest, "/")
		if name == "" {
			return nil, fmt.Errorf("[proof] media %q: r2:// needs r2://<account-id>/<bucket>", v)
		}
		b.Name, rest = name, prefix
		b.Endpoint = "https://" + u.Host + ".r2.cloudflarestorage.com"
		if b.Region == "" {
			b.Region = "auto"
		}
	default:
		return nil, fmt.Errorf("[proof] media %q: scheme %q is not a bucket (want s3 or r2)", v, u.Scheme)
	}
	if e, err := url.Parse(b.Endpoint); err != nil || (e.Scheme != "https" && e.Scheme != "http") || e.Host == "" {
		return nil, fmt.Errorf("[proof] media %q: endpoint %q is not an http(s) URL", v, b.Endpoint)
	}
	b.Endpoint = strings.TrimRight(b.Endpoint, "/")
	b.Prefix = rest
	return b, nil
}

// Key is the object key of an artifact: content-addressed under the prefix,
// grouped by repository and story so purge and lifecycle rules can find it.
func (b *Bucket) Key(repo string, story int, sha string) string {
	return path.Join(b.Prefix, "proof", repo, fmt.Sprint(story), sha)
}

// ErrNoObject means the bucket has no such object.
var ErrNoObject = errors.New("no such object in the media bucket")

// Put uploads an object.
func (b *Bucket) Put(ctx context.Context, key string, data []byte) error {
	_, err := b.do(ctx, http.MethodPut, key, nil, data, nil)
	return err
}

// Get downloads an object.
func (b *Bucket) Get(ctx context.Context, key string) ([]byte, error) {
	return b.do(ctx, http.MethodGet, key, nil, nil, nil)
}

// Delete removes an object; a missing object is not an error.
func (b *Bucket) Delete(ctx context.Context, key string) error {
	_, err := b.do(ctx, http.MethodDelete, key, nil, nil, nil)
	if errors.Is(err, ErrNoObject) {
		return nil
	}
	return err
}

// LifecycleRuleID prefixes the ID of the expiration rule aayudha keeps on the
// bucket for each repository (<id>/<owner>/<name>).
const LifecycleRuleID = "aayudha-proof-retention"

type lifecycle struct {
	XMLName xml.Name        `xml:"LifecycleConfiguration"`
	NS      string          `xml:"xmlns,attr,omitempty"`
	Rules   []lifecycleRule `xml:"Rule"`
}

type lifecycleRule struct {
	ID    string `xml:"ID"`
	Inner []byte `xml:",innerxml"`
}

// EnsureLifecycle makes the bucket expire this repository's proof objects
// after days (the [proof] retention_days TTL). Other rules on the bucket are
// kept: S3 replaces the whole configuration on every write. days 0 removes
// aayudha's rule (keep forever).
func (b *Bucket) EnsureLifecycle(ctx context.Context, repo string, days int) (changed bool, err error) {
	var cur lifecycle
	raw, err := b.do(ctx, http.MethodGet, "", url.Values{"lifecycle": {""}}, nil, nil)
	switch {
	case errors.Is(err, ErrNoObject):
	case err != nil:
		return false, err
	default:
		if err := xml.Unmarshal(raw, &cur); err != nil {
			return false, fmt.Errorf("bucket lifecycle: %w", err)
		}
	}
	prefix := path.Join(b.Prefix, "proof", repo) + "/"
	id := LifecycleRuleID + "/" + repo
	want := fmt.Sprintf("<ID>%s</ID><Filter><Prefix>%s</Prefix></Filter><Status>Enabled</Status><Expiration><Days>%d</Days></Expiration>", xmlEscape(id), xmlEscape(prefix), days)
	var rules []string
	found := false
	for _, r := range cur.Rules {
		if r.ID == id {
			found = true
			if days > 0 && normalizeXML(string(r.Inner)) == normalizeXML(want) {
				return false, nil
			}
			continue
		}
		rules = append(rules, string(r.Inner))
	}
	if days == 0 && !found {
		return false, nil
	}
	if days > 0 {
		rules = append(rules, want)
	}
	var body bytes.Buffer
	body.WriteString(`<?xml version="1.0" encoding="UTF-8"?><LifecycleConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/">`)
	for _, r := range rules {
		body.WriteString("<Rule>" + r + "</Rule>")
	}
	body.WriteString("</LifecycleConfiguration>")
	if len(rules) == 0 {
		_, err = b.do(ctx, http.MethodDelete, "", url.Values{"lifecycle": {""}}, nil, nil)
		return err == nil, err
	}
	// S3 requires an integrity checksum on lifecycle writes: SHA-256 through
	// the flexible-checksum headers rather than Content-MD5.
	sum := sha256.Sum256(body.Bytes())
	_, err = b.do(ctx, http.MethodPut, "", url.Values{"lifecycle": {""}}, body.Bytes(), map[string]string{
		"X-Amz-Sdk-Checksum-Algorithm": "SHA256", "X-Amz-Checksum-Sha256": base64.StdEncoding.EncodeToString(sum[:]), "Content-Type": "application/xml"})
	return err == nil, err
}

// Lifecycle returns the bucket's lifecycle rule for repo ("" when none).
func (b *Bucket) Lifecycle(ctx context.Context, repo string) (string, error) {
	raw, err := b.do(ctx, http.MethodGet, "", url.Values{"lifecycle": {""}}, nil, nil)
	if errors.Is(err, ErrNoObject) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var cur lifecycle
	if err := xml.Unmarshal(raw, &cur); err != nil {
		return "", err
	}
	for _, r := range cur.Rules {
		if r.ID == LifecycleRuleID+"/"+repo {
			return string(r.Inner), nil
		}
	}
	return "", nil
}

func xmlEscape(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func normalizeXML(s string) string { return strings.Join(strings.Fields(s), "") }

func (b *Bucket) do(ctx context.Context, method, key string, q url.Values, body []byte, hdr map[string]string) ([]byte, error) {
	p := "/" + b.Name
	if key != "" {
		p += "/" + key
	}
	u := b.Endpoint + sigv4.EscapePath(p)
	if len(q) > 0 {
		u += "?" + sigv4.CanonicalQuery(q)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.ContentLength = int64(len(body))
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	now := time.Now
	if b.Now != nil {
		now = b.Now
	}
	sigv4.Sign(req, body, b.AccessKey, b.SecretKey, b.Region, "s3", now())
	c := b.HTTP
	if c == nil {
		c = &http.Client{Timeout: 5 * time.Minute}
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("media bucket %s %s: %w", method, b.Name, err)
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
		return nil, fmt.Errorf("media bucket %s %s/%s: HTTP %d: %s", method, b.Name, key, resp.StatusCode, msg)
	}
	return out, nil
}
