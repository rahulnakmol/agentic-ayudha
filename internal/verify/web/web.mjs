// aayudha web floors (docs/BUILD-PLAN.md WP4.5), embedded in the aayudha binary and run with node by
// `aayu verify`. Input (stdin, JSON): {mode, states: [{n, kind, url, video}], out, checks}. Output (stdout,
// JSON). Playwright and axe-core resolve from the product repository (cwd), then NODE_PATH.
import { createRequire } from "node:module";
import { readFileSync, mkdirSync } from "node:fs";
import { join } from "node:path";

const require = createRequire(join(process.cwd(), "package.json"));
const { chromium } = require("playwright");
const input = JSON.parse(readFileSync(0, "utf8"));

if (input.mode === "chrome-path") {
  process.stdout.write(JSON.stringify({ chrome: chromium.executablePath() }));
  process.exit(0);
}

const axePath = require.resolve("axe-core/axe.min.js");
const versions = {
  playwright: require("playwright/package.json").version,
  axe: require("axe-core/package.json").version,
};
const c = input.checks;
mkdirSync(input.out, { recursive: true });

// Synthetic data uses reserved domains (RFC 2606/6761); any other address is treated as personal data.
const reserved = /(^|\.)(example\.(com|org|net)|test|invalid|localhost|example)$/i;
const emailRe = /[A-Za-z0-9._%+-]+@([A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)+)/g;
const cardRe = /\b(?:\d[ -]?){13,19}\b/g;
const ssnRe = /\b\d{3}-\d{2}-\d{4}\b/g;

function luhn(digits) {
  let sum = 0;
  for (let i = 0; i < digits.length; i++) {
    let d = Number(digits[digits.length - 1 - i]);
    if (i % 2 === 1) {
      d *= 2;
      if (d > 9) d -= 9;
    }
    sum += d;
  }
  return sum % 10 === 0;
}

function mask(s) {
  return s.length <= 4 ? "****" : s.slice(0, 2) + "***" + s.slice(-2);
}

export function scanPII(text) {
  const found = [];
  for (const m of text.matchAll(emailRe)) {
    if (!reserved.test(m[1])) found.push("email address " + mask(m[0]));
  }
  for (const m of text.matchAll(cardRe)) {
    const d = m[0].replace(/\D/g, "");
    if (d.length >= 13 && luhn(d)) found.push("payment card number " + mask(d));
  }
  for (const m of text.matchAll(ssnRe)) found.push("national ID number " + mask(m[0]));
  return found;
}

const browser = await chromium.launch();
const results = [];
for (const st of input.states) {
  // bypassCSP: axe-core is injected as a script tag, which a page's own Content Security Policy would block.
  const ctxOpts = { viewport: { width: 1280, height: 800 }, reducedMotion: c.reduced_motion ? "reduce" : "no-preference", bypassCSP: true };
  if (st.video) ctxOpts.recordVideo = { dir: join(input.out, "video-" + st.n), size: { width: 1280, height: 800 } };
  const context = await browser.newContext(ctxOpts);
  const page = await context.newPage();
  const r = { n: st.n, url: st.url };
  try {
    const resp = await page.goto(st.url, { waitUntil: "networkidle", timeout: 30000 });
    r.status = resp ? resp.status() : 0;

    // DOM masking: values that can hold personal data never reach a capture.
    await page.evaluate(() => {
      for (const el of document.querySelectorAll("[data-aayu-mask]")) el.textContent = "\u2588\u2588\u2588";
      for (const el of document.querySelectorAll('input[type=password], input[type=email], input[type=tel], input[autocomplete^="cc-"]'))
        el.value = "\u2588\u2588\u2588";
    });
    const text = await page.evaluate(() =>
      document.body.innerText + "\n" + [...document.querySelectorAll("input, textarea")].map((e) => e.value).join("\n"));
    r.pii = scanPII(text);

    await page.addScriptTag({ path: axePath });
    const axe = await page.evaluate(async (tags) => {
      const res = await window.axe.run(document, { runOnly: { type: "tag", values: tags } });
      return res.violations.map((v) => ({ id: v.id, impact: v.impact, help: v.help, nodes: v.nodes.length, target: v.nodes[0] ? String(v.nodes[0].target) : "" }));
    }, c.axe_tags);
    r.axe = axe;

    r.motion = await page.evaluate(() =>
      document.getAnimations().filter((a) => a.playState === "running").map((a) => (a.animationName || a.constructor.name)));

    const sel = 'a[href], button, input:not([type=hidden]), select, textarea, [tabindex]:not([tabindex="-1"]), [role=button], [role=link]';
    const focusables = await page.evaluate((sel) => {
      const out = [];
      let i = 0;
      for (const el of document.querySelectorAll(sel)) {
        const r = el.getBoundingClientRect();
        const cs = getComputedStyle(el);
        if (el.disabled || cs.visibility === "hidden" || cs.display === "none" || r.width === 0 || r.height === 0) continue;
        el.setAttribute("data-aayu-kbd", String(i));
        const inline = el.tagName === "A" && cs.display === "inline" && el.parentElement && el.parentElement.textContent.trim() !== el.textContent.trim();
        out.push({ id: String(i), label: (el.getAttribute("aria-label") || el.textContent || el.getAttribute("name") || el.tagName).trim().slice(0, 40),
          w: r.width, h: r.height, inline });
        i++;
      }
      return out;
    }, sel);
    const reached = new Set();
    for (let k = 0; k < focusables.length * 2 + 5; k++) {
      await page.keyboard.press("Tab");
      const id = await page.evaluate(() => document.activeElement && document.activeElement.getAttribute("data-aayu-kbd"));
      if (id !== null && id !== undefined) reached.add(id);
    }
    r.keyboard_unreachable = c.keyboard ? focusables.filter((f) => !reached.has(f.id)).map((f) => f.label) : [];
    r.small_targets = focusables.filter((f) => !f.inline && (f.w < c.min_target_px || f.h < c.min_target_px))
      .map((f) => `${f.label} (${Math.round(f.w)}x${Math.round(f.h)})`);

    if (!r.pii.length) {
      r.screenshot = join(input.out, `state-${st.n}.png`);
      await page.screenshot({ path: r.screenshot, fullPage: true });
    }

    r.clipped = await page.evaluate((scale) => {
      document.documentElement.style.zoom = String(scale / 100);
      const out = [];
      for (const el of document.querySelectorAll("body *")) {
        const cs = getComputedStyle(el);
        if (!el.childNodes.length || ![...el.childNodes].some((n) => n.nodeType === 3 && n.textContent.trim())) continue;
        const clips = ["hidden", "clip"].includes(cs.overflowX) || ["hidden", "clip"].includes(cs.overflowY) || cs.textOverflow === "ellipsis";
        if (clips && (el.scrollWidth > el.clientWidth + 1 || el.scrollHeight > el.clientHeight + 1)) out.push(el.textContent.trim().slice(0, 40));
      }
      return out;
    }, c.text_scale_percent);
  } catch (e) {
    r.error = String(e && e.message ? e.message : e).slice(0, 300);
  }
  await context.close();
  if (st.video && !r.pii?.length) r.video = await page.video()?.path();
  results.push(r);
}
await browser.close();
process.stdout.write(JSON.stringify({ versions, states: results }));
