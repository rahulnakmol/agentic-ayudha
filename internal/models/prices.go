package models

import (
	"slices"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
)

// Rates are USD per million tokens. CachedInput is a cache read and
// CacheWrite a prompt written to the provider's cache; either at 0 is charged
// at the Input rate. CacheWrite1h is a write to Anthropic's 1-hour cache (2x
// input, where CacheWrite is the 5-minute write at 1.25x); at 0 it is charged
// at CacheWrite.
type Rates struct {
	Input, CachedInput, CacheWrite, CacheWrite1h, Output float64
}

// ListPrice is a model's price as its provider publishes it, with where and
// when it was read. A prompt of more than LongAbove input tokens (cache reads
// and writes included) is charged at Long for the whole call, as every
// provider below bills it; LongAbove 0 means one rate at every length.
type ListPrice struct {
	Model     string
	Short     Rates
	LongAbove int
	Long      Rates
	// Priority multiplies every rate for a call served at TierPriority (fast
	// serving); 0 when the provider publishes no priority price here.
	Priority float64
	Source   string // the provider's official pricing page
	Read     string // the date the price was read there
}

// ListPrices are the published API prices of every model in the registry
// (verbs.Model*), standard tier, global endpoints, no batch, fast mode, or
// data-residency uplift; Priority is a model's fast-serving (service_tier
// priority) multiplier, where the provider publishes one. Reasoning and
// thinking tokens bill at the output rate on every provider; the clients count
// them as output (decode). The AI gateway the evals use returns no cost of its
// own (D-3.4p), so these are what the evals and the [budget] breaker charge;
// a [[models.prices]] entry in config overrides one, for a gateway that bills
// otherwise. Anthropic bills a cache write by its TTL: 5-minute writes at
// CacheWrite (1.25x input), 1-hour writes at CacheWrite1h (2x input). The AI
// gateway upgrades Claude cache writes to the 1-hour TTL, and the usage block
// says which tokens went to which.
var ListPrices = []ListPrice{
	{Model: "claude-opus-5-5", Short: Rates{Input: 4, CachedInput: 0.20, CacheWrite: 5, CacheWrite1h: 8, Output: 20},
		Source: "https://platform.claude.com/docs/en/about-claude/pricing", Read: "2026-10-17"},
	{Model: "claude-sonnet-5-5", Short: Rates{Input: 2, CachedInput: 0.10, CacheWrite: 2.50, CacheWrite1h: 4, Output: 10},
		Source: "https://platform.claude.com/docs/en/about-claude/pricing", Read: "2026-10-17"},
	{Model: "claude-haiku-5-5", Short: Rates{Input: 0.10, CachedInput: 0.01, CacheWrite: 0.125, CacheWrite1h: 0.20, Output: 0.50},
		LongAbove: 100000, Long: Rates{Input: 0.50, CachedInput: 0.05, CacheWrite: 0.625, CacheWrite1h: 1, Output: 2.50},
		Source: "https://platform.claude.com/docs/en/about-claude/pricing", Read: "2026-10-17"},
	{Model: "claude-fable-5-1", Short: Rates{Input: 10, CachedInput: 0.25, CacheWrite: 12.50, CacheWrite1h: 20, Output: 50},
		Source: "https://platform.claude.com/docs/en/about-claude/pricing", Read: "2026-10-17"},
	// OpenAI writes a prompt to its cache implicitly (GPT-5.6 and later) and bills
	// the write at 1.25x input: https://developers.openai.com/api/docs/guides/prompt-caching
	{Model: "gpt-6.1-sol", Short: Rates{Input: 2, CachedInput: 0.10, CacheWrite: 2.50, Output: 10},
		LongAbove: 272000, Long: Rates{Input: 4, CachedInput: 0.20, CacheWrite: 5, Output: 15},
		Source: "https://developers.openai.com/api/docs/pricing", Read: "2026-10-17"},
	{Model: "gpt-6-astra", Short: Rates{Input: 10, CachedInput: 1, CacheWrite: 12.50, Output: 50},
		LongAbove: 272000, Long: Rates{Input: 20, CachedInput: 2, CacheWrite: 25, Output: 75},
		Source: "https://developers.openai.com/api/docs/pricing", Read: "2026-10-17"},
	// xAI's long-context rate starts when the prompt reaches 200k tokens.
	// Priority Processing (service_tier priority) bills 2x every token type,
	// after the cache discount, and only when the reply confirms priority:
	// https://docs.x.ai/developers/advanced-api-usage/priority-processing
	// (read 2026-10-18). inspect charges it whenever it asked, so the cap holds.
	{Model: "grok-4.7", Short: Rates{Input: 2, CachedInput: 0.50, Output: 6},
		LongAbove: 199999, Long: Rates{Input: 4, CachedInput: 1, Output: 12}, Priority: 2,
		Source: "https://docs.x.ai/developers/pricing", Read: "2026-10-17"},
	// Google's Gemini rates below are the standard tier, one rate at every
	// prompt length, and double on the date named; update them then.
	// gemini-3.8-flash: the ui-review lane's interim model (D-6.6b); this rate
	// runs through 2026-12-31 (from 2027-01-01: input 1.50, cached 0.15,
	// output 7.50).
	{Model: "gemini-3.8-flash", Short: Rates{Input: 0.75, CachedInput: 0.075, Output: 3.75},
		Source: "https://ai.google.dev/gemini-api/docs/pricing", Read: "2026-10-17"},
	// gemini-4-argon-high, the lane's target: the gateway's name for Gemini 4
	// Argon (Google's gemini-4-argon) at its high reasoning tier, the same model
	// at the same price (thinking bills as output). This rate runs through
	// 2026-11-30 (from 2026-12-01: input 4, cached 0.20, output 20).
	{Model: "gemini-4-argon-high", Short: Rates{Input: 2, CachedInput: 0.10, Output: 10},
		Source: "https://ai.google.dev/gemini-api/docs/pricing", Read: "2026-10-17"},
	// gemini-4-argon: Copilot's ID for Gemini 4 Argon, on the Copilot route
	// (the ui-review lane's default where Copilot is set up), billed in AI
	// credits at these per-token rates (promotional until 2026-12-01). The route charges the credits Copilot reports; this rate is
	// for a call whose usage file is missing.
	{Model: "gemini-4-argon", Short: Rates{Input: 2, CachedInput: 0.10, Output: 10},
		Source: "https://docs.github.com/en/copilot/reference/copilot-billing/models-and-pricing", Read: "2026-10-18"},
}

// PriceOf returns model's price: a [[models.prices]] entry in config first,
// then its list price. ok is false when neither names it.
func PriceOf(prices []config.Price, model string) (p ListPrice, source string, ok bool) {
	i := slices.IndexFunc(ListPrices, func(l ListPrice) bool { return l.Model == model })
	for _, c := range prices {
		if c.Model == model {
			p := ListPrice{Model: model, Short: Rates{Input: c.InputPerMTok, CachedInput: c.CachedInputPerMTok,
				CacheWrite: c.CacheWritePerMTok, Output: c.OutputPerMTok}}
			if i >= 0 {
				p.Priority = ListPrices[i].Priority // the provider's priority multiplier applies to an overridden rate too
			}
			return p, CostPriceTable, true
		}
	}
	if i >= 0 {
		return ListPrices[i], CostListPrice, true
	}
	return ListPrice{}, "", false
}

// Where a call's cost came from (Cost).
const (
	CostReported   = "reported"      // the endpoint's own figure
	CostPriceTable = "price-table"   // [[models.prices]] in config
	CostListPrice  = "list-price"    // ListPrices
	CostFallback   = "fallback-rate" // no price: FallbackInputPerMTok and FallbackOutputPerMTok
)
