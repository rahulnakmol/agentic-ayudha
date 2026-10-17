"use client"
import { useBooleanFlagValue } from "@openfeature/react-sdk"
import { Card } from "@aayudha/design-system"
import { useOrders } from "../../orders/useOrders"
import { OrdersTable } from "../../orders/OrdersTable"
import { NoOrders, OrdersFailed } from "../../orders/OrdersMessages"
import { LegacyOrdersPage } from "../../orders/LegacyOrdersPage"

export default function OrdersPage() {
  // Off by default (flags.flagd.json); flagd turns it on for environment "preview" only.
  const deliveryStatus = useBooleanFlagValue("orders-delivery-status", false)
  const { orders, failed, retrying, retry } = useOrders()
  if (!deliveryStatus) return <LegacyOrdersPage />
  return (
    <main className="mx-auto max-w-3xl px-page py-page">
      <h1 className="mb-section text-header-2 font-semibold">Your orders</h1>
      <Card className="p-section">
        {failed ? <OrdersFailed onRetry={retry} retrying={retrying} />
          : orders.length === 0 ? <NoOrders />
          : <OrdersTable orders={orders} />}
      </Card>
    </main>
  )
}
