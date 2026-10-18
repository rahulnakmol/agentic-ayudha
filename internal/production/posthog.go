package production

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
)

// PostHogAPI is the read the PostHog source needs: one query node run on the
// project's query endpoint (posthog.Client).
type PostHogAPI interface {
	Query(ctx context.Context, name string, node map[string]any) (body []byte, request string, err error)
}

// PostHog answers investigate's logs and exceptions queries from one PostHog
// project, live and read-only (Decision #449, phase P1), with the export's
// columns. PostHog's error tracking groups exceptions by issue, not by
// release: its exceptions rows leave version empty, refuse a filter or a
// grouping on it, and give each issue's first and last seen over its whole
// life, which the provenance says.
type PostHog struct {
	API     PostHogAPI
	Project string
}

// logLevels are PostHog's log severities, in its order.
var logLevels = []string{"trace", "debug", "info", "warn", "error", "fatal"}

// Run evaluates the query against the PostHog project.
func (p PostHog) Run(ctx context.Context, q Query) (Result, Provenance, error) {
	pv := Provenance{Source: "posthog project " + p.Project}
	if err := q.Check(); err != nil {
		return Result{}, pv, err
	}
	dates := map[string]any{"date_from": q.From.UTC().Format(time.RFC3339), "date_to": q.To.UTC().Format(time.RFC3339), "explicitDate": true}
	var node map[string]any
	switch q.Kind {
	case "logs":
		for k := range q.Where {
			if k != "service" && k != "level" {
				return Result{}, pv, fmt.Errorf("%w: where %s is not a filter aayudha sends to PostHog logs (service, level)", errSource, k)
			}
		}
		levels := logLevels
		if lv := strings.ToLower(q.Where["level"]); lv != "" {
			if !slices.Contains(logLevels, lv) {
				return Result{}, pv, fmt.Errorf("%w: level %q: PostHog logs know %s", errSource, q.Where["level"], strings.Join(logLevels, ", "))
			}
			levels = []string{lv}
		}
		node = map[string]any{"kind": "LogsQuery", "dateRange": dates, "severityLevels": levels, "serviceNames": []string{q.Where["service"]},
			"filterGroup": map[string]any{"type": "AND", "values": []any{}}, "orderBy": "latest", "limit": q.Limit + 1}
	case "exceptions":
		if len(q.By) > 1 || (len(q.By) == 1 && q.By[0] != "version") || len(q.Where) > 1 {
			return Result{}, pv, fmt.Errorf("%w: PostHog error tracking groups by issue, not by release or route; ask exceptions where service=<name> only", errSource)
		}
		// PostHog error tracking has no service field to filter on, so the
		// rows are the whole project's; the provenance says so rather than
		// passing them off as the service's.
		pv.Source += fmt.Sprintf(" (error tracking: project-wide, not scoped to service %s, since PostHog error tracking has no service filter; first and last are each issue's first and last seen, over its whole life)", q.Where["service"])
		node = map[string]any{"kind": "ErrorTrackingQuery", "dateRange": dates, "orderBy": "occurrences", "orderDirection": "DESC",
			"status": "active", "volumeResolution": 1, "withAggregations": true, "limit": q.Limit + 1}
	default:
		return Result{}, pv, fmt.Errorf("%w: %s is not answered from PostHog; it answers logs and exceptions", errSource, q.Kind)
	}
	b, req, err := p.API.Query(ctx, "aayudha investigate", node)
	pv.Requests = append(pv.Requests, req)
	if err != nil {
		return Result{}, pv, err
	}
	sum := sha256.Sum256([]byte(req + "\n" + string(b) + "\n"))
	pv.Digest = hex.EncodeToString(sum[:])
	if q.Kind == "logs" {
		r, err := posthogLogs(b)
		return limit(r, q.Limit), pv, err
	}
	r, err := posthogExceptions(b)
	return limit(r, q.Limit), pv, err
}

func posthogLogs(b []byte) (Result, error) {
	var body struct {
		Results []struct {
			Timestamp string            `json:"timestamp"`
			Severity  string            `json:"severity_text"`
			Body      string            `json:"body"`
			TraceID   string            `json:"trace_id"`
			Resource  map[string]string `json:"resource_attributes"`
		} `json:"results"`
	}
	if err := json.Unmarshal(b, &body); err != nil {
		return Result{}, fmt.Errorf("%w: undecodable logs answer: %w", errSource, err)
	}
	r := Result{Columns: []string{"time", "level", "msg", "version", "trace_id", "attributes"}}
	for _, l := range body.Results {
		r.Rows = append(r.Rows, []string{whenNano(l.Timestamp), l.Severity, clip(redact(l.Body), 200), l.Resource["service.version"], strings.ToLower(l.TraceID), ""})
	}
	return r, nil
}

func posthogExceptions(b []byte) (Result, error) {
	var body struct {
		Results []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			FirstSeen   string `json:"first_seen"`
			LastSeen    string `json:"last_seen"`
			Agg         struct {
				Occurrences float64 `json:"occurrences"`
			} `json:"aggregations"`
		} `json:"results"`
	}
	if err := json.Unmarshal(b, &body); err != nil {
		return Result{}, fmt.Errorf("%w: undecodable error tracking answer: %w", errSource, err)
	}
	r := Result{Columns: []string{"version", "type", "message", "top_frame", "count", "first", "last"}}
	for _, x := range body.Results {
		r.Rows = append(r.Rows, []string{"", clip(redact(x.Name), 120), clip(redact(x.Description), 160), "", cell(x.Agg.Occurrences), when(x.FirstSeen), when(x.LastSeen)})
	}
	return r, nil
}
