export function BasketSummary({ items, total }: { items: number; total: string }) {
  return <aside className="crumb-basket">{items} items, {total}</aside>;
}
