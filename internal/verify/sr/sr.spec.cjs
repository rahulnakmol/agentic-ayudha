// aayudha screen-reader floor (docs/BUILD-PLAN.md WP4.7), embedded in the aayudha binary. `aayu verify` writes
// this spec, playwright.config.cjs, and states.json to a scratch directory and runs Playwright Test with the
// product's @playwright/test and @guidepup/playwright (CommonJS, so NODE_PATH resolves them). VoiceOver on macOS,
// NVDA on Windows; one transcript per state.
const { readFileSync, writeFileSync } = require("node:fs");
const { join } = require("node:path");
const { voiceOverTest, nvdaTest } = require("@guidepup/playwright");

const states = JSON.parse(readFileSync(join(__dirname, "states.json"), "utf8"));

async function capture(page, sr, st) {
  await page.goto(st.url, { waitUntil: "load" });
  await sr.navigateToWebContent();
  for (let i = 0; i < st.steps; i++) await sr.next();
  writeFileSync(join(__dirname, `transcript-${st.n}.json`), JSON.stringify(await sr.spokenPhraseLog()));
}

for (const st of states) {
  if (process.platform === "darwin") {
    voiceOverTest(`state ${st.n}`, async ({ page, voiceOver }) => { await capture(page, voiceOver, st); });
  } else {
    nvdaTest(`state ${st.n}`, async ({ page, nvda }) => { await capture(page, nvda, st); });
  }
}
