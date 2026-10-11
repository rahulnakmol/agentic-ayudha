import type { ReactNode } from "react"
import { FunnelSimpleIcon } from "@phosphor-icons/react"

import { Button } from "../components/ui/button"
import { PageHeader } from "./shell"

/**
 * List report: a filter bar in the page header, then one table of matching objects with a titled toolbar.
 */
export function ListReport({
  breadcrumb,
  title,
  actions,
  filters,
  onGo,
  tableTitle,
  tableActions,
  children,
}: {
  breadcrumb?: ReactNode
  title: ReactNode
  actions?: ReactNode
  /** Labelled filter fields. */
  filters: ReactNode
  onGo?: () => void
  /** Usually "Items (count)". */
  tableTitle: ReactNode
  tableActions?: ReactNode
  /** The result table. */
  children: ReactNode
}) {
  return (
    <>
      <PageHeader breadcrumb={breadcrumb} title={title} actions={actions}>
        <form
          role="search"
          aria-label="Filters"
          className="flex flex-wrap items-end gap-stack"
          onSubmit={(e) => {
            e.preventDefault()
            onGo?.()
          }}
        >
          {filters}
          <Button type="submit">
            <FunnelSimpleIcon aria-hidden /> Go
          </Button>
        </form>
      </PageHeader>
      <section aria-labelledby="list-report-table" className="grid gap-stack px-page py-section">
        <div className="flex flex-wrap items-center gap-stack">
          <h2 id="list-report-table" className="text-base font-semibold">
            {tableTitle}
          </h2>
          {tableActions && <div className="ml-auto flex flex-wrap gap-2">{tableActions}</div>}
        </div>
        <div className="rounded-xl border bg-card">{children}</div>
      </section>
    </>
  )
}
