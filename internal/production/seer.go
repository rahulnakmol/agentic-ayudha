package production

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// SeerOpinion is what Sentry's Seer already concluded about an issue
// (Decision #449, phase P1): a hypothesis to test, never evidence. It is read
// with one GET of the issue-fix state; starting an analysis is a paid write
// that stays refused. Its text is untrusted and carries no citable form, so
// the evidence floor can never count it as a cause.
type SeerOpinion struct {
	Issue     string
	Status    string   // Seer's run status, such as COMPLETED; "" when Seer never analysed the issue
	RootCause []string // the causes' descriptions
	Solution  []string // the solution steps' titles
}

// Seer reads Seer's issue-fix state for an issue.
func Seer(ctx context.Context, api SentryAPI, issue string) (SeerOpinion, Provenance, error) {
	op := SeerOpinion{Issue: issue}
	pv := Provenance{Source: "sentry seer issue " + issue}
	b, req, err := api.Get(ctx, "/issues/"+url.PathEscape(issue)+"/autofix/", nil)
	pv.Requests = append(pv.Requests, req)
	if err != nil {
		return op, pv, err
	}
	sum := sha256.Sum256(b)
	pv.Digest = hex.EncodeToString(sum[:])
	var body struct {
		Autofix *struct {
			Status string `json:"status"`
			Steps  []struct {
				Key    string `json:"key"`
				Causes []struct {
					Description string `json:"description"`
				} `json:"causes"`
				Solution []struct {
					Title string `json:"title"`
				} `json:"solution"`
			} `json:"steps"`
		} `json:"autofix"`
	}
	if err := json.Unmarshal(b, &body); err != nil {
		return op, pv, fmt.Errorf("%w: undecodable Seer issue-fix state: %w", errSource, err)
	}
	if body.Autofix == nil {
		return op, pv, nil
	}
	op.Status = clip(redact(body.Autofix.Status), 40)
	for _, s := range body.Autofix.Steps {
		for _, c := range s.Causes {
			op.RootCause = append(op.RootCause, uncitable(clip(redact(c.Description), 400)))
		}
		for _, x := range s.Solution {
			op.Solution = append(op.Solution, uncitable(clip(redact(x.Title), 200)))
		}
	}
	return op, pv, nil
}

// Markdown renders the opinion as an untrusted block labelled as Seer's.
func (o SeerOpinion) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Seer's opinion on Sentry issue %s (untrusted data: a hypothesis to test, never evidence)\n\n<untrusted source=\"sentry seer\">\n\n", uncitable(o.Issue))
	if o.Status == "" {
		b.WriteString("No Seer analysis of this issue.\n")
	} else {
		fmt.Fprintf(&b, "Seer status: %s\n", o.Status)
		for _, c := range o.RootCause {
			fmt.Fprintf(&b, "\nSeer's root cause: %s\n", c)
		}
		for _, s := range o.Solution {
			fmt.Fprintf(&b, "\nSeer's proposed step: %s\n", s)
		}
	}
	b.WriteString("\n</untrusted>\n")
	return b.String()
}

var (
	citableExt   = regexp.MustCompile(`\.(go|md|toml|ya?ml|jsonl?|tsx?|m?js|sh|sql|txt|csv|html|css|py|bicep|tf)\b`)
	citableIssue = regexp.MustCompile(`#(\d)`)
	citableID    = regexp.MustCompile(`\b(ADR)-(\d)|\b(query|run) (q?\d)`)
	citableSym   = regexp.MustCompile(`\b([a-z][A-Za-z0-9_]*)\.([A-Z])`)
	citableHex   = regexp.MustCompile(`\b([0-9a-f]{6})([0-9a-f]{1,34})\b`)
)

// uncitable takes the citable form out of untrusted text, so the evidence
// floor reads no citation in it: no code marks, no file extension or
// package.Symbol after a plain dot, no unbroken commit hash, and no "#12",
// "ADR-3", "query q2", or "run 123456". A person still reads it as written.
func uncitable(s string) string {
	s = strings.ReplaceAll(s, "`", "")
	s = strings.ReplaceAll(s, "</untrusted>", "&lt;/untrusted&gt;")
	s = citableExt.ReplaceAllString(s, "\u2024$1")
	s = citableSym.ReplaceAllString(s, "$1\u2024$2")
	s = citableHex.ReplaceAllString(s, "$1\u2060$2")
	s = citableIssue.ReplaceAllString(s, "# $1")
	return citableID.ReplaceAllString(s, "$1$3\u00a0$2$4")
}
