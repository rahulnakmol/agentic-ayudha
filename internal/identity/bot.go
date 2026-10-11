package identity

import (
	"context"
	"errors"
	"fmt"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
)

// Bot kinds.
const (
	KindApp           = "app"
	KindMachineUser   = "machine-user"
	KindHumanFallback = "human-fallback"
)

// HumanFallbackTag marks writes made under the human identity because no bot
// identity exists (the WP1.4 day-one path). `audit` lists them (WP7.6).
const HumanFallbackTag = "identity:human-fallback"

// Bot is the identity agent lanes and CI write under.
type Bot struct {
	Kind  string
	Login string // the App's <slug>[bot], the machine user, or the human's login
	Token Token
	Tag   string // HumanFallbackTag on the day-one path, else empty
}

// ErrNoBotInCI means CI has no App installation token. GITHUB_TOKEN is never a
// bot identity: it cannot be a ledger bypass actor on a personal repo, and its
// writes are indistinguishable from any other workflow's (Appendix B D1).
var ErrNoBotInCI = errors.New("CI needs the bot App's installation token in AAYU_BOT_TOKEN (minted with actions/create-github-app-token); GITHUB_TOKEN is never used for bot or ledger writes")

// Minter mints a short-lived App installation token for a laptop (the broker).
type Minter interface {
	Mint(ctx context.Context) (Token, error)
}

// Configured reports whether the repo has a bot identity, and which kind.
func Configured(id config.Identity) string {
	switch {
	case id.AppID != 0:
		return KindApp
	case id.MachineUser != "":
		return KindMachineUser
	}
	return ""
}

// Bot resolves the bot identity.
//
//   - In CI (--ci): AAYU_BOT_TOKEN, an App installation token from
//     actions/create-github-app-token. A token equal to GITHUB_TOKEN is refused.
//   - On a laptop with an App: AAYU_BOT_TOKEN if set, else a token minted by the
//     broker workflow; the App private key never leaves the repo secrets.
//   - With a machine user: its token from the secrets store.
//   - Otherwise the human token, tagged identity:human-fallback (day-one path).
func (r Resolver) Bot(ctx context.Context, id config.Identity, broker Minter, humanLogin string) (Bot, error) {
	kind := Configured(id)
	login := id.AppSlug + "[bot]"
	if v := r.Getenv("AAYU_BOT_TOKEN"); v != "" && (r.CI || kind == KindApp) {
		if gt := r.Getenv("GITHUB_TOKEN"); gt != "" && gt == v {
			return Bot{}, ErrNoBotInCI
		}
		if kind != KindApp {
			login = ""
		}
		return Bot{Kind: KindApp, Login: login, Token: Token{v, "AAYU_BOT_TOKEN"}}, nil
	}
	if r.CI {
		return Bot{}, ErrNoBotInCI
	}
	switch kind {
	case KindApp:
		if broker == nil {
			return Bot{}, errors.New("bot App configured but no token broker; run `aayu init --gates` to install it")
		}
		t, err := broker.Mint(ctx)
		if err != nil {
			return Bot{}, fmt.Errorf("token broker: %w", err)
		}
		return Bot{Kind: KindApp, Login: login, Token: t}, nil
	case KindMachineUser:
		if r.Secrets == nil {
			return Bot{}, errors.New("no secrets store")
		}
		v, backend, err := r.Secrets.Get(id.MachineUserSecret)
		if err != nil {
			return Bot{}, fmt.Errorf("machine user %s: %w", id.MachineUser, err)
		}
		return Bot{Kind: KindMachineUser, Login: id.MachineUser, Token: Token{v, "secrets store (" + backend + ")"}}, nil
	}
	h, err := r.Human(ctx)
	if err != nil {
		return Bot{}, err
	}
	return Bot{Kind: KindHumanFallback, Login: humanLogin, Token: h, Tag: HumanFallbackTag}, nil
}
