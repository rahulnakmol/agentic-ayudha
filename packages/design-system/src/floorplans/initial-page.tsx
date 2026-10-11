import type { ReactNode } from "react"

/**
 * Initial page: one focused entry point (an ID or search) that opens an object, with optional shortcuts below.
 */
export function InitialPage({
  title,
  description,
  children,
  aside,
}: {
  title: ReactNode
  description?: ReactNode
  /** The entry form: usually one labelled input and a primary action. */
  children: ReactNode
  /** Recent items or shortcuts. */
  aside?: ReactNode
}) {
  return (
    <div className="mx-auto grid w-full max-w-2xl gap-section px-page py-[calc(var(--ay-space-section)*2)]">
      <div className="grid gap-2 text-center">
        <h1 className="text-3xl font-semibold">{title}</h1>
        {description && <p className="text-muted-foreground">{description}</p>}
      </div>
      <div className="rounded-xl border bg-card p-section">{children}</div>
      {aside}
    </div>
  )
}
