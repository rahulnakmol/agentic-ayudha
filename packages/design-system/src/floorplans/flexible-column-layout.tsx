import type { ReactNode } from "react"

import { cn } from "../lib/utils"

export type FlexibleColumn = { label: string; content: ReactNode }

/**
 * Flexible column layout: list, detail, and detail-of-detail side by side. Two columns split 1:2, three split
 * 1:2:1; below the md breakpoint only the last open column shows.
 */
export function FlexibleColumnLayout({
  begin,
  mid,
  end,
}: {
  begin: FlexibleColumn
  mid?: FlexibleColumn
  end?: FlexibleColumn
}) {
  const columns = [begin, mid, end].filter((c): c is FlexibleColumn => Boolean(c))
  const template = { 1: "md:grid-cols-1", 2: "md:grid-cols-[1fr_2fr]", 3: "md:grid-cols-[1fr_2fr_1fr]" }[columns.length]
  return (
    <div className={cn("grid min-h-0 flex-1 md:divide-x", template)}>
      {columns.map((c, i) => (
        <section
          key={c.label}
          aria-label={c.label}
          className={cn("min-w-0 overflow-auto bg-card", i < columns.length - 1 && "hidden md:block")}
        >
          {c.content}
        </section>
      ))}
    </div>
  )
}
