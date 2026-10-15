// aayudha prototype capture (docs/BUILD-PLAN.md WP6.5), embedded in the aayudha binary and run with node by
// `aayu prototype`. Input (stdin, JSON): {states: [{id, url, width, height}], out}. For each state it renders the
// page at the Figma frame's size (device scale 1, reduced motion), saves a viewport screenshot, and samples the
// computed colours and type of every visible element with its own text or a visible box. Output (stdout, JSON):
// [{id, url, png, styles, error}]. Playwright resolves from the product repository (cwd), then NODE_PATH.
import { createRequire } from "node:module";
import { readFileSync, mkdirSync } from "node:fs";
import { join } from "node:path";

const require = createRequire(join(process.cwd(), "package.json"));
const { chromium } = require("playwright");
const input = JSON.parse(readFileSync(0, "utf8"));
mkdirSync(input.out, { recursive: true });

const browser = await chromium.launch();
const out = [];
for (const st of input.states) {
  const r = { id: st.id, url: st.url, png: "", styles: [] };
  const context = await browser.newContext({ viewport: { width: st.width, height: st.height }, deviceScaleFactor: 1, reducedMotion: "reduce" });
  const page = await context.newPage();
  try {
    const resp = await page.goto(st.url, { waitUntil: "networkidle", timeout: 30000 });
    if (resp && resp.status() >= 400) throw new Error("HTTP " + resp.status());
    await page.evaluate(() => document.fonts && document.fonts.ready);
    r.png = st.id + ".png";
    await page.screenshot({ path: join(input.out, r.png) });
    r.styles = await page.evaluate(() => {
      const skip = new Set(["SCRIPT", "STYLE", "HEAD", "META", "LINK", "TITLE", "NOSCRIPT", "TEMPLATE"]);
      const name = (el) => {
        let s = el.tagName.toLowerCase();
        if (el.id) return s + "#" + el.id;
        if (el.classList && el.classList.length) s += "." + [...el.classList].slice(0, 2).join(".");
        return s;
      };
      const res = [];
      for (const el of document.querySelectorAll("body, body *")) {
        if (skip.has(el.tagName)) continue;
        const cs = getComputedStyle(el);
        const box = el.getBoundingClientRect();
        if (cs.display === "none" || cs.visibility === "hidden" || box.width === 0 || box.height === 0) continue;
        const text = [...el.childNodes].some((n) => n.nodeType === 3 && n.textContent.trim());
        const colors = [];
        if (text) colors.push(cs.color);
        colors.push(cs.backgroundColor);
        for (const side of ["Top", "Right", "Bottom", "Left"]) {
          if (parseFloat(cs["border" + side + "Width"]) > 0 && cs["border" + side + "Style"] !== "none") colors.push(cs["border" + side + "Color"]);
        }
        if (cs.outlineStyle !== "none" && parseFloat(cs.outlineWidth) > 0) colors.push(cs.outlineColor);
        if (el instanceof SVGElement) {
          for (const v of [cs.fill, cs.stroke]) if (v && v !== "none") colors.push(v);
        }
        const s = { selector: name(el), colors };
        if (text) Object.assign(s, { font_size: cs.fontSize, font_weight: cs.fontWeight, font_family: cs.fontFamily });
        res.push(s);
      }
      return res;
    });
  } catch (e) {
    r.error = String(e && e.message ? e.message : e).slice(0, 300);
  }
  await context.close();
  out.push(r);
}
await browser.close();
process.stdout.write(JSON.stringify(out));
