// Card payments through the bakery's payment stub. The seeded "declining" test card is always declined.
export async function chargeCard(amount: string, card: string) {
  const res = await fetch("/api/payments", { method: "POST", body: JSON.stringify({ amount, card }) });
  return res.json() as Promise<{ status: "paid" | "declined" }>;
}
