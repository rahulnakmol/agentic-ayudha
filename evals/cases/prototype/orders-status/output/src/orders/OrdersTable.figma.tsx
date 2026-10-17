import figma from "@figma/code-connect"
import { OrdersTable } from "./OrdersTable"

figma.connect(OrdersTable, "https://www.figma.com/design/Ord3rsSt4tus/Orders-delivery-status?node-id=3-30", {
  example: () => <OrdersTable orders={[{ id: "10482", placedAt: "2026-10-14", total: "€42.10", status: "out_for_delivery" }]} />,
})
