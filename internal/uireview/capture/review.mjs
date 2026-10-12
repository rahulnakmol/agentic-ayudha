// aayudha UI review capture (docs/BUILD-PLAN.md WP6.6), embedded in the aayudha binary and run with node by
// `aayu review`, `aayu polish`, and `aayu baseline`. Input (stdin, JSON): {mode, states: [{id, url}], out,
// min_target_px, axe_tags}. mode "chrome-path" prints Playwright's Chromium path. Otherwise, per state at
// 1280x800 with reduced motion: a screenshot at rest and one with keyboard focus on the first control (the
// images lane A sees), the page's markup and accessibility tree, and lane B's probes: axe, controls whose focus
// changes nothing visible, and targets under min_target_px. Output (stdout, JSON): [{id, url, ...}].
// Playwright and axe-core resolve from the product repository (cwd), then NODE_PATH.
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
mkdirSync(input.out, { recursive: true });

const controls = 'a[href], button, input:not([type=hidden]), select, textarea, [tabindex]:not([tabindex="-1"]), [role=button], [role=link]';

const browser = await chromium.launch();
const out = [];
for (const st of input.states) {
  const r = { id: st.id, url: st.url, axe: [], focus_missing: [], small_targets: [] };
  // bypassCSP: axe-core is injected as a script tag, which a page's own Content Security Policy would block.
  const context = await browser.newContext({ viewport: { width: 1280, height: 800 }, deviceScaleFactor: 1, reducedMotion: "reduce", bypassCSP: true });
  const page = await context.newPage();
  try {
    const resp = await page.goto(st.url, { waitUntil: "networkidle", timeout: 30000 });
    r.status = resp ? resp.status() : 0;
    if (r.status >= 400) throw new Error("HTTP " + r.status);
    r.rest = join(input.out, st.id + "-rest.png");
    await page.screenshot({ path: r.rest });
    r.html = (await page.content()).replace(/<!--[\s\S]*?-->/g, "").slice(0, 60000);
    r.aria = await page.locator("body").ariaSnapshot();

    await page.addScriptTag({ path: axePath });
    r.axe = await page.evaluate(async (tags) => {
      const res = await window.axe.run(document, { runOnly: { type: "tag", values: tags } });
      return res.violations.map((v) => ({ id: v.id, impact: v.impact, help: v.help, nodes: v.nodes.length, target: v.nodes[0] ? String(v.nodes[0].target) : "" }));
    }, input.axe_tags);

    const list = await page.evaluate(({ sel, min }) => {
      const res = [];
      let i = 0;
      for (const el of document.querySelectorAll(sel)) {
        const b = el.getBoundingClientRect();
        const cs = getComputedStyle(el);
        if (el.disabled || cs.visibility === "hidden" || cs.display === "none" || b.width === 0 || b.height === 0) continue;
        el.setAttribute("data-aayu-review", String(i));
        const inline = el.tagName === "A" && cs.display === "inline" && el.parentElement && el.parentElement.textContent.trim() !== el.textContent.trim();
        const label = (el.getAttribute("aria-label") || (el.labels && el.labels[0] && el.labels[0].textContent) || el.textContent || el.getAttribute("name") || el.tagName).trim().replace(/\s+/g, " ").slice(0, 40);
        res.push({ id: String(i), label, w: Math.round(b.width), h: Math.round(b.height), small: !inline && (b.width < min || b.height < min) });
        i++;
      }
      return res;
    }, { sel: controls, min: input.min_target_px });
    r.small_targets = list.filter((c) => c.small).map((c) => `${c.label} (${c.w}x${c.h})`);

    // Focus indicator: Tab through the page and compare each control's look focused and not.
    const rest = await page.evaluate((sel) => Object.fromEntries([...document.querySelectorAll("[data-aayu-review]")].map((el) => {
      const cs = getComputedStyle(el);
      return [el.getAttribute("data-aayu-review"), [cs.outlineStyle, cs.outlineWidth, cs.outlineColor, cs.boxShadow, cs.borderTopColor, cs.borderBottomColor,
        cs.borderTopWidth, cs.backgroundColor, cs.color, cs.textDecorationLine].join("|")];
    })), controls);
    const seen = new Set();
    for (let k = 0; k < list.length * 2 + 3; k++) {
      await page.keyboard.press("Tab");
      const now = await page.evaluate(() => {
        const el = document.activeElement;
        if (!el || !el.hasAttribute("data-aayu-review")) return null;
        const cs = getComputedStyle(el);
        return { id: el.getAttribute("data-aayu-review"), look: [cs.outlineStyle, cs.outlineWidth, cs.outlineColor, cs.boxShadow, cs.borderTopColor,
          cs.borderBottomColor, cs.borderTopWidth, cs.backgroundColor, cs.color, cs.textDecorationLine].join("|") };
      });
      if (!now || seen.has(now.id)) continue;
      seen.add(now.id);
      if (seen.size === 1) {
        r.focus = join(input.out, st.id + "-focus.png");
        await page.screenshot({ path: r.focus });
      }
      const [style, width] = now.look.split("|");
      const outlined = style !== "none" && parseFloat(width) > 0;
      if (!outlined && now.look === rest[now.id]) r.focus_missing.push(list.find((c) => c.id === now.id).label);
    }
  } catch (e) {
    r.error = String(e && e.message ? e.message : e).slice(0, 300);
  }
  await context.close();
  out.push(r);
}
await browser.close();
process.stdout.write(JSON.stringify(out));
