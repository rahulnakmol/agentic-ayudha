import { useEffect, useState } from "react"
import type { Window } from "./WindowOption"

// GET /api/delivery-windows?days=3 and PUT /api/preferences/window. A 409 {error: "full", nearest} is the
// slot-full state. On the preview target, ?seed=slot-full or ?seed=saved stands in for those API answers so
// each state can be captured; production ignores the parameter.
export function useDeliveryWindows() {
  const [windows, setWindows] = useState<Window[]>([])
  useEffect(() => {
    void fetch(`/api/delivery-windows?days=3&${window.location.search.slice(1)}`).then(async (r) => setWindows(await r.json()))
  }, [])
  async function save(windowId: string) {
    const r = await fetch(`/api/preferences/window${window.location.search}`, { method: "PUT", body: JSON.stringify({ windowId }) })
    if (r.status === 409) return { ok: false as const, nearest: (await r.json()).nearest as Window[] }
    const w = windows.find((x) => x.id === windowId)!
    return { ok: true as const, saved: { day: w.day, start: w.start, end: w.end } }
  }
  return { windows, save }
}
