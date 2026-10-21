import { Badge } from "@aayudha/design-system"

export type OrderStatus = "packed" | "out_for_delivery" | "delivered"

// Status in words, on its status token pair (research P2: "a little label, like out for delivery").
const statuses: Record<OrderStatus, { label: string; tone: "neutral" | "informative" | "positive" }> = {
  packed: { label: "Being packed", tone: "neutral" },
  out_for_delivery: { label: "Out for delivery", tone: "informative" },
  delivered: { label: "Delivered", tone: "positive" },
}

export function OrderStatusBadge({ status }: { status: OrderStatus }) {
  const s = statuses[status]
  return <Badge variant={s.tone}>{s.label}</Badge>
}
