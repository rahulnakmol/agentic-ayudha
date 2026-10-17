"use client"
import { useBooleanFlagValue } from "@openfeature/react-sdk"
import { DeliveryWindowForm } from "../../delivery/DeliveryWindowForm"
import { useDeliveryWindows } from "../../delivery/useDeliveryWindows"
import { LegacyPreferences } from "../../delivery/LegacyPreferences"

export default function PreferencesPage() {
  // Off by default (flags.flagd.json); flagd turns it on for environment "preview" only.
  const choice = useBooleanFlagValue("delivery-window-choice", false)
  const { windows, save } = useDeliveryWindows()
  if (!choice) return <LegacyPreferences />
  return (
    <main className="mx-auto max-w-xl px-page py-page">
      <h1 className="mb-stack text-header-2 font-semibold">Delivery window</h1>
      <p className="mb-section text-label">Choose when we deliver your next order.</p>
      <DeliveryWindowForm windows={windows} save={save} />
    </main>
  )
}
