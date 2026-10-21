// New for #570 (Figma "Window option", node 41:3): one radio row of the window list. The radio itself is the
// 44 px target (cozy control height), so it meets the target floor on phones; a full window is disabled and says so.
export type Window = { id: string; day: string; start: string; end: string; available: boolean }

export function WindowOption({ window: w, name, checked, onChange, showDay = false }: {
  window: Window; name: string; checked: boolean; onChange: (id: string) => void; showDay?: boolean
}) {
  return (
    <label className="flex min-h-row items-center gap-stack border-t border-border">
      <input
        type="radio" name={name} value={w.id} checked={checked} disabled={!w.available}
        onChange={() => onChange(w.id)}
        className="size-control flex-none appearance-none rounded-full border-2 border-field-border bg-field checked:border-primary checked:bg-primary checked:shadow-[inset_0_0_0_12px_var(--ay-color-field-background)] disabled:border-border disabled:bg-neutral-background focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
      />
      <span className="flex-1 font-semibold">{w.start}–{w.end}{showDay ? `, ${w.day}` : ""}</span>
      <span className="text-body text-label">{w.available ? "Available" : "Full, cannot be chosen"}</span>
    </label>
  )
}
