import { Table, TableBody, TableCaption, TableCell, TableHead, TableHeader, TableRow } from "@aayudha/design-system"
import { OrderStatusBadge, type OrderStatus } from "./OrderStatusBadge"

export type Order = { id: string; placedAt: string; total: string; status: OrderStatus }

// A real table with a caption and column headers (research P3); at 390 px rows stack as cards (P5).
export function OrdersTable({ orders }: { orders: Order[] }) {
  return (
    <Table className="max-sm:[&_thead]:sr-only max-sm:[&_tr]:grid max-sm:[&_tr]:grid-cols-[1fr_auto]">
      <TableCaption>Your orders, newest first</TableCaption>
      <TableHeader>
        <TableRow>
          <TableHead scope="col">Order</TableHead>
          <TableHead scope="col">Date</TableHead>
          <TableHead scope="col">Total</TableHead>
          <TableHead scope="col">Delivery status</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {orders.map((o) => (
          <TableRow key={o.id} className="h-row">
            <TableCell>
              <a className="inline-flex min-h-control min-w-control items-center font-semibold text-primary" href={`/orders/${o.id}`}>
                #{o.id}
              </a>
            </TableCell>
            <TableCell>{new Date(o.placedAt).toLocaleDateString("en-IE", { day: "numeric", month: "short" })}</TableCell>
            <TableCell>{o.total}</TableCell>
            <TableCell><OrderStatusBadge status={o.status} /></TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}
