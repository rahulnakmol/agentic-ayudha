// Package web is the benefits-qa example's Ask page (layer L01,
// docs/BUILD-PLAN.md WP8.1): a server-rendered page, styled only with the
// design-system tokens, that asks the question workflow and shows the
// answer with its sources, a referral to HR, or a refusal. The question
// travels in the URL (GET /?q=…), so every state has an address.
package web

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"html/template"
	"net/http"
	"regexp"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/ask"
	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/contracts"
)

//go:embed static/app.css
var static embed.FS

// Cookie holds the pseudonymous visitor ID analytics keys on.
const Cookie = "bqa_uid"

// Suggestions are the example questions offered on the empty page.
var Suggestions = []string{
	"What is the dental annual deductible?",
	"How many weeks of parental leave do I get?",
	"Does Example Co match my 401(k) contributions?",
}

var page = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{if .Q}}{{.Q}} · {{end}}Benefits answers · Example Co</title>
<meta name="description" content="Answers to Example Co benefits questions, quoted from the plan documents.">
<link rel="stylesheet" href="/tokens.css">
<link rel="stylesheet" href="/app.css">
</head>
<body>
<header class="shell"><span class="product">Example Co Benefits</span><span class="year">Plan year 2026</span></header>
<main id="main">
<h1>Ask about your benefits</h1>
<p class="lead">Answers quote the Example Co plan documents and name their source. Questions the documents do not answer go to an HR specialist.</p>
<form method="get" action="/" role="search">
<label for="q">Your question</label>
<div class="row"><input id="q" name="q" type="text" maxlength="500" value="{{.Q}}" autocomplete="off" required><button type="submit">Ask</button></div>
</form>
{{with .Resp}}<section class="card {{.Status}}" aria-labelledby="result">
{{if eq .Status "answered"}}<h2 id="result">Answer</h2>
<blockquote><p>{{.Answer}}</p></blockquote>
{{range .Citations}}<p class="source">Source: {{.Plan}}, {{.Section}}</p>
{{end}}{{else if eq .Status "referred"}}<h2 id="result">We sent this to HR</h2>
<p>{{.Message}}</p>
{{else}}<h2 id="result">We cannot answer this</h2>
<p>{{.Message}}</p>
{{end}}</section>
{{else}}<section aria-labelledby="try">
<h2 id="try">Try a question</h2>
<ul class="try">{{range $.Suggestions}}<li><a href="/?q={{.}}">{{.}}</a></li>{{end}}</ul>
</section>
{{end}}</main>
<footer>Synthetic demo data for the fictional employer Example Co. This is not real benefits advice.</footer>
</body>
</html>
`))

type view struct {
	Q           string
	Resp        *contracts.AskResponse
	Suggestions []string
}

// Register adds the page and its stylesheets. tokens is the path of the
// design system's tokens.css.
func Register(mux *http.ServeMux, svc *ask.Service, tokens string) {
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		user := visitor(w, r)
		v := view{Q: strings.TrimSpace(r.URL.Query().Get("q")), Suggestions: Suggestions}
		_ = svc.Visit(user) // every page view is a visit; analytics counts a visitor's first
		if v.Q != "" {
			resp, _ := svc.Ask(user, v.Q) // a recorder failure never hides the answer
			v.Resp = &resp
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		_ = page.Execute(w, v)
	})
	mux.HandleFunc("GET /app.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		http.ServeFileFS(w, r, static, "static/app.css")
	})
	mux.HandleFunc("GET /tokens.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		http.ServeFile(w, r, tokens)
	})
}

// visitor returns the visitor's pseudonymous ID, setting one when absent.
func visitor(w http.ResponseWriter, r *http.Request) string {
	if c, err := r.Cookie(Cookie); err == nil && validID(c.Value) {
		return c.Value
	}
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	id := "v-" + hex.EncodeToString(b)
	http.SetCookie(w, &http.Cookie{Name: Cookie, Value: id, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode, MaxAge: 365 * 24 * 3600})
	return id
}

func validID(s string) bool { return idRE.MatchString(s) }

var idRE = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)
