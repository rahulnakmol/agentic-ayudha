package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/ask"
	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/store"
)

type memory struct{ events []ask.Event }

func (m *memory) Record(e ask.Event) error { m.events = append(m.events, e); return nil }

func server(t *testing.T) (*httptest.Server, *memory) {
	t.Helper()
	s, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	m := &memory{}
	mux := http.NewServeMux()
	Register(mux, ask.New(s, m, nil), filepath.Join("..", "..", "..", "packages", "design-system", "src", "styles", "tokens.css"))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, m
}

type fetched struct {
	code    int
	header  http.Header
	cookies []*http.Cookie
	body    string
}

func get(t *testing.T, srv *httptest.Server, path string, cookies ...*http.Cookie) fetched {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, srv.URL+path, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return fetched{resp.StatusCode, resp.Header, resp.Cookies(), string(b)}
}

// Each state of the Ask page renders what the story's screenshots show: the
// empty page with a labelled field, an Ask button, and suggestions; an
// answer quoted with its source; a referral; and a refusal. A question is
// escaped, never markup.
func TestAskPageStates(t *testing.T) {
	srv, m := server(t)
	p := get(t, srv, "/")
	body := p.body
	for _, want := range []string{`<h1>Ask about your benefits</h1>`, `<label for="q">Your question</label>`, `<button type="submit">Ask</button>`,
		`href="/?q=What%20is%20the%20dental%20annual%20deductible%3f"`, `<link rel="stylesheet" href="/tokens.css">`, `lang="en"`} {
		if !strings.Contains(body, want) {
			t.Errorf("empty page lacks %s", want)
		}
	}
	if n := strings.Count(body, `<li><a href="/?q=`); n != len(Suggestions) || n != 3 {
		t.Errorf("%d suggested questions, want 3", n)
	}
	if p.header.Get("Content-Security-Policy") == "" || len(p.cookies) != 1 || p.cookies[0].Name != Cookie || !p.cookies[0].HttpOnly || !p.cookies[0].Secure {
		t.Errorf("headers: %v %v", p.header, p.cookies)
	}
	if len(m.events) != 1 || m.events[0].Name != ask.EventVisit || !strings.HasPrefix(m.events[0].User, "v-") {
		t.Errorf("visit events: %+v", m.events)
	}
	for q, want := range map[string][]string{
		"What is the dental annual deductible?": {`<h2 id="result">Answer</h2>`, "50 dollars per person", "Source: Dental, Deductible", `class="card answered"`},
		"Can I bring my dog to work?":           {`<h2 id="result">We sent this to HR</h2>`, "HR specialist will reply within 2 business days"},
		"What is my colleague's claim status?":  {`<h2 id="result">We cannot answer this</h2>`, "another person&#39;s records"},
		`<script>alert(1)</script>`:             {`&lt;script&gt;alert(1)&lt;/script&gt;`},
	} {
		body := get(t, srv, "/?q="+url.QueryEscape(q), &http.Cookie{Name: Cookie, Value: "emp-0001"}).body
		for _, w := range want {
			if !strings.Contains(body, w) {
				t.Errorf("%q: page lacks %s", q, w)
			}
		}
		if strings.Contains(body, "<script>") {
			t.Errorf("%q: unescaped markup", q)
		}
	}
	if last := m.events[len(m.events)-1]; last.User != "emp-0001" {
		t.Errorf("a valid visitor cookie is not reused: %+v", last)
	}
	// Each question page is a visit, a question, and its outcome.
	if n := len(m.events); n != 1+4*3 || m.events[1].Name != ask.EventVisit || m.events[2].Name != ask.EventAsked {
		t.Errorf("%d events: %+v", n, m.events)
	}
	for path, want := range map[string]string{"/app.css": "var(--ay-color-brand-default)", "/tokens.css": "--ay-color-brand-default:"} {
		p := get(t, srv, path)
		if p.code != 200 || !strings.HasPrefix(p.header.Get("Content-Type"), "text/css") || !strings.Contains(p.body, want) {
			t.Errorf("%s: %d %s", path, p.code, p.header.Get("Content-Type"))
		}
	}
	if p := get(t, srv, "/nope"); p.code != http.StatusNotFound {
		t.Errorf("/nope: %d", p.code)
	}
}

// The stylesheet uses design-system tokens only: no colour literal, no
// literal length or duration (only var(--ay-…) and calc() over one, with
// unitless factors), no unitless line-height, and every token it names exists
// in the design system's tokens.css.
func TestStylesUseTokens(t *testing.T) {
	b, err := static.ReadFile("static/app.css")
	if err != nil {
		t.Fatal(err)
	}
	css := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(string(b), "")
	for _, bad := range []*regexp.Regexp{
		regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b|\b(rgb|rgba|hsl|hsla|hwb|lab|lch|oklab|oklch|color)\(`),
		regexp.MustCompile(`\d(px|rem|em|ex|ch|%|vh|vw|vmin|vmax|pt|pc|cm|mm|in|ms|s)\b`),
		regexp.MustCompile(`line-height:\s*[\d.]+\s*[;}]`),
		regexp.MustCompile(`(?i)\b(white|black|red|green|blue|gray|grey)\b`),
	} {
		if m := bad.FindString(css); m != "" {
			t.Errorf("app.css holds a literal value: %q", m)
		}
	}
	tokens, err := os.ReadFile(filepath.Join("..", "..", "..", "packages", "design-system", "src", "styles", "tokens.css"))
	if err != nil {
		t.Fatal(err)
	}
	used := regexp.MustCompile(`var\((--ay-[a-z0-9-]+)\)`).FindAllStringSubmatch(css, -1)
	if len(used) < 20 {
		t.Errorf("only %d token references", len(used))
	}
	for _, m := range used {
		if !strings.Contains(string(tokens), m[1]+":") {
			t.Errorf("app.css names %s, which tokens.css does not define", m[1])
		}
	}
	for _, id := range []string{"", "x y", strings.Repeat("a", 65)} {
		if validID(id) {
			t.Errorf("visitor ID %q accepted", id)
		}
	}
}
