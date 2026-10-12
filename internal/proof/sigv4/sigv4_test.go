package sigv4

import (
	"net/http"
	"testing"
	"time"
)

// AWS's published SigV4 example "GET Bucket Lifecycle" (S3 API reference,
// "Examples: Signature Calculations"): the signer is checked against a value
// that does not come from this code.
func TestSignMatchesAWSExample(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "https://examplebucket.s3.amazonaws.com/?lifecycle", nil)
	Sign(req, nil, "AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", "us-east-1", "s3", time.Date(2013, 5, 24, 0, 0, 0, 0, time.UTC))
	want := "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-content-sha256;x-amz-date, Signature=fea454ca298b7da1c68078a5d1bdbfbbe0d65c699e0f91ac7a200a0136783543"
	if got := req.Header.Get("Authorization"); got != want {
		t.Fatalf("Authorization:\n got %s\nwant %s", got, want)
	}
}
