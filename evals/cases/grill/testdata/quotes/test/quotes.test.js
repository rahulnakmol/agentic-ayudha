// Synthetic eval fixture: the quote amounts for the seeded parcels.
const test = require("node:test");
const assert = require("node:assert");
const { rateFor } = require("../src/rates");

test("seeded parcels keep their prices", async () => {
  assert.strictEqual(await rateFor("EH1 1AA", "LS1 1AA", 400), 345);
  assert.strictEqual(await rateFor("EH1 1AA", "ZZ1 1AA", 2500), 995);
});
