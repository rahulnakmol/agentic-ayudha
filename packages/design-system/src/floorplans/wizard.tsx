import type { ReactNode } from "react"
import { CheckIcon } from "@phosphor-icons/react"

import { Button } from "../components/ui/button"
import { cn } from "../lib/utils"
import { PageHeader } from "./shell"

export type WizardStep = { id: string; title: string }

/**
 * Wizard: a long or unfamiliar task split into ordered steps, with a review step before the final action.
 */
export function Wizard({
  title,
  steps,
  current,
  onStepChange,
  onCancel,
  onFinish,
  finishLabel = "Submit",
  children,
}: {
  title: ReactNode
  steps: WizardStep[]
  /** Index of the current step. */
  current: number
  onStepChange: (index: number) => void
  onCancel?: () => void
  onFinish?: () => void
  finishLabel?: string
  /** Content of the current step. */
  children: ReactNode
}) {
  const last = current === steps.length - 1
  return (
    <>
      <PageHeader title={title}>
        <nav aria-label="Steps">
          <ol className="flex flex-wrap gap-stack">
            {steps.map((s, i) => {
              const done = i < current
              return (
                <li key={s.id} aria-current={i === current ? "step" : undefined} className="flex items-center gap-2">
                  <span
                    aria-hidden
                    className={cn(
                      "grid size-7 place-items-center rounded-full border text-xs font-semibold",
                      i === current && "border-primary bg-primary text-primary-foreground",
                      done && "border-primary text-primary",
                    )}
                  >
                    {done ? <CheckIcon /> : i + 1}
                  </span>
                  <span className={cn(i === current ? "font-semibold" : "text-muted-foreground")}>
                    {s.title}
                    {done && <span className="sr-only"> (completed)</span>}
                  </span>
                </li>
              )
            })}
          </ol>
        </nav>
      </PageHeader>
      <section aria-labelledby="wizard-step" className="mx-auto grid w-full max-w-3xl gap-section px-page py-section">
        <h2 id="wizard-step" className="text-xl font-semibold">
          {current + 1}. {steps[current].title}
        </h2>
        {children}
      </section>
      <footer className="sticky bottom-0 mt-auto flex justify-end gap-2 border-t bg-card px-page py-3">
        {onCancel && (
          <Button variant="ghost" onClick={onCancel}>
            Cancel
          </Button>
        )}
        <Button variant="outline" disabled={current === 0} onClick={() => onStepChange(current - 1)}>
          Back
        </Button>
        {last ? (
          <Button onClick={onFinish}>{finishLabel}</Button>
        ) : (
          <Button onClick={() => onStepChange(current + 1)}>Next: {steps[current + 1].title}</Button>
        )}
      </footer>
    </>
  )
}
