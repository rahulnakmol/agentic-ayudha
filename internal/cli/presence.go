package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
)

// Access is the repository permission a human-only action needs from the
// person who approves it.
type Access string

// Access levels a human-only action may need.
const (
	AccessAny   Access = ""      // any person: the action writes nothing to the repository
	AccessWrite Access = "write" // write, maintain, or admin
	AccessAdmin Access = "admin" // admin: bypasses, rulesets, hooks, and protected files
)

// DeviceClientVariables are the repository variables that name the client ID
// for the presence check, in order: an OAuth App made for it, then the
// aayudha GitHub App. A client ID is public; neither is a secret.
var DeviceClientVariables = []string{"AAYUDHA_DEVICE_CLIENT_ID", "AAYUDHA_APP_CLIENT_ID"}

// Approval is a person's approval of a human-only action through GitHub's
// device flow (presence check).
type Approval struct {
	Login      string `json:"login"`
	Permission string `json:"permission"` // the approver's repository permission
	Via        string `json:"via"`        // device-flow
}

// covers reports whether the approval already allows an action that needs need.
func (a *Approval) covers(need Access) bool {
	if a == nil {
		return false
	}
	switch need {
	case AccessAdmin:
		return a.Permission == "admin"
	case AccessWrite:
		return a.Permission == "admin" || a.Permission == "write"
	}
	return true
}

// requireHuman guards a human-only action (§0.1, Bug #170). A person at an
// interactive terminal, with no agent marker, passes. Otherwise the process
// may be an agent holding the person's token (an agent's shell, or a person's
// shell that inherits an agent's marker, such as an orb's Terminal tab), and
// neither the environment nor a terminal can tell them apart: aayu asks the
// person to approve the action through GitHub's device flow, on any device,
// with their own GitHub sign-in (presenceCheck). In hosts that forward shell
// commands, the pre-tool hook stops most agent calls first
// (hosts.HumanOnlyCommand); this check holds for whatever reaches the binary.
func (e *Env) requireHuman(ctx context.Context, what string, need Access) error {
	sig := e.agentSignal()
	if sig == "" && e.isTerminal(e.In) {
		return nil
	}
	refusal := Errorf(ExitBlocked, "%s is a person's decision and needs an interactive terminal: with neither a terminal nor an agent marker, aayu cannot tell a person from an agent using the person's token", what)
	if sig != "" {
		refusal = Errorf(ExitBlocked, "%s is a person's decision and is refused for an agent lane (%s); ask a person to run it in their own terminal", what, sig)
	}
	_, err := e.presenceCheck(ctx, what, need, refusal)
	return err
}

// presenceCheck proves that a person approved what, through GitHub's OAuth
// device flow: aayu prints a one-time code and https://github.com/login/device,
// and the person enters it on any device, signed in to GitHub with their own
// password and second factor. The approver must be a person (not a bot or a
// configured bot login), the same account as the token aayu acts with, and
// hold the repository permission need asks for. The device-flow token proves
// who approved and nothing else: it asks for no scopes, reads only /user, and
// is never stored. aayu offers an agent no way to complete the flow: there is
// no flag, variable, or file that supplies an approval.
//
// It returns refusal unchanged when no approval can be asked for: aayu's own
// agent lanes (AAYU_AGENT=1: the MCP server and loops, which run unattended),
// no repository or token to check the approver against (unless need is
// AccessAny), or no client ID ([identity] device_client_id, else
// DeviceClientVariables). An approval covers later human-only steps of the
// same command at the same or a lower permission.
func (e *Env) presenceCheck(ctx context.Context, what string, need Access, refusal error) (*Approval, error) {
	if e.approval.covers(need) {
		return e.approval, nil
	}
	if e.Getenv("AAYU_AGENT") == "1" {
		return nil, refusal
	}
	l, err := e.Config()
	if err != nil {
		return nil, err
	}
	// The account aayu acts as, and the repository the approver needs access
	// to. An action that needs no repository access (AccessAny: auth
	// set-secret, self-update) can be approved without them, so a fresh
	// runner can provision its first token.
	var c *github.Client
	var me *identity.User
	repo, rerr := e.RepoName(ctx)
	if rerr == nil {
		if hc, tok, err := e.HumanGitHub(ctx); err == nil {
			if u, err := identity.WhoAmI(ctx, e.HTTP, e.githubAPI(), tok); err == nil {
				c, me = hc, &u
			}
		}
	}
	if need != AccessAny && c == nil {
		return nil, refusal
	}
	owner, name, _ := github.SplitRepo(repo)
	client := l.Config.Identity.DeviceClientID
	for _, v := range DeviceClientVariables {
		if client != "" || c == nil {
			break
		}
		if variable, _, err := c.REST.Actions.GetRepoVariable(ctx, owner, name, v); err == nil {
			client = variable.Value
		}
	}
	if client == "" {
		return nil, WithNext(refusal, fmt.Sprintf("to approve it as yourself from any device, set the repository variable %s (or [identity] device_client_id) to the client ID of a GitHub OAuth App (or of aayudha's GitHub App) with Device Flow enabled", DeviceClientVariables[0]))
	}
	on := ""
	if c != nil {
		on = " on " + repo
	}

	code, err := e.deviceCode(ctx, client)
	if err != nil {
		return nil, Wrap(ExitExternal, fmt.Errorf("%s: the presence check could not start: %w", what, err))
	}
	fmt.Fprintf(e.Err, "%s%s needs a person's approval.\n  On any device, open %s and enter %s (expires in %d minutes).\n  Approve only if you started this command yourself. Waiting...\n",
		what, on, code.VerificationURI, code.UserCode, max(code.ExpiresIn/60, 1))
	devTok, err := e.deviceToken(ctx, client, code)
	if err != nil {
		return nil, Errorf(ExitBlocked, "%s%s was not approved: %v; nothing was done", what, on, err)
	}
	who, err := identity.WhoAmI(ctx, e.HTTP, e.githubAPI(), identity.Token{Value: devTok, Source: "device flow"}) // its only use; never stored
	if err != nil {
		return nil, Wrap(ExitExternal, fmt.Errorf("%s: reading the approving account: %w", what, err))
	}
	switch {
	case identity.IsBot(identity.Account{Login: who.Login, Type: who.Type}, l.Config.Identity.BotLogins):
		return nil, Errorf(ExitBlocked, "%s: approved by %s, a bot identity; only a person may approve it", what, who.Login)
	case me != nil && !strings.EqualFold(who.Login, me.Login):
		return nil, Errorf(ExitBlocked, "%s: approved by @%s, but aayu acts as @%s here; the person whose account acts must approve it", what, who.Login, me.Login)
	}
	a := &Approval{Login: who.Login, Via: "device-flow"}
	if c != nil {
		perm, _, err := c.REST.Repositories.GetPermissionLevel(ctx, owner, name, who.Login)
		if err != nil {
			return nil, Wrap(ExitExternal, err)
		}
		a.Permission = permissionOf(perm)
	}
	if !a.covers(need) {
		return nil, Errorf(ExitBlocked, "%s%s needs %s access; @%s has %s", what, on, need, a.Login, a.Permission)
	}
	// No record, no approval: the actions log is where the approval is kept.
	log, err := e.Actions()
	if err == nil {
		_, err = log.Do(ctx, actions.Action{Kind: "presence.approve", Target: repo, Identity: "human",
			Payload: map[string]any{"command": what, "approver": a.Login, "via": a.Via, "permission": a.Permission}},
			func(context.Context) (actions.Outcome, error) { return actions.Outcome{}, nil })
	}
	if err != nil {
		return nil, Wrap(ExitInternal, fmt.Errorf("%s: recording the approval in the actions log: %w; nothing was done", what, err))
	}
	access := a.Permission
	if access == "" {
		access = "no repository"
	}
	fmt.Fprintf(e.Err, "Approved by @%s (%s, %s access).\n", a.Login, a.Via, access)
	e.approval = a
	return a, nil
}

// permissionOf is admin, write (write or maintain), or what GitHub reports.
func permissionOf(p *gh.RepositoryPermissionLevel) string {
	switch {
	case p.GetPermission() == "admin":
		return "admin"
	case p.GetPermission() == "write" || p.GetRoleName() == "maintain":
		return "write"
	}
	return p.GetPermission()
}

// deviceCodeReply is GitHub's reply to POST /login/device/code.
type deviceCodeReply struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// githubWeb is github.com, or the test server standing in for GitHub.
func (e *Env) githubWeb() string {
	if e.GitHubAPI != "" {
		return e.GitHubAPI
	}
	return "https://github.com"
}

func (e *Env) devicePost(ctx context.Context, path string, form url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.githubWeb()+path, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := e.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("%s: HTTP %d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (e *Env) deviceCode(ctx context.Context, client string) (deviceCodeReply, error) {
	var r struct {
		deviceCodeReply
		Error string `json:"error"`
	}
	if err := e.devicePost(ctx, "/login/device/code", url.Values{"client_id": {client}}, &r); err != nil {
		return r.deviceCodeReply, err
	}
	if r.Error != "" || r.DeviceCode == "" {
		return r.deviceCodeReply, fmt.Errorf("GitHub refused the device code (%s); is Device Flow enabled on the app?", r.Error)
	}
	return r.deviceCodeReply, nil
}

// deviceToken polls until the person approves, denies, or the code expires.
func (e *Env) deviceToken(ctx context.Context, client string, code deviceCodeReply) (string, error) {
	interval := time.Duration(max(code.Interval, 1)) * time.Second
	if e.DevicePoll > 0 {
		interval = e.DevicePoll
	}
	deadline := time.Now().Add(time.Duration(code.ExpiresIn) * time.Second)
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(interval):
		}
		var r struct {
			AccessToken string `json:"access_token"`
			Error       string `json:"error"`
		}
		err := e.devicePost(ctx, "/login/oauth/access_token", url.Values{"client_id": {client}, "device_code": {code.DeviceCode},
			"grant_type": {"urn:ietf:params:oauth:grant-type:device_code"}}, &r)
		switch {
		case err != nil:
			return "", err
		case r.AccessToken != "":
			return r.AccessToken, nil
		case r.Error == "slow_down":
			interval += 5 * time.Second
		case r.Error == "access_denied":
			return "", fmt.Errorf("the request was denied")
		case r.Error == "expired_token":
			return "", fmt.Errorf("the code expired")
		case r.Error != "authorization_pending":
			return "", fmt.Errorf("GitHub replied %q", r.Error)
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("the code expired")
		}
	}
}
