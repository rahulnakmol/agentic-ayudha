import { useRef, useState } from "react"
import { Alert, AlertDescription, AlertTitle, Button, Tabs, TabsList, TabsTrigger } from "@aayudha/design-system"
import { WindowOption, type Window } from "./WindowOption"

type Saved = { day: string; start: string; end: string }

export function DeliveryWindowForm({ windows, save }: {
  windows: Window[]
  save: (id: string) => Promise<{ ok: true; saved: Saved } | { ok: false; nearest: Window[] }>
}) {
  const days = [...new Set(windows.map((w) => w.day))]
  const [day, setDay] = useState(days[0])
  const [chosen, setChosen] = useState<string>()
  const [full, setFull] = useState<{ window: Window; nearest: Window[] }>()
  const [saved, setSaved] = useState<Saved>()
  const alertRef = useRef<HTMLDivElement>(null)

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    if (!chosen) return
    const r = await save(chosen)
    if (r.ok) return setSaved(r.saved)
    // The window filled up (#572): say which, offer the nearest, and never change the choice silently.
    setFull({ window: windows.find((w) => w.id === chosen)!, nearest: r.nearest })
    setChosen(undefined)
    requestAnimationFrame(() => alertRef.current?.focus())
  }

  if (saved) {
    return (
      <Alert variant="positive" role="status">
        <AlertTitle>Saved: we deliver on {saved.day}, {saved.start}–{saved.end}</AlertTitle>
        <AlertDescription>If your plans change, choose another window here.</AlertDescription>
        <div className="mt-stack flex flex-wrap gap-stack">
          <Button variant="outline" onClick={() => setSaved(undefined)}>Change window</Button>
          <Button asChild><a href="/orders">Back to your orders</a></Button>
        </div>
      </Alert>
    )
  }
  return (
    <form onSubmit={submit} className="rounded-container border border-border bg-surface p-section">
      {full && (
        <Alert variant="critical" role="alert" tabIndex={-1} ref={alertRef} className="mb-section">
          <AlertTitle>{full.window.start}–{full.window.end} on {full.window.day} filled up while you were choosing</AlertTitle>
          <AlertDescription>Choose another window. These are the nearest ones still free:</AlertDescription>
          {full.nearest.map((w) => <WindowOption key={w.id} window={w} name="window" showDay checked={chosen === w.id} onChange={setChosen} />)}
        </Alert>
      )}
      <Tabs value={day} onValueChange={setDay} className="mb-section">
        <TabsList aria-label="Delivery day">
          {days.map((d) => <TabsTrigger key={d} value={d} className="h-control">{d}</TabsTrigger>)}
        </TabsList>
      </Tabs>
      <fieldset className="mb-section">
        <legend className="mb-stack font-semibold">Windows on {day}</legend>
        {windows.filter((w) => w.day === day).map((w) => (
          <WindowOption key={w.id} window={w} name="window" checked={chosen === w.id} onChange={setChosen} />
        ))}
      </fieldset>
      <Button type="submit" className="max-sm:w-full">Save delivery window</Button>
    </form>
  )
}
