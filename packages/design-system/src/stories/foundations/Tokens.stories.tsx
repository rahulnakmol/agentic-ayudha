import type { Meta, StoryObj } from "@storybook/react-vite"

import { Button } from "../../components/ui/button"
import { Input } from "../../components/ui/input"
import { Label } from "../../components/ui/label"
import tokens from "../../../tokens/tokens.json"

type Leaf = { $type: string; $value: unknown; $description?: string }

function leaves(group: object, prefix: string[] = []): [string[], Leaf][] {
  return Object.entries(group).flatMap(([k, v]) => {
    if (k.startsWith("$") || typeof v !== "object") return []
    return "$value" in v ? [[[...prefix, k], v as Leaf]] : leaves(v, [...prefix, k])
  })
}

const meta = {
  title: "Foundations/Tokens",
  parameters: {
    docs: {
      description: {
        component:
          "Semantic tokens from tokens/tokens.json, emitted as --ay-* CSS variables and as SwiftUI and Compose constants with the same names. OpenUI5-inspired naming; no SAP assets, icons, or fonts.",
      },
    },
  },
} satisfies Meta

export default meta
type Story = StoryObj<typeof meta>

export const Colors: Story = {
  render: () => (
    <ul className="grid grid-cols-[repeat(auto-fill,minmax(14rem,1fr))] gap-stack">
      {leaves(tokens.color, ["color"]).map(([path, t]) => (
        <li key={path.join("-")} className="flex items-center gap-3 rounded-lg border bg-card p-2">
          <span
            aria-hidden
            className="size-10 shrink-0 rounded-md border"
            style={{ background: `var(--ay-${path.join("-")})` }}
          />
          <span className="min-w-0">
            <code className="block truncate font-mono text-xs">--ay-{path.join("-")}</code>
            <span className="text-xs text-muted-foreground">{String(t.$value)}</span>
          </span>
        </li>
      ))}
    </ul>
  ),
}

export const Typography: Story = {
  render: () => (
    <div className="grid gap-stack">
      {leaves(tokens.font.size, ["font", "size"]).map(([path, t]) => (
        <p key={path.join("-")} style={{ fontSize: `var(--ay-${path.join("-")})` }}>
          {path.at(-1)} · {String(t.$value)} px · The claim was approved.
        </p>
      ))}
    </div>
  ),
}

function DensitySample({ mode }: { mode: "cozy" | "compact" }) {
  return (
    <section data-density={mode} aria-labelledby={`density-${mode}`} className="grid gap-stack rounded-xl border bg-card p-section">
      <h2 id={`density-${mode}`} className="text-base font-semibold">
        {mode === "cozy" ? "Cozy (default)" : "Compact (dense)"}
      </h2>
      <div className="grid gap-2">
        <Label htmlFor={`id-${mode}`}>Employee ID</Label>
        <Input id={`id-${mode}`} placeholder="E-000000" />
      </div>
      <div className="flex gap-stack">
        <Button>Save</Button>
        <Button variant="outline">Cancel</Button>
      </div>
      <dl className="grid grid-cols-2 gap-x-4 text-xs text-muted-foreground">
        {leaves(tokens.density[mode]).map(([path, t]) => (
          <div key={path.join("-")} className="contents">
            <dt>
              <code className="font-mono">--ay-{path.join("-")}</code>
            </dt>
            <dd>{String(t.$value)} px</dd>
          </div>
        ))}
      </dl>
    </section>
  )
}

/** Both density modes side by side. Set data-density="compact" on any ancestor to switch. */
export const Density: Story = {
  render: () => (
    <div className="grid gap-section md:grid-cols-2">
      <DensitySample mode="cozy" />
      <DensitySample mode="compact" />
    </div>
  ),
}
