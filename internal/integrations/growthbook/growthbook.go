// Package growthbook is the GrowthBook adapter (WP6.7, WP7.2): it finds an
// experiment by its tracking key and reads one metric's per-variation users
// and conversions from the latest results, for `aayu readout`, and the
// control rate as a baseline for `aayu measure`. It only reads;
// GrowthBook's REST API reports no key role, so the configured read scope is
// trusted (doctor says so).
package growthbook

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
)

// DefaultURL is GrowthBook Cloud's API host; self-hosted instances set
// [integrations.growthbook] url to their API_HOST.
const DefaultURL = "https://api.growthbook.io"

// Client reads one GrowthBook organization.
type Client struct {
	hc   *http.Client
	base string
	key  string
	now  func() time.Time
}

// New returns a client for a resolved [integrations.growthbook].
func New(c integrations.Conn, rt http.RoundTripper, now func() time.Time) *Client {
	return &Client{hc: integrations.Client(rt), base: c.BaseURL(DefaultURL), key: c.Key, now: now}
}

func (c *Client) get(ctx context.Context, path string, q url.Values, out any) (string, error) {
	u := c.base + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := integrations.NewRequest(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	return "GET " + u, integrations.DoJSON(c.hc, req, out)
}

// Scope checks that the key reaches the API (GET /api/v1/experiments?limit=1).
// GrowthBook does not report a secret key's role, so it is not verified.
func (c *Client) Scope(ctx context.Context) (integrations.Scope, error) {
	var res struct {
		Experiments []struct{} `json:"experiments"`
	}
	ep, err := c.get(ctx, "/api/v1/experiments", url.Values{"limit": {"1"}}, &res)
	if err != nil {
		return integrations.Scope{}, err
	}
	return integrations.Scope{Account: "GrowthBook API key", Endpoint: ep}, nil
}

// Variation is one variation's users and the metric's numerator.
type Variation struct {
	ID        string
	Key       string
	Users     int64
	Converted int64
}

var (
	trackingKey = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,200}$`)
	metricID    = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,200}$`)
)

// Results finds the experiment whose tracking key is key, reads its latest
// results, and returns each variation's users and the binomial metric's
// conversions, control (the first variation) first, with the exact requests.
func (c *Client) Results(ctx context.Context, key, metric string) ([]Variation, integrations.Window, string, error) {
	var w integrations.Window
	if !trackingKey.MatchString(key) {
		return nil, w, "", fmt.Errorf("%w: GrowthBook tracking key %q", integrations.ErrQuery, key)
	}
	if !metricID.MatchString(metric) {
		return nil, w, "", fmt.Errorf("%w: GrowthBook metric ID %q", integrations.ErrQuery, metric)
	}
	var list struct {
		Experiments []struct {
			ID          string `json:"id"`
			TrackingKey string `json:"trackingKey"`
		} `json:"experiments"`
	}
	q1, err := c.get(ctx, "/api/v1/experiments", url.Values{"trackingKey": {key}, "limit": {"2"}}, &list)
	if err != nil {
		return nil, w, "", err
	}
	if len(list.Experiments) != 1 || list.Experiments[0].TrackingKey != key {
		return nil, w, "", fmt.Errorf("%w: GrowthBook has %d experiments with tracking key %q; want exactly one", integrations.ErrProvider, len(list.Experiments), key)
	}
	id := list.Experiments[0].ID
	var res struct {
		Experiment struct {
			Variations []struct {
				VariationID string `json:"variationId"`
				Key         string `json:"key"`
			} `json:"variations"`
		} `json:"experiment"`
		Result struct {
			DateStart string `json:"dateStart"`
			DateEnd   string `json:"dateEnd"`
			Results   []struct {
				Dimension string `json:"dimension"`
				Metrics   []struct {
					MetricID   string `json:"metricId"`
					Variations []struct {
						VariationID string `json:"variationId"`
						Analyses    []struct {
							Engine      string  `json:"engine"`
							Numerator   float64 `json:"numerator"`
							Denominator float64 `json:"denominator"`
						} `json:"analyses"`
					} `json:"variations"`
				} `json:"metrics"`
			} `json:"results"`
		} `json:"result"`
	}
	q2, err := c.get(ctx, "/api/v1/experiments/"+url.PathEscape(id)+"/results", nil, &res)
	if err != nil {
		return nil, w, "", err
	}
	source := q1 + "; " + q2
	if len(res.Result.Results) != 1 {
		return nil, w, "", fmt.Errorf("%w: GrowthBook results for %s hold %d dimension rows; want the one undimensioned row", integrations.ErrProvider, key, len(res.Result.Results))
	}
	if w.From, err = time.Parse(time.RFC3339, res.Result.DateStart); err == nil {
		w.To, err = time.Parse(time.RFC3339, res.Result.DateEnd)
	}
	if err != nil {
		return nil, w, "", fmt.Errorf("%w: GrowthBook results dates %q to %q", integrations.ErrProvider, res.Result.DateStart, res.Result.DateEnd)
	}
	for _, m := range res.Result.Results[0].Metrics {
		if m.MetricID != metric {
			continue
		}
		var out []Variation
		for _, v := range res.Experiment.Variations {
			found := false
			for _, mv := range m.Variations {
				if mv.VariationID != v.VariationID || len(mv.Analyses) == 0 {
					continue
				}
				a := mv.Analyses[0]
				for _, x := range mv.Analyses {
					if x.Engine == "frequentist" {
						a = x
					}
				}
				if a.Numerator != float64(int64(a.Numerator)) || a.Denominator != float64(int64(a.Denominator)) || a.Numerator > a.Denominator {
					return nil, w, "", fmt.Errorf("%w: metric %s is not a conversion metric (numerator %v, denominator %v); readout compares conversion rates", integrations.ErrQuery, metric, a.Numerator, a.Denominator)
				}
				out = append(out, Variation{ID: v.VariationID, Key: v.Key, Users: int64(a.Denominator), Converted: int64(a.Numerator)})
				found = true
			}
			if !found {
				return nil, w, "", fmt.Errorf("%w: GrowthBook results for %s have no %s analysis for variation %s", integrations.ErrProvider, key, metric, v.VariationID)
			}
		}
		return out, w, source, nil
	}
	return nil, w, "", fmt.Errorf("%w: GrowthBook results for %s have no metric %s (%s)", integrations.ErrQuery, key, metric, strings.TrimSpace(source))
}

// Metrics this adapter answers as a baseline (WP7.2). conversion_rate is the
// control variation's rate for a conversion metric in an experiment's latest
// results; the filter is "<tracking key> <metric ID>".
var Metrics = []string{"conversion_rate"}

// Baseline reads conversion_rate. The window is the results' own analysis
// window, which GrowthBook sets; the query's window is not sent.
func (c *Client) Baseline(ctx context.Context, q integrations.Query) (integrations.Baseline, error) {
	f := strings.Fields(q.Filter)
	if q.Metric != "conversion_rate" || len(f) != 2 {
		return integrations.Baseline{}, fmt.Errorf("%w: growthbook metric %q with filter %q: want conversion_rate with the filter \"<tracking key> <metric ID>\"", integrations.ErrQuery, q.Metric, q.Filter)
	}
	vs, w, source, err := c.Results(ctx, f[0], f[1])
	if err != nil {
		return integrations.Baseline{}, err
	}
	if len(vs) == 0 || vs[0].Users == 0 {
		return integrations.Baseline{}, fmt.Errorf("%w: GrowthBook results for %s have no users in the control variation", integrations.ErrProvider, f[0])
	}
	return integrations.Baseline{
		Metric:      q.Metric,
		Population:  fmt.Sprintf("GrowthBook experiment %s, control variation %s: %d users, %d converted on %s", f[0], vs[0].Key, vs[0].Users, vs[0].Converted, f[1]),
		Window:      integrations.Window{From: w.From.UTC(), To: w.To.UTC()},
		Value:       float64(vs[0].Converted) / float64(vs[0].Users),
		SourceQuery: source,
		FetchedAt:   c.now().UTC(),
	}, nil
}
