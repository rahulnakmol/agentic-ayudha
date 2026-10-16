// Synthetic eval fixture: today's behaviour with every option given.
const test = require("node:test");
const assert = require("node:assert");
const { createSlugger } = require("../src/index.js");

test("slug with all options", () => {
  const s = createSlugger({ separator: "-", lowercase: true, maxLength: 0, locale: "en", transliterate: true });
  assert.strictEqual(s.slug("Crème Brûlée Recipe"), "creme-brulee-recipe");
});

test("missing options throw", () => {
  assert.throws(() => createSlugger({ separator: "-" }), /invalid options/);
});
