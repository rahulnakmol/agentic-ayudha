package tell

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
)

// AppCodePage is the only redirect_url the start page accepts: the github.com
// page where GitHub's one-time code lands (initrepo.AppCodePage, #578).
const AppCodePage = "https://github.com/rahulnakmol/agentic-ayudha/blob/dev/docs/app-code.md"

// appManifestScript reads the manifest from the link's fragment, which no
// server receives, removes it from the address bar, and posts the manifest only
// to GitHub's App registration page, only with a github.com redirect.
const appManifestScript = `(function () {
  var p = new URLSearchParams(location.hash.slice(1));
  history.replaceState(null, "", location.pathname);
  var msg = document.getElementById("msg");
  var to = p.get("to") || "", state = p.get("state") || "", m;
  try { m = JSON.parse(p.get("manifest") || ""); } catch (e) { m = null; }
  if (!/^(settings|organizations\/[A-Za-z0-9-]{1,39}\/settings)\/apps\/new$/.test(to) || !/^[0-9a-f]{32}$/.test(state) ||
      !m || typeof m.name !== "string" || m.redirect_url !== "APPCODEPAGE") {
    msg.textContent = "This link is incomplete or was changed. Run aayu init --identity again and open the new link.";
    return;
  }
  document.getElementById("name").textContent = m.name;
  document.getElementById("manifest").value = JSON.stringify(m);
  var f = document.getElementById("f");
  f.action = "https://github.com/" + to + "?state=" + state;
  f.hidden = false;
  msg.textContent = "";
})();`

// appManifestPage is the start of init --identity from any device. It loads
// nothing from anywhere else, sends no referrer, and its policy lets it post
// only to github.com.
var appManifestPage = func() []byte {
	script := strings.Replace(appManifestScript, "APPCODEPAGE", AppCodePage, 1)
	sum := sha256.Sum256([]byte(script))
	csp := "default-src 'none'; script-src 'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'; style-src 'unsafe-inline'; form-action https://github.com; base-uri 'none'"
	return []byte(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta http-equiv="Content-Security-Policy" content="` + csp + `">
<meta name="referrer" content="no-referrer">
<title>aayudha: create the bot App</title>
<style>body{font:1.1rem/1.5 system-ui,sans-serif;max-width:34rem;margin:2rem auto;padding:0 1rem}button{font:inherit;padding:.6rem 1.2rem}</style>
</head>
<body>
<main>
<h1>Create the aayudha bot App</h1>
<p id="msg">Reading the link…</p>
<form id="f" method="post" hidden>
<p>GitHub will ask you to confirm the App <strong id="name"></strong>. It gets only what agents need: contents, issues and pull requests write; actions and checks read.</p>
<p>After you confirm, GitHub shows an aayudha page. Copy that page's whole address and paste it into the terminal where <code>aayu init --identity</code> is waiting.</p>
<input type="hidden" name="manifest" id="manifest">
<button type="submit">Continue to GitHub</button>
</form>
</main>
<script>` + script + `</script>
</body>
</html>
`)
}()
