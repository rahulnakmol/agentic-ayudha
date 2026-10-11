import type { ReactNode } from "react"

import { PageHeader } from "./shell"

/**
 * Overview page: a role-based entry page of cards (KPIs, lists, tables), optionally filtered globally.
 */
export function OverviewPage({
  title,
  subtitle,
  filters,
  children,
}: {
  title: ReactNode
  subtitle?: ReactNode
  filters?: ReactNode
  /** The cards. */
  children: ReactNode
}) {
  return (
    <>
      <PageHeader title={title} subtitle={subtitle}>
        {filters}
      </PageHeader>
      <div className="grid grid-cols-[repeat(auto-fill,minmax(18rem,1fr))] items-start gap-section px-page py-section">
        {children}
      </div>
    </>
  )
}
