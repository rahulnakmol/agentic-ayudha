package models

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/notify"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// Fallback price for a model with no [[models.prices]] entry and no reported
// cost: deliberately high, so an unpriced model trips the breaker early rather
// than never.
const (
	FallbackInputPerMTok  = 15.0
	FallbackOutputPerMTok = 75.0
)

// Cost prices a reply: the endpoint's own figure, else the model's price
// (PriceOf: config, then the list price), else the fallback rate. Cache reads
// and writes, which TokensIn includes, are charged at their own rates (a
// 1-hour cache write at CacheWrite1h), and a
// prompt past the long-context threshold at the long-context rates.
//
// A priority call (Billed) is charged at least at the priority rate, even when
// the endpoint reports a lower figure of its own, so a spend cap holds.
func Cost(prices []config.Price, model string, r Response) (usd float64, source string) {
	if r.CostUSD > 0 && r.ServiceTier != TierPriority {
		return r.CostUSD, CostReported
	}
	p, source, ok := PriceOf(prices, model)
	if !ok {
		p, source = ListPrice{Short: Rates{Input: FallbackInputPerMTok, Output: FallbackOutputPerMTok}}, CostFallback
	}
	rt := p.Short
	if p.LongAbove > 0 && r.TokensIn > p.LongAbove {
		rt = p.Long
	}
	if r.ServiceTier == TierPriority && p.Priority > 0 {
		x := p.Priority
		rt = Rates{Input: rt.Input * x, CachedInput: rt.CachedInput * x, CacheWrite: rt.CacheWrite * x, CacheWrite1h: rt.CacheWrite1h * x, Output: rt.Output * x}
	}
	or := func(v float64) float64 {
		if v == 0 {
			return rt.Input
		}
		return v
	}
	write1h := or(rt.CacheWrite)
	if rt.CacheWrite1h > 0 {
		write1h = rt.CacheWrite1h
	}
	plain := max(r.TokensIn-r.CachedIn-r.CacheWriteIn, 0)
	w1h := min(r.CacheWrite1hIn, r.CacheWriteIn)
	usd = (float64(plain)*rt.Input + float64(r.CachedIn)*or(rt.CachedInput) + float64(r.CacheWriteIn-w1h)*or(rt.CacheWrite) +
		float64(w1h)*write1h + float64(r.TokensOut)*rt.Output) / 1e6
	if r.CostUSD > usd {
		return r.CostUSD, CostReported
	}
	return usd, source
}

// Billed is resp as Cost prices it for req: a call that asked for priority
// serving is charged at the priority rate whatever tier its reply states, or
// when it states none, so a spend cap holds.
func Billed(req Request, resp Response) Response {
	if req.ServiceTier == TierPriority {
		resp.ServiceTier = TierPriority
	}
	return resp
}

// ErrBudget means the [budget] circuit breaker has tripped: agent lanes are
// paused and the CLI exits 6.
var ErrBudget = errors.New("budget limit reached; agent lanes are paused")

// Breaker is the [budget] circuit breaker. It trips when spend for a story
// goes over per_story_usd (the boundary build's cost_usd uses) or spend for
// the local day reaches per_day_usd (that boundary itself trips), pauses agent lanes by writing PauseFile, and pages
// once per scope per day through Notify.
type Breaker struct {
	Budget    config.Budget
	Clock     config.Clock
	Now       func() time.Time
	PauseFile string // presence pauses agent lanes until `aayu resume` (WP5.7) or removal
	Notify    *notify.Sender

	mu       sync.Mutex
	perStory map[string]float64
	perDay   map[string]float64
}

func (b *Breaker) init() {
	if b.perStory == nil {
		b.perStory, b.perDay = map[string]float64{}, map[string]float64{}
	}
}

func (b *Breaker) day(t time.Time) string { return t.In(b.Clock.Loc).Format("2006-01-02") }

// Seed loads spend from existing actions records (e.g. a fresh process).
func (b *Breaker) Seed(recs []actions.Record) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.init()
	for _, r := range recs {
		if r.CostUSD == 0 {
			continue
		}
		if r.Story != "" {
			b.perStory[r.Story] += r.CostUSD
		}
		b.perDay[b.day(r.Time)] += r.CostUSD
	}
}

// SeedLog seeds the breaker from the actions log at path (WP1.5), so the
// per-day and per-story limits count the spend of every process that logged
// to it, not only this one's (Bug #177). A missing log seeds nothing.
func (b *Breaker) SeedLog(path string) error {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	var recs []actions.Record
	for dec := json.NewDecoder(f); ; {
		var r actions.Record
		err := dec.Decode(&r)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil { // a read error (a directory, say) as well as a malformed record
			return fmt.Errorf("%s: %w", path, err)
		}
		recs = append(recs, r)
	}
	b.Seed(recs)
	return nil
}

// Paused reports whether agent lanes are paused.
func (b *Breaker) Paused() bool {
	_, err := os.Stat(b.PauseFile)
	return err == nil
}

// Check refuses a call while the breaker is tripped.
func (b *Breaker) Check(story string) error {
	if b.Paused() {
		return fmt.Errorf("%w (%s)", ErrBudget, b.PauseFile)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.init()
	// A story trips the breaker only over its limit, the same boundary build's
	// cost_usd uses, so the breaker never stops a story its build budget allows.
	if lim := b.Budget.PerStoryUSD; lim > 0 && story != "" && b.perStory[story] > lim {
		return fmt.Errorf("%w: story %s spent %.2f of %.2f USD", ErrBudget, story, b.perStory[story], lim)
	}
	if lim := b.Budget.PerDayUSD; lim > 0 && b.perDay[b.day(b.Now())] >= lim {
		return fmt.Errorf("%w: today's spend reached %.2f USD", ErrBudget, lim)
	}
	return nil
}

// Add records spend and trips the breaker when a limit is reached.
func (b *Breaker) Add(ctx context.Context, story string, usd float64) error {
	b.mu.Lock()
	b.init()
	day := b.day(b.Now())
	b.perDay[day] += usd
	var scope, msg string
	if story != "" {
		b.perStory[story] += usd
		if lim := b.Budget.PerStoryUSD; lim > 0 && b.perStory[story] > lim {
			scope, msg = "per_story:"+story, fmt.Sprintf("story %s spent %.2f USD, over [budget] per_story_usd = %.2f", story, b.perStory[story], lim)
		}
	}
	if lim := b.Budget.PerDayUSD; scope == "" && lim > 0 && b.perDay[day] >= lim {
		scope, msg = "per_day", fmt.Sprintf("spend on %s reached %.2f USD, the [budget] per_day_usd limit of %.2f", day, b.perDay[day], lim)
	}
	b.mu.Unlock()
	if scope == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(b.PauseFile), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(b.PauseFile, []byte(msg+"\n"), 0o644); err != nil {
		return err
	}
	if b.Notify != nil {
		if _, err := b.Notify.Page(ctx, notify.Page{Key: "budget:" + scope, Subject: "aayudha budget breaker tripped", Body: msg + ". Agent lanes are paused."}); err != nil {
			return errors.Join(fmt.Errorf("%w: %s", ErrBudget, msg), err)
		}
	}
	return fmt.Errorf("%w: %s", ErrBudget, msg)
}

// Gateway makes direct model calls with accounting and the breaker.
type Gateway struct {
	Models   config.Models
	Resolver Resolver
	Actions  *actions.Log
	Breaker  *Breaker
	// Key returns the value of a named secret (WP1.3 store).
	Key func(name string) (string, error)
	// Client overrides client construction (tests use the fake provider).
	Client func(route Route, key string) Client
	HTTP   *http.Client
}

// Call is one direct model call for a story on a lane.
type Call struct {
	Story string
	Seat  string
	Lane  verbs.LaneID
	Host  verbs.HostID
	Heavy bool
	Req   Request
}

// Do resolves the lane, refuses when the breaker is tripped, calls the model
// through the gateway or its direct provider, records tokens and cost in the
// actions log, and trips the breaker when the call reaches a limit.
func (g *Gateway) Do(ctx context.Context, c Call) (Response, Resolved, error) {
	// The Copilot route keeps the ui-review lane's copilot row; every other
	// call resolves its direct model.
	var m Resolved
	var err error
	if c.Lane == verbs.LaneUIReview && c.Host == verbs.HostCopilot {
		m, err = g.Resolver.Resolve(c.Lane, c.Host)
	}
	if err == nil && !CopilotRouted(m) {
		m, err = g.Resolver.ResolveDirect(c.Lane, c.Host, c.Heavy)
	}
	if err != nil {
		return Response{}, m, err
	}
	if g.Breaker != nil {
		if err := g.Breaker.Check(c.Story); err != nil {
			return Response{}, m, err
		}
	}
	route, err := RouteOf(g.Models, m)
	if err != nil {
		return Response{}, m, err
	}
	req := c.Req
	req.Model = m.Model
	if req.Effort == "" {
		req.Effort = m.Effort
	}
	var resp Response
	var usd float64
	a := actions.Action{Kind: "model.call", Target: m.Model, Story: c.Story, Seat: c.Seat, Host: string(c.Host),
		Payload: map[string]any{"lane": string(c.Lane), "family": string(m.Family), "via": route.Via, "style": route.Style, "source": m.Source}}
	_, err = g.Actions.Do(ctx, a, func(ctx context.Context) (actions.Outcome, error) {
		key, err := g.Key(route.Secret)
		if err != nil {
			return actions.Outcome{}, fmt.Errorf("model key %q: %w", route.Secret, err)
		}
		client := g.client(route, key)
		// A failed call can still have used tokens (a stream that errored after
		// message_start returns its usage so far): record and charge them.
		resp, err = client.Complete(ctx, req)
		var source string
		usd, source = Cost(g.Models.Prices, m.Model, Billed(req, resp))
		return actions.Outcome{TokensIn: resp.TokensIn, TokensOut: resp.TokensOut, CostUSD: usd, Attempts: 1,
			Result: map[string]any{"cost_source": source, "model": resp.Model}}, err
	})
	if g.Breaker != nil && !g.Actions.DryRun() && (err == nil || usd > 0) {
		if berr := g.Breaker.Add(ctx, c.Story, usd); berr != nil {
			return resp, m, errors.Join(err, berr)
		}
	}
	return resp, m, err
}

// WithModel is the gateway with lane on host pinned to model, for a call the
// binary routes to a named model of the lane (inspect's peer reviewer,
// WP4.8b). Accounting, the breaker, and the actions log are shared.
func (g *Gateway) WithModel(lane verbs.LaneID, host verbs.HostID, model string) *Gateway {
	c := *g
	lanes := map[string]map[string]config.ModelChoice{}
	for l, hosts := range g.Models.Lanes {
		lanes[l] = map[string]config.ModelChoice{}
		for h, mc := range hosts {
			lanes[l][h] = mc
		}
	}
	if lanes[string(lane)] == nil {
		lanes[string(lane)] = map[string]config.ModelChoice{}
	}
	lanes[string(lane)][string(host)] = config.ModelChoice{Model: model}
	c.Models.Lanes = lanes
	c.Resolver.Models = c.Models
	return &c
}

func (g *Gateway) client(route Route, key string) Client {
	if g.Client != nil {
		return g.Client(route, key)
	}
	if route.Via == CopilotRoute.Via {
		return CopilotCLI{Token: key, Path: os.Getenv("PATH"), Host: cmp.Or(os.Getenv("COPILOT_GH_HOST"), os.Getenv("GH_HOST"))}
	}
	return HTTPClient{BaseURL: route.BaseURL, Style: route.Style, Key: key, KeyAuth: route.KeyAuth, HTTP: g.HTTP}
}

// Fake is the fake provider for tests: it returns scripted replies in order
// (the last one repeats) and records every request.
type Fake struct {
	mu       sync.Mutex
	Replies  []Response
	Err      error
	Requests []Request
}

// Complete implements Client.
func (f *Fake) Complete(_ context.Context, r Request) (Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Requests = append(f.Requests, r)
	if f.Err != nil {
		return Response{}, f.Err
	}
	if len(f.Replies) == 0 {
		return Response{Text: "ok", Model: r.Model}, nil
	}
	reply := f.Replies[0]
	if len(f.Replies) > 1 {
		f.Replies = f.Replies[1:]
	}
	if reply.Model == "" {
		reply.Model = r.Model
	}
	return reply, nil
}
