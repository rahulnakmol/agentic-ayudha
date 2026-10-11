import type { ReactNode } from "react"

import { Avatar, AvatarFallback } from "../components/ui/avatar"
import { cn } from "../lib/utils"

/** Application frame shared by every floorplan: shell bar (banner landmark) plus the main landmark. */
export function AppShell({
  productName,
  userInitials,
  userName,
  shellActions,
  children,
}: {
  productName: string
  userInitials: string
  userName: string
  shellActions?: ReactNode
  children: ReactNode
}) {
  return (
    <div className="flex min-h-screen flex-col bg-page">
      <header className="flex h-14 shrink-0 items-center gap-stack border-b border-shell-border bg-shell px-page">
        <span aria-hidden className="grid size-8 place-items-center rounded-md bg-primary text-xs font-bold text-primary-foreground">
          {productName.slice(0, 1)}
        </span>
        <span className="font-semibold">{productName}</span>
        <div className="ml-auto flex items-center gap-stack">
          {shellActions}
          <Avatar aria-label={`Signed in as ${userName}`} role="img">
            <AvatarFallback aria-hidden>{userInitials}</AvatarFallback>
          </Avatar>
        </div>
      </header>
      <main className="flex min-h-0 flex-1 flex-col">{children}</main>
    </div>
  )
}

/** Dynamic-page header: breadcrumb, title, status, actions, then optional facets (key facts, filter bar). */
export function PageHeader({
  breadcrumb,
  title,
  subtitle,
  status,
  actions,
  children,
  className,
}: {
  breadcrumb?: ReactNode
  title: ReactNode
  subtitle?: ReactNode
  status?: ReactNode
  actions?: ReactNode
  children?: ReactNode
  className?: string
}) {
  return (
    <div className={cn("grid gap-stack border-b bg-card px-page py-section", className)}>
      {breadcrumb}
      <div className="flex flex-wrap items-start gap-stack">
        <div className="grid min-w-0 gap-1">
          <div className="flex flex-wrap items-center gap-stack">
            <h1 className="text-2xl font-semibold">{title}</h1>
            {status}
          </div>
          {subtitle && <p className="text-muted-foreground">{subtitle}</p>}
        </div>
        {actions && <div className="ml-auto flex flex-wrap items-center gap-2">{actions}</div>}
      </div>
      {children}
    </div>
  )
}

/** Header facets: label/value pairs shown under an object or page title. */
export function KeyFacts({ facts }: { facts: { label: string; value: ReactNode }[] }) {
  return (
    <dl className="flex flex-wrap gap-x-section gap-y-stack">
      {facts.map((f) => (
        <div key={f.label} className="grid gap-0.5">
          <dt className="text-xs text-muted-foreground">{f.label}</dt>
          <dd className="font-medium">{f.value}</dd>
        </div>
      ))}
    </dl>
  )
}
