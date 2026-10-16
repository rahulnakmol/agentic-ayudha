// Synthetic eval fixture: the quotes API's HTTP server.
const express = require("express");
const quotes = require("./routes/quotes");

const app = express();
app.use(express.json());
app.post("/quotes", quotes.create);
app.get("/quotes/:id", quotes.get);
app.get("/health", (_req, res) => res.json({ ok: true }));

app.listen(process.env.PORT || 8080);
