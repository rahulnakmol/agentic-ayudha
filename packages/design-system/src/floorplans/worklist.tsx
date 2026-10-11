import type { ReactNode } from "react"

import { Tabs, TabsContent, TabsList, TabsTrigger } from "../components/ui/tabs"
import { PageHeader } from "./shell"

export type WorklistTab = { value: string; label: string; count: number }

/**
 * Worklist: items a user has to process, split by tabs with counts. No filter bar; the table is the page.
 */
export function Worklist({
  title,
  actions,
  tabs,
  value,
  onValueChange,
  toolbar,
  children,
}: {
  title: ReactNode
  actions?: ReactNode
  tabs: WorklistTab[]
  value: string
  onValueChange: (value: string) => void
  /** Actions for the selected items. */
  toolbar?: ReactNode
  /** The table for the selected tab. */
  children: ReactNode
}) {
  return (
    <>
      <PageHeader title={title} actions={actions} />
      <Tabs value={value} onValueChange={onValueChange} className="gap-stack px-page py-section">
        <div className="flex flex-wrap items-center gap-stack">
          <TabsList aria-label="Work item status">
            {tabs.map((t) => (
              <TabsTrigger key={t.value} value={t.value}>
                {t.label} <span className="tabular-nums text-muted-foreground">({t.count})</span>
              </TabsTrigger>
            ))}
          </TabsList>
          {toolbar && <div className="ml-auto flex flex-wrap gap-2">{toolbar}</div>}
        </div>
        <TabsContent value={value} className="rounded-xl border bg-card">
          {children}
        </TabsContent>
      </Tabs>
    </>
  )
}
