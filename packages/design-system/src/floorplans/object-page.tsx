import type { ReactNode } from "react"

import { Tabs, TabsContent, TabsList, TabsTrigger } from "../components/ui/tabs"
import { KeyFacts, PageHeader } from "./shell"

export type ObjectPageSection = { id: string; title: string; content: ReactNode }

/**
 * Object page: header with title, status, actions, and key facts; an anchor bar; one section per facet.
 */
export function ObjectPage({
  breadcrumb,
  title,
  subtitle,
  status,
  actions,
  facts,
  sections,
  defaultSection,
}: {
  breadcrumb?: ReactNode
  title: ReactNode
  subtitle?: ReactNode
  status?: ReactNode
  actions?: ReactNode
  facts: { label: string; value: ReactNode }[]
  sections: ObjectPageSection[]
  defaultSection?: string
}) {
  return (
    <Tabs defaultValue={defaultSection ?? sections[0]?.id} className="gap-0">
      <PageHeader
        breadcrumb={breadcrumb}
        title={title}
        subtitle={subtitle}
        status={status}
        actions={actions}
        className="border-b-0 pb-0"
      >
        <KeyFacts facts={facts} />
        <TabsList variant="line" aria-label="Sections" className="-mb-px">
          {sections.map((s) => (
            <TabsTrigger key={s.id} value={s.id}>
              {s.title}
            </TabsTrigger>
          ))}
        </TabsList>
      </PageHeader>
      <div className="border-t" />
      {sections.map((s) => (
        <TabsContent key={s.id} value={s.id} className="px-page py-section">
          <section aria-labelledby={`section-${s.id}`} className="grid gap-stack">
            <h2 id={`section-${s.id}`} className="text-base font-semibold">
              {s.title}
            </h2>
            {s.content}
          </section>
        </TabsContent>
      ))}
    </Tabs>
  )
}
