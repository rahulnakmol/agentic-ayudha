package cli

import (
	"encoding/json"
	"math"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/experiment"
	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/fixture"
	"github.com/rahulnakmol/agentic-ayudha/internal/secrets"
	"github.com/rahulnakmol/agentic-ayudha/internal/status"
)

// The PostHog and GrowthBook exchanges below are synthetic: written from each
// provider's API reference (PostHog HogQL query API; GrowthBook REST
// GET /v1/experiments and /v1/experiments/{id}/results) because no account
// exists yet. TestLive-style live checks need keys (founder action).
const synthetic = "Synthetic: written from the provider's API reference; no account exists yet"

func posthogScope() fixture.Interaction {
	return fixture.Interaction{Method: "GET", URL: "https://us.posthog.com/api/personal_api_keys/@current/", Status: 200,
		Response: []byte(`{"label":"aayudha","scopes":["query:read"]}`)}
}

func posthogQuery(t *testing.T, name, hogql, results string) fixture.Interaction {
	body, err := json.Marshal(map[string]any{"name": name, "query": map[string]string{"kind": "HogQLQuery", "query": hogql}})
	if err != nil {
		t.Fatal(err)
	}
	return fixture.Interaction{Method: "POST", URL: "https://us.posthog.com/api/projects/12345/query/", Body: body, Status: 200,
		Response: []byte(`{"results":` + results + `,"hasMore":false}`)}
}

const armsHogQL = "SELECT properties.`$feature/checkout-v2` AS variant, count(DISTINCT person_id) AS users, count(DISTINCT if(event = 'signed_up', person_id, NULL)) AS converted" +
	" FROM events WHERE timestamp >= toDateTime('2026-10-12 09:00:00', 'UTC') AND timestamp < toDateTime('2026-11-09 09:30:00', 'UTC')" +
	" AND properties.`$feature/checkout-v2` IS NOT NULL GROUP BY variant ORDER BY variant"

// experimentBody is an Experiment as `aayu experiment` writes it, typed out by
// hand: 3841 per arm is the hand-worked size for 0.10 → 0.12 at alpha 0.05,
// power 0.8 (see internal/experiment's TestMinSampleHandComputed).
func experimentBody(minSample string) string {
	return "### Hypothesis\n\nA one-page checkout lifts sign-ups.\n\n### Metric\n\nsigned_up\n\n### Baseline\n\nConversion rate 0.1.\n\n" +
		"### Flag\n\ncheckout-v2\n\n### Minimum sample\n\n" + minSample + "\n"
}

const minSample3841 = "3841 per arm; two-sided two-proportion z-test, alpha 0.05, power 0.8, baseline rate 0.1, detectable rate 0.12"

type experimentRig struct {
	g    *fake.GitHub
	env  map[string]string
	opts func(now time.Time, rec ...fixture.Interaction) (Options, *fixture.Replayer)
}

func newExperimentRig(t *testing.T) experimentRig {
	g := fake.New(t, "o/r", "User")
	env := map[string]string{"AAYU_GITHUB_TOKEN": "t", "AAYU_SECRET_POSTHOG": "phx_test", "AAYU_SECRET_GROWTHBOOK": "secret_readonly_test"}
	return experimentRig{g: g, env: env, opts: func(now time.Time, rec ...fixture.Interaction) (Options, *fixture.Replayer) {
		r := &fixture.Replayer{T: t, Rec: &fixture.Recording{Source: synthetic, Interactions: rec}}
		return Options{GitHubAPI: g.URL(), GitHubGap: -1, Now: func() time.Time { return now }, HTTP: &http.Client{Transport: local{r}},
			Secrets: secrets.New(secrets.Env{Getenv: func(k string) string { return env[k] }})}, r
	}}
}

var integrationFlags = []string{"--repo", "o/r", "--json",
	"--set", "integrations.posthog.secret=posthog", "--set", "integrations.posthog.scope=read", "--set", "integrations.posthog.project=12345",
	"--set", "integrations.growthbook.secret=growthbook", "--set", "integrations.growthbook.scope=read"}

// WP6.7 accept: `experiment` sets the minimum sample from a reproducible
// baseline; `readout` computes the fixture's lift, writes the Result, and the
// Feature becomes closable under the parent-close rule.
func TestExperimentReadoutLift(t *testing.T) {
	rig := newExperimentRig(t)
	feature := rig.g.AddIssue("One-page checkout", "### Summary\n\nCheckout in one page.", "type:feature", "status:in-progress")
	rig.g.AddIssue("Ship the page", "", "type:story", "status:released").Parent = feature.Number
	feature.Subs = []int{2}

	window := "timestamp >= toDateTime('2026-09-14 00:00:00', 'UTC') AND timestamp < toDateTime('2026-10-12 00:00:00', 'UTC')"
	opts, r := rig.opts(time.Date(2026, 10, 12, 9, 30, 0, 0, time.UTC),
		posthogScope(),
		posthogQuery(t, "aayudha baseline users", "SELECT count(DISTINCT person_id) FROM events WHERE "+window+" AND (person.properties.plan = 'free')", "[[40000]]"),
		posthogScope(),
		posthogQuery(t, "aayudha baseline users", "SELECT count(DISTINCT person_id) FROM events WHERE "+window+" AND ((person.properties.plan = 'free') AND event = 'signed_up')", "[[4000]]"),
	)
	code, out, errOut := run(t, rig.env, opts, append([]string{"experiment", "1", "--hypothesis", "A one-page checkout lifts sign-ups.",
		"--metric", "signed_up", "--flag", "checkout-v2", "--mde", "20%", "--exposure", "person.properties.plan = 'free'"}, integrationFlags...)...)
	r.Done()
	if code != 0 {
		t.Fatalf("experiment: exit %d: %s", code, errOut)
	}
	var exp struct {
		Experiment int `json:"experiment"`
		MinSample  int `json:"min_sample_per_arm"`
	}
	_ = json.Unmarshal([]byte(out), &exp)
	// Baseline 4000/40000 = 0.10; a 20 % relative MDE detects 0.12; the hand-worked size is 3841.
	if exp.Experiment != 3 || exp.MinSample != 3841 {
		t.Fatalf("experiment: %s", out)
	}
	rig.g.Mu.Lock()
	created := rig.g.Issues[3]
	body, parent := created.Body, created.Parent
	rig.g.Mu.Unlock()
	if parent != 1 || !strings.Contains(body, "### Minimum sample\n\n"+minSample3841+"\n") || !slices.Contains(rig.g.LabelNames(3), "status:planned") {
		t.Fatalf("created Experiment: parent %d, labels %v, body:\n%s", parent, rig.g.LabelNames(3), body)
	}

	// Readout over 2026-10-12 09:00 (the Experiment's creation) to now.
	// Hand-worked lift: control 400/4000 = 0.10, test 492/4100 = 0.12, so
	// +0.02 absolute and +20 % relative; pooled 892/8100 = 0.1101235,
	// SE √(0.1101235·0.8898765·(1/4000 + 1/4100)) = 0.0069571, z = 2.8748,
	// p = erfc(2.8748/√2) = 0.00404; unpooled SE √(0.09/4000 + 0.1056/4100)
	// = 0.0069467, 95 % CI 0.02 ± 1.959964·0.0069467 = [0.00638, 0.03362].
	opts, r = rig.opts(time.Date(2026, 11, 9, 9, 30, 0, 0, time.UTC), posthogScope(),
		posthogQuery(t, "aayudha readout checkout-v2", armsHogQL, `[["control",4000,400],["test",4100,492]]`))
	code, out, errOut = run(t, rig.env, opts, append([]string{"readout", "3"}, integrationFlags...)...)
	r.Done()
	if code != 0 {
		t.Fatalf("readout: exit %d: %s", code, errOut)
	}
	var rep struct {
		Lift experiment.Lift `json:"lift"`
	}
	_ = json.Unmarshal([]byte(out), &rep)
	for _, c := range []struct {
		name           string
		got, want, tol float64
	}{
		{"absolute", rep.Lift.Absolute, 0.02, 1e-12}, {"relative", rep.Lift.Relative, 0.20, 1e-12}, {"z", rep.Lift.Z, 2.8748, 1e-4},
		{"p", rep.Lift.P, 0.00404, 1e-5}, {"ci low", rep.Lift.CILow, 0.00638, 1e-5}, {"ci high", rep.Lift.CIHigh, 0.03362, 1e-5},
	} {
		if math.Abs(c.got-c.want) > c.tol {
			t.Errorf("lift %s = %v, want %v ± %v", c.name, c.got, c.want, c.tol)
		}
	}
	rig.g.Mu.Lock()
	body = rig.g.Issues[3].Body
	rig.g.Mu.Unlock()
	for _, want := range []string{"### Result\n\nLift of test over control: +20.00 % relative (+2.00 pts), significant at alpha 0.05",
		"[+0.64 pts, +3.36 pts]", "| test | 4100 | 492 | 12.00 % |", "### Minimum sample\n\n" + minSample3841} {
		if !strings.Contains(body, want) {
			t.Errorf("Experiment body lacks %q:\n%s", want, body)
		}
	}
	if got := rig.g.LabelNames(3); !slices.Contains(got, "status:measured") || slices.Contains(got, "status:planned") {
		t.Errorf("Experiment labels after readout: %v", got)
	}

	// Parent-close rule: every child of the Feature is released or measured
	// and the Experiment holds a readout Result.
	code, out, errOut = run(t, rig.env, Options{GitHubAPI: rig.g.URL(), GitHubGap: -1}, "status", "1", "--repo", "o/r", "--json")
	var st struct {
		Roots []*status.Rollup `json:"roots"`
	}
	_ = json.Unmarshal([]byte(out), &st)
	if code != 0 || len(st.Roots) != 1 || !st.Roots[0].Closable {
		t.Errorf("status after readout: exit %d, %s %s", code, out, errOut)
	}
}

// WP6.7 accept: under-sample refused. One arm a user short of the minimum,
// and a body whose stated minimum disagrees with its own parameters, are both
// refused with exit 4 and nothing written. GrowthBook's results give a
// negative, non-significant lift.
func TestReadoutGates(t *testing.T) {
	rig := newExperimentRig(t)
	rig.g.AddIssue("Experiment: checkout-v2 on signed_up", experimentBody(minSample3841), "type:experiment", "status:in-progress")
	rig.g.AddIssue("Experiment: tampered", experimentBody(strings.Replace(minSample3841, "3841", "1000", 1)), "type:experiment", "status:in-progress")
	now := time.Date(2026, 11, 9, 9, 30, 0, 0, time.UTC)

	opts, r := rig.opts(now, posthogScope(), posthogQuery(t, "aayudha readout checkout-v2", armsHogQL, `[["control",3840,400],["test",4100,492]]`))
	code, _, errOut := run(t, rig.env, opts, append([]string{"readout", "1"}, integrationFlags...)...)
	r.Done()
	if code != ExitBlocked || !strings.Contains(errOut, "arm control has 3840 of the 3841 users per arm") {
		t.Errorf("under-sample: exit %d: %s", code, errOut)
	}
	opts, r = rig.opts(now)
	code, _, errOut = run(t, rig.env, opts, append([]string{"readout", "2"}, integrationFlags...)...)
	r.Done()
	if code != ExitBlocked || !strings.Contains(errOut, "states 1000 per arm, but its parameters give 3841") {
		t.Errorf("tampered minimum: exit %d: %s", code, errOut)
	}
	if w := rig.g.Writes(); len(w) != 0 {
		t.Fatalf("refused readouts wrote %v", w)
	}

	// GrowthBook, hand-worked: control (first variation) 600/5000 = 0.12,
	// treatment 540/5000 = 0.108: -0.012 absolute, -10 % relative; pooled
	// 0.114, SE √(0.114·0.886·2/5000) = 0.0063562, z = -1.8879, p = 0.0590,
	// not significant at 0.05.
	results := `{"experiment":{"id":"exp_19g61","trackingKey":"checkout-v2","variations":[{"variationId":"var_c","key":"control","name":"Control"},{"variationId":"var_t","key":"treatment","name":"Treatment"}]},
"result":{"id":"snp_1","experimentId":"exp_19g61","phase":"0","dateStart":"2026-10-12T09:00:00Z","dateEnd":"2026-11-09T00:00:00Z","dimension":{"type":"none"},"results":[{"dimension":"All","totalUsers":10000,"checks":{"srm":0.99},
"metrics":[{"metricId":"met_signup","variations":[{"variationId":"var_t","analyses":[{"engine":"frequentist","numerator":540,"denominator":5000,"mean":0.108,"stddev":0.31,"percentChange":-0.1,"effectStandardError":0.05,"ciLow":-0.2,"ciHigh":0.01,"pValue":0.06}]},
{"variationId":"var_c","analyses":[{"engine":"frequentist","numerator":600,"denominator":5000,"mean":0.12,"stddev":0.32,"percentChange":0,"effectStandardError":0,"ciLow":0,"ciHigh":0}]}]}]}]}}`
	opts, r = rig.opts(now,
		fixture.Interaction{Method: "GET", URL: "https://api.growthbook.io/api/v1/experiments?limit=2&trackingKey=checkout-v2", Status: 200,
			Response: []byte(`{"experiments":[{"id":"exp_19g61","trackingKey":"checkout-v2"}],"limit":2,"offset":0,"count":1,"total":1,"hasMore":false}`)},
		fixture.Interaction{Method: "GET", URL: "https://api.growthbook.io/api/v1/experiments/exp_19g61/results", Status: 200, Response: []byte(results)},
	)
	r.AuthHeader, r.AuthValue = "Authorization", "Bearer secret_readonly_test"
	code, out, errOut := run(t, rig.env, opts, append([]string{"readout", "1", "--source", "growthbook", "--metric", "met_signup"}, integrationFlags...)...)
	r.Done()
	var rep struct {
		Lift experiment.Lift `json:"lift"`
	}
	_ = json.Unmarshal([]byte(out), &rep)
	if code != 0 || rep.Lift.Control.Name != "control" || math.Abs(rep.Lift.Relative+0.10) > 1e-12 || math.Abs(rep.Lift.Z+1.8879) > 1e-4 ||
		math.Abs(rep.Lift.P-0.0590) > 1e-4 || rep.Lift.Significant {
		t.Errorf("growthbook readout: exit %d, %+v: %s", code, rep.Lift, errOut)
	}
	rig.g.Mu.Lock()
	body := rig.g.Issues[1].Body
	rig.g.Mu.Unlock()
	if !strings.Contains(body, "Lift of treatment over control: -10.00 % relative (-1.20 pts), not significant at alpha 0.05") {
		t.Errorf("Experiment body after GrowthBook readout:\n%s", body)
	}
}
