// Playwright Test configuration of the aayudha screen-reader floor (WP4.7); see sr.spec.cjs.
const { screenReaderConfig } = require("@guidepup/playwright");

module.exports = {
  ...screenReaderConfig,
  testDir: __dirname,
  testMatch: "sr.spec.cjs",
  timeout: 5 * 60 * 1000,
  retries: 1,
  reporter: "line",
  projects: [{ name: "chromium", use: { browserName: "chromium", headless: false, viewport: { width: 1280, height: 800 } } }],
};
