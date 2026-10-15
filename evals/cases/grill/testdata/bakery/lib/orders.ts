export type OrderLine = { sku: string; qty: number; options: string[] };

export async function submitOrder(lines: OrderLine[], pickupAt: string) {
  const res = await fetch("/api/orders", { method: "POST", body: JSON.stringify({ lines, pickupAt }) });
  return res.json();
}
