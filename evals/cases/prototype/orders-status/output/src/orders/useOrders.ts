import { useCallback, useEffect, useState } from "react"
import type { Order } from "./OrdersTable"

// Loads GET /api/orders (newest first from the server). On the preview target, ?seed=empty or ?seed=error
// stands in for those API answers so each state can be captured; production ignores the parameter.
export function useOrders() {
  const [orders, setOrders] = useState<Order[]>([])
  const [failed, setFailed] = useState(false)
  const [retrying, setRetrying] = useState(false)
  const load = useCallback(async () => {
    try {
      const res = await fetch(`/api/orders${window.location.search}`)
      if (!res.ok) throw new Error(String(res.status))
      setOrders(await res.json())
      setFailed(false)
    } catch {
      setOrders([]) // never keep a stale list (#423)
      setFailed(true)
    } finally {
      setRetrying(false)
    }
  }, [])
  useEffect(() => { void load() }, [load])
  return { orders, failed, retrying, retry: () => { setRetrying(true); void load() } }
}
