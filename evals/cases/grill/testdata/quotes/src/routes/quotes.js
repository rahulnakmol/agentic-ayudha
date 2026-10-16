// POST /quotes and GET /quotes/:id. Each parcel is priced on its own.
const { rateFor } = require("../rates");
const store = new Map();

async function create(req, res) {
  const { from, to, parcels } = req.body;
  const lines = [];
  for (const p of parcels) {
    // One rate lookup per parcel, in turn.
    lines.push({ weight_g: p.weight_g, pence: await rateFor(from, to, p.weight_g) });
  }
  const quote = { id: String(store.size + 1), from, to, lines, total_pence: lines.reduce((s, l) => s + l.pence, 0) };
  store.set(quote.id, quote);
  res.json(quote);
}

function get(req, res) {
  const q = store.get(req.params.id);
  if (!q) return res.status(404).json({ error: "no such quote" });
  res.json(q);
}

module.exports = { create, get };
