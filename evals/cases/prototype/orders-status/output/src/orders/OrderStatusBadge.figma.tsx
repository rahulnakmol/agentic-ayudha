import figma from "@figma/code-connect"
import { OrderStatusBadge } from "./OrderStatusBadge"

figma.connect(OrderStatusBadge, "https://www.figma.com/design/Ord3rsSt4tus/Orders-delivery-status?node-id=3-21", {
  props: {
    status: figma.enum("Status", { "Being packed": "packed", "Out for delivery": "out_for_delivery", Delivered: "delivered" }),
  },
  example: ({ status }) => <OrderStatusBadge status={status} />,
})
