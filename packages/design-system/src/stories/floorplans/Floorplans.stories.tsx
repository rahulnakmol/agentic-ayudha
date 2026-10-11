import type { Meta, StoryObj } from "@storybook/react-vite"
import { useState, type ReactNode } from "react"
import {
  ArrowRightIcon,
  BellIcon,
  DownloadSimpleIcon,
  PencilSimpleIcon,
  PlusIcon,
  XIcon,
} from "@phosphor-icons/react"

import { Badge } from "../../components/ui/badge"
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "../../components/ui/breadcrumb"
import { Button } from "../../components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../../components/ui/card"
import { Checkbox } from "../../components/ui/checkbox"
import { Input } from "../../components/ui/input"
import { Label } from "../../components/ui/label"
import { Progress } from "../../components/ui/progress"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/select"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../../components/ui/table"
import { Textarea } from "../../components/ui/textarea"
import {
  AppShell,
  FlexibleColumnLayout,
  InitialPage,
  KeyFacts,
  ListReport,
  ObjectPage,
  OverviewPage,
  Wizard,
  Worklist,
} from "../../floorplans"
import { claims, statusVariant, type Claim } from "../fixtures"

const meta = {
  title: "Floorplans",
  parameters: {
    fullBleed: true,
    docs: {
      story: { inline: false, height: "720px" },
      description: {
        component:
          "Seven Fiori-shaped floorplans composed from the primitives. Shapes and interaction patterns only; no SAP assets, icons, or fonts. Data is synthetic.",
      },
    },
  },
} satisfies Meta

export default meta
type Story = StoryObj<typeof meta>

function Frame({ children }: { children: ReactNode }) {
  return (
    <AppShell
      productName="Benefits"
      userName="Asha Rao"
      userInitials="AR"
      shellActions={
        <Button variant="ghost" size="icon" aria-label="Notifications">
          <BellIcon aria-hidden />
        </Button>
      }
    >
      {children}
    </AppShell>
  )
}

function ClaimsTable({ rows, selectable = false, caption }: { rows: Claim[]; selectable?: boolean; caption: string }) {
  return (
    <Table aria-label={caption}>
      <TableHeader>
        <TableRow>
          {selectable && (
            <TableHead className="w-10">
              <Checkbox aria-label="Select all claims" />
            </TableHead>
          )}
          <TableHead>Claim</TableHead>
          <TableHead>Employee</TableHead>
          <TableHead>Plan</TableHead>
          <TableHead>Submitted</TableHead>
          <TableHead className="text-right">Amount (EUR)</TableHead>
          <TableHead>Status</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {rows.map((c) => (
          <TableRow key={c.id}>
            {selectable && (
              <TableCell>
                <Checkbox aria-label={`Select ${c.id}`} />
              </TableCell>
            )}
            <TableCell className="font-medium">
              <a href={`#${c.id}`} className="text-primary underline-offset-4 hover:underline">
                {c.id}
              </a>
            </TableCell>
            <TableCell>{c.employee}</TableCell>
            <TableCell>{c.plan}</TableCell>
            <TableCell className="tabular-nums">{c.submitted}</TableCell>
            <TableCell className="text-right tabular-nums">{c.amount.toFixed(2)}</TableCell>
            <TableCell>
              <Badge variant={statusVariant[c.status]}>{c.status}</Badge>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

function Field({ id, label, className, children }: { id: string; label: string; className?: string; children: ReactNode }) {
  return (
    <div className={`grid gap-1.5 ${className ?? ""}`}>
      <Label htmlFor={id}>{label}</Label>
      {children}
    </div>
  )
}

function PlanSelect({ id, defaultValue = "all" }: { id: string; defaultValue?: string }) {
  return (
    <Select defaultValue={defaultValue}>
      <SelectTrigger id={id} className="w-40">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value="all">All plans</SelectItem>
        <SelectItem value="dental">Dental</SelectItem>
        <SelectItem value="vision">Vision</SelectItem>
        <SelectItem value="wellness">Wellness</SelectItem>
      </SelectContent>
    </Select>
  )
}

const claimsBreadcrumb = (current?: string) => (
  <Breadcrumb>
    <BreadcrumbList>
      <BreadcrumbItem>
        <BreadcrumbLink href="#home">Home</BreadcrumbLink>
      </BreadcrumbItem>
      <BreadcrumbSeparator />
      <BreadcrumbItem>
        {current ? <BreadcrumbLink href="#claims">Claims</BreadcrumbLink> : <BreadcrumbPage>Claims</BreadcrumbPage>}
      </BreadcrumbItem>
      {current && (
        <>
          <BreadcrumbSeparator />
          <BreadcrumbItem>
            <BreadcrumbPage>{current}</BreadcrumbPage>
          </BreadcrumbItem>
        </>
      )}
    </BreadcrumbList>
  </Breadcrumb>
)

// ------------------------------------------------------------------ 1. Initial page
export const InitialPageFloorplan: Story = {
  name: "1. Initial page",
  render: () => (
    <Frame>
      <InitialPage
        title="Open a claim"
        description="Enter a claim number to review it."
        aside={
          <section aria-labelledby="recent" className="grid gap-2">
            <h2 id="recent" className="text-sm font-semibold text-muted-foreground">
              Recently opened
            </h2>
            <ul className="divide-y rounded-xl border bg-card">
              {claims.slice(0, 3).map((c) => (
                <li key={c.id} className="flex h-row items-center gap-stack px-control-x">
                  <a href={`#${c.id}`} className="font-medium text-primary underline-offset-4 hover:underline">
                    {c.id}
                  </a>
                  <span className="text-muted-foreground">{c.employee}</span>
                  <Badge className="ml-auto" variant={statusVariant[c.status]}>
                    {c.status}
                  </Badge>
                </li>
              ))}
            </ul>
          </section>
        }
      >
        <form className="flex items-end gap-stack" onSubmit={(e) => e.preventDefault()}>
          <Field id="claim-id" label="Claim number" className="flex-1">
            <Input id="claim-id" placeholder="C-0000" className="w-full" />
          </Field>
          <Button type="submit">
            Open <ArrowRightIcon aria-hidden />
          </Button>
        </form>
      </InitialPage>
    </Frame>
  ),
}

// ------------------------------------------------------------------ 2. List report
export const ListReportFloorplan: Story = {
  name: "2. List report",
  render: () => (
    <Frame>
      <ListReport
        breadcrumb={claimsBreadcrumb()}
        title="Claims"
        actions={
          <Button>
            <PlusIcon aria-hidden /> Create claim
          </Button>
        }
        filters={
          <>
            <Field id="f-search" label="Search">
              <Input id="f-search" type="search" placeholder="Claim or employee" className="w-56" />
            </Field>
            <Field id="f-plan" label="Plan">
              <PlanSelect id="f-plan" />
            </Field>
            <Field id="f-from" label="Submitted from">
              <Input id="f-from" type="date" defaultValue="2026-09-01" className="w-44" />
            </Field>
          </>
        }
        tableTitle={`Claims (${claims.length})`}
        tableActions={
          <>
            <Button variant="outline">
              <DownloadSimpleIcon aria-hidden /> Export
            </Button>
            <Button variant="outline">Delete</Button>
          </>
        }
      >
        <ClaimsTable rows={claims} selectable caption="Claims" />
      </ListReport>
    </Frame>
  ),
}

// ------------------------------------------------------------------ 3. Worklist
function WorklistDemo() {
  const [tab, setTab] = useState("review")
  const byTab: Record<string, Claim[]> = {
    review: claims.filter((c) => c.status === "In review"),
    overdue: claims.filter((c) => c.status === "Overdue"),
    all: claims,
  }
  return (
    <Frame>
      <Worklist
        title="My approvals"
        tabs={[
          { value: "review", label: "To review", count: byTab.review.length },
          { value: "overdue", label: "Overdue", count: byTab.overdue.length },
          { value: "all", label: "All", count: byTab.all.length },
        ]}
        value={tab}
        onValueChange={setTab}
        toolbar={
          <>
            <Button>Approve</Button>
            <Button variant="outline">Reject</Button>
          </>
        }
      >
        <ClaimsTable rows={byTab[tab]} selectable caption="Claims to process" />
      </Worklist>
    </Frame>
  )
}

export const WorklistFloorplan: Story = { name: "3. Worklist", render: () => <WorklistDemo /> }

// ------------------------------------------------------------------ 4. Object page
export const ObjectPageFloorplan: Story = {
  name: "4. Object page",
  render: () => (
    <Frame>
      <ObjectPage
        breadcrumb={claimsBreadcrumb("C-1042")}
        title="Claim C-1042"
        subtitle="Dental · Asha Rao"
        status={<Badge variant="informative">In review</Badge>}
        actions={
          <>
            <Button>Approve</Button>
            <Button variant="outline">Reject</Button>
            <Button variant="ghost" size="icon" aria-label="Edit claim">
              <PencilSimpleIcon aria-hidden />
            </Button>
          </>
        }
        facts={[
          { label: "Amount", value: "180.00 EUR" },
          { label: "Submitted", value: "3 October 2026" },
          { label: "Reviewer", value: "Jonas Meyer" },
          { label: "Annual limit used", value: "64 %" },
        ]}
        sections={[
          {
            id: "general",
            title: "General information",
            content: (
              <Card>
                <CardContent>
                  <dl className="grid gap-x-section gap-y-stack sm:grid-cols-2 lg:grid-cols-3">
                    {[
                      ["Plan", "Dental"],
                      ["Provider", "Northside Dental Clinic"],
                      ["Treatment date", "28 September 2026"],
                      ["Payment method", "Payroll"],
                      ["Currency", "EUR"],
                      ["Receipts", "2 attached"],
                    ].map(([k, v]) => (
                      <div key={k} className="grid gap-0.5">
                        <dt className="text-xs text-muted-foreground">{k}</dt>
                        <dd>{v}</dd>
                      </div>
                    ))}
                  </dl>
                </CardContent>
              </Card>
            ),
          },
          {
            id: "items",
            title: "Items",
            content: (
              <div className="rounded-xl border bg-card">
                <Table aria-label="Claim items">
                  <TableHeader>
                    <TableRow>
                      <TableHead>Item</TableHead>
                      <TableHead className="text-right">Amount (EUR)</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    <TableRow>
                      <TableCell>Cleaning</TableCell>
                      <TableCell className="text-right tabular-nums">95.00</TableCell>
                    </TableRow>
                    <TableRow>
                      <TableCell>X-rays</TableCell>
                      <TableCell className="text-right tabular-nums">85.00</TableCell>
                    </TableRow>
                  </TableBody>
                </Table>
              </div>
            ),
          },
          { id: "history", title: "History", content: <p>Submitted 3 October 2026. Assigned to Jonas Meyer.</p> },
        ]}
      />
    </Frame>
  ),
}

// ------------------------------------------------------------------ 5. Overview page
export const OverviewPageFloorplan: Story = {
  name: "5. Overview page",
  render: () => (
    <Frame>
      <OverviewPage
        title="Benefits overview"
        subtitle="Plan year 2026"
        filters={
          <div className="flex flex-wrap items-end gap-stack">
            <Field id="o-plan" label="Plan">
              <PlanSelect id="o-plan" />
            </Field>
          </div>
        }
      >
        <Card>
          <CardHeader>
            <CardTitle>
              <h2>Claims awaiting review</h2>
            </CardTitle>
            <CardDescription>Across all plans</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-1">
            <p className="text-3xl font-semibold tabular-nums">14</p>
            <p className="text-sm text-critical">3 overdue</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>
              <h2>Annual limit used</h2>
            </CardTitle>
            <CardDescription>Share of the budget per plan</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-stack">
            {[
              ["Dental", 64],
              ["Vision", 41],
              ["Wellness", 87],
            ].map(([plan, value]) => (
              <div key={plan} className="grid gap-1">
                <div className="flex justify-between text-sm">
                  <span id={`use-${plan}`}>{plan}</span>
                  <span className="tabular-nums">{value} %</span>
                </div>
                <Progress value={value as number} aria-labelledby={`use-${plan}`} />
              </div>
            ))}
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>
              <h2>Latest claims</h2>
            </CardTitle>
            <CardDescription>Most recent first</CardDescription>
          </CardHeader>
          <CardContent>
            <ul className="divide-y">
              {claims.slice(0, 4).map((c) => (
                <li key={c.id} className="flex h-row items-center gap-stack">
                  <span className="font-medium">{c.id}</span>
                  <span className="truncate text-muted-foreground">{c.employee}</span>
                  <Badge className="ml-auto" variant={statusVariant[c.status]}>
                    {c.status}
                  </Badge>
                </li>
              ))}
            </ul>
          </CardContent>
        </Card>
      </OverviewPage>
    </Frame>
  ),
}

// ------------------------------------------------------------------ 6. Wizard
function WizardDemo({ initialStep = 0 }: { initialStep?: number }) {
  const [step, setStep] = useState(initialStep)
  const steps = [
    { id: "plan", title: "Plan" },
    { id: "details", title: "Details" },
    { id: "receipts", title: "Receipts" },
    { id: "review", title: "Review" },
  ]
  return (
    <Frame>
      <Wizard title="New claim" steps={steps} current={step} onStepChange={setStep} onCancel={() => setStep(0)}>
        {step === 0 && (
          <Field id="w-plan" label="Plan">
            <PlanSelect id="w-plan" defaultValue="dental" />
          </Field>
        )}
        {step === 1 && (
          <div className="grid gap-stack sm:grid-cols-2">
            <Field id="w-date" label="Treatment date">
              <Input id="w-date" type="date" defaultValue="2026-09-28" />
            </Field>
            <Field id="w-amount" label="Amount (EUR)">
              <Input id="w-amount" inputMode="decimal" defaultValue="180.00" />
            </Field>
            <div className="sm:col-span-2">
              <Field id="w-notes" label="Notes for the reviewer">
                <Textarea id="w-notes" placeholder="Optional" />
              </Field>
            </div>
          </div>
        )}
        {step === 2 && <p>Attach itemised receipts (PDF or image).</p>}
        {step === 3 && (
          <Card>
            <CardContent>
              <KeyFacts
                facts={[
                  { label: "Plan", value: "Dental" },
                  { label: "Treatment date", value: "28 September 2026" },
                  { label: "Amount", value: "180.00 EUR" },
                ]}
              />
            </CardContent>
          </Card>
        )}
      </Wizard>
    </Frame>
  )
}

export const WizardFloorplan: Story = { name: "6. Wizard", render: () => <WizardDemo initialStep={1} /> }

// ------------------------------------------------------------------ 7. Flexible column layout
function ColumnHeader({ title, onClose }: { title: string; onClose?: () => void }) {
  return (
    <div className="flex h-row items-center gap-2 border-b px-control-x">
      <h2 className="font-semibold">{title}</h2>
      {onClose && (
        <Button variant="ghost" size="icon" className="ml-auto" aria-label={`Close ${title}`} onClick={onClose}>
          <XIcon aria-hidden />
        </Button>
      )}
    </div>
  )
}

function FlexibleColumnDemo() {
  const [selected, setSelected] = useState<Claim | undefined>(claims[0])
  const [itemOpen, setItemOpen] = useState(true)
  return (
    <Frame>
      <FlexibleColumnLayout
        begin={{
          label: "Claims",
          content: (
            <>
              <ColumnHeader title="Claims" />
              <ul>
                {claims.map((c) => (
                  <li key={c.id}>
                    <button
                      type="button"
                      aria-current={selected?.id === c.id ? "true" : undefined}
                      onClick={() => setSelected(c)}
                      className="flex min-h-row w-full items-center gap-stack border-b px-control-x py-2 text-left hover:bg-list-hover aria-[current=true]:bg-list-selected"
                    >
                      <span className="grid">
                        <span className="font-medium">{c.id}</span>
                        <span className="text-xs text-muted-foreground">{c.employee}</span>
                      </span>
                      <Badge className="ml-auto" variant={statusVariant[c.status]}>
                        {c.status}
                      </Badge>
                    </button>
                  </li>
                ))}
              </ul>
            </>
          ),
        }}
        mid={
          selected && {
            label: `Claim ${selected.id}`,
            content: (
              <>
                <ColumnHeader title={`Claim ${selected.id}`} onClose={() => setSelected(undefined)} />
                <div className="grid gap-section p-section">
                  <KeyFacts
                    facts={[
                      { label: "Employee", value: selected.employee },
                      { label: "Plan", value: selected.plan },
                      { label: "Amount", value: `${selected.amount.toFixed(2)} EUR` },
                      { label: "Status", value: <Badge variant={statusVariant[selected.status]}>{selected.status}</Badge> },
                    ]}
                  />
                  <Button variant="outline" className="w-fit" onClick={() => setItemOpen(true)}>
                    Show receipt
                  </Button>
                </div>
              </>
            ),
          }
        }
        end={
          selected && itemOpen
            ? {
                label: "Receipt",
                content: (
                  <>
                    <ColumnHeader title="Receipt" onClose={() => setItemOpen(false)} />
                    <p className="p-section text-sm text-muted-foreground">receipt-{selected.id.toLowerCase()}.pdf · 2 pages</p>
                  </>
                ),
              }
            : undefined
        }
      />
    </Frame>
  )
}

export const FlexibleColumnLayoutFloorplan: Story = {
  name: "7. Flexible column layout",
  render: () => <FlexibleColumnDemo />,
}

// ------------------------------------------------------------------ Dense variants (axe runs on these too)
export const ListReportCompact: Story = { ...ListReportFloorplan, name: "2. List report (compact)", globals: { density: "compact" } }
export const ObjectPageCompact: Story = { ...ObjectPageFloorplan, name: "4. Object page (compact)", globals: { density: "compact" } }
export const WorklistCompact: Story = { ...WorklistFloorplan, name: "3. Worklist (compact)", globals: { density: "compact" } }
