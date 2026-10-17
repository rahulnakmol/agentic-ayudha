package models

import "github.com/rahulnakmol/agentic-ayudha/internal/config"

// Rates are USD per million tokens. CachedInput is a cache read and
// CacheWrite a prompt written to the provider's cache; either at 0 is charged
// at the Input rate.
type Rates struct {
	Input, CachedInput, CacheWrite, Output float64
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
	Source    string // the provider's official pricing page
	Read      string // the date the price was read there
}

// ListPrices are the published API prices of every model in the registry
// (verbs.Model*), standard tier, global endpoints, no batch, fast mode, or
// data-residency uplift. Reasoning and thinking tokens bill at the output
// rate on every provider; the clients count them as output (decode). The AI
// gateway the evals use returns no cost of its own (D-3.4p), so these are what
// the evals and the [budget] breaker charge; a [[models.prices]] entry in
// config overrides one, for a gateway that bills otherwise. Anthropic cache
// writes are at the 5-minute rate; the clients never ask for the 1-hour cache.
var ListPrices = []ListPrice{
	{Model: "claude-opus-5-5", Short: Rates{Input: 4, CachedInput: 0.20, CacheWrite: 5, Output: 20},
		Source: "https://platform.claude.com/docs/en/about-claude/pricing", Read: "2026-10-17"},
	{Model: "claude-sonnet-5-5", Short: Rates{Input: 2, CachedInput: 0.10, CacheWrite: 2.50, Output: 10},
		Source: "https://platform.claude.com/docs/en/about-claude/pricing", Read: "2026-10-17"},
	{Model: "claude-haiku-5-5", Short: Rates{Input: 0.10, CachedInput: 0.01, CacheWrite: 0.125, Output: 0.50},
		LongAbove: 100000, Long: Rates{Input: 0.50, CachedInput: 0.05, CacheWrite: 0.625, Output: 2.50},
		Source: "https://platform.claude.com/docs/en/about-claude/pricing", Read: "2026-10-17"},
	{Model: "claude-fable-5-1", Short: Rates{Input: 10, CachedInput: 0.25, CacheWrite: 12.50, Output: 50},
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
	{Model: "grok-4.7", Short: Rates{Input: 2, CachedInput: 0.50, Output: 6},
		LongAbove: 199999, Long: Rates{Input: 4, CachedInput: 1, Output: 12},
		Source: "https://docs.x.ai/developers/pricing", Read: "2026-10-17"},
	// The gateway's gemini-3.1-pro-low is Gemini 3.1 Pro (Google lists it as
	// gemini-3.1-pro-preview) at low thinking; the price is the model's.
	{Model: "gemini-3.1-pro-low", Short: Rates{Input: 2, CachedInput: 0.20, Output: 12},
		LongAbove: 200000, Long: Rates{Input: 4, CachedInput: 0.40, Output: 18},
		Source: "https://ai.google.dev/gemini-api/docs/pricing", Read: "2026-10-17"},
}

// PriceOf returns model's price: a [[models.prices]] entry in config first,
// then its list price. ok is false when neither names it.
func PriceOf(prices []config.Price, model string) (p ListPrice, source string, ok bool) {
	for _, c := range prices {
		if c.Model == model {
			return ListPrice{Model: model, Short: Rates{Input: c.InputPerMTok, CachedInput: c.CachedInputPerMTok,
				CacheWrite: c.CacheWritePerMTok, Output: c.OutputPerMTok}}, CostPriceTable, true
		}
	}
	for _, l := range ListPrices {
		if l.Model == model {
			return l, CostListPrice, true
		}
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
