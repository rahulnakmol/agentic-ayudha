// Rate lookup: reads and parses the rate table from disk on every call.
const fs = require("fs/promises");
const path = require("path");

async function rateFor(from, to, weightG) {
  const table = JSON.parse(await fs.readFile(path.join(__dirname, "..", "data", "rates.json"), "utf8"));
  const zone = table.zones[`${from.slice(0, 2)}-${to.slice(0, 2)}`] || table.zones.default;
  const band = zone.find((b) => weightG <= b.max_g) || zone[zone.length - 1];
  return band.pence;
}

module.exports = { rateFor };
