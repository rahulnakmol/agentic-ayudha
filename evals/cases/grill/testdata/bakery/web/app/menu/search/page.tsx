"use client";
import { useState } from "react";
import { searchProducts } from "../../../../lib/search";
import { ProductTile } from "../../../components/ProductTile";
import { track } from "../../../../lib/analytics";

export default function Search() {
  const [q, setQ] = useState("");
  const results = searchProducts(q);
  return (
    <main>
      <input aria-label="Search the menu" value={q} onChange={(e) => { setQ(e.target.value); track("search_performed", { q }); }} />
      {results.length === 0 ? <p>No matches</p> : results.map((p) => <ProductTile key={p.sku} name={p.name} price={p.price} />)}
    </main>
  );
}
