import products from "../data/products.json";

// Linear scan over every product on every keystroke.
export function searchProducts(query: string) {
  const q = query.toLowerCase();
  return products.filter((p) => p.name.toLowerCase().includes(q) || p.description.toLowerCase().includes(q));
}
