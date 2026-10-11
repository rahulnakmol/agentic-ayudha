// WP6.3 platform parity: the CSS, SwiftUI, and Compose exports carry the same token names with the same
// values as tokens/tokens.json. Each file is parsed on its own terms (not through the generator), so a
// generator bug or a hand edit to one platform shows up as a difference.
import { readFileSync } from "node:fs"
import { describe, expect, it } from "vitest"

const read = (path: string) => readFileSync(new URL(`../${path}`, import.meta.url), "utf8")

type Value = string | number
type Tokens = Record<string, Value>

const camel = (kebab: string) => kebab.replace(/-([a-z0-9])/g, (_, c: string) => c.toUpperCase())
const rgba = (hex: string) => (hex.length === 6 ? `${hex}ff` : hex).toLowerCase()
const round = (n: number) => Math.round(n * 1e6) / 1e6
const nums = (s: string) => s.split(",").map((x) => round(Number(x.trim().replace(/f$/, "")))).join(",")

// ---------------------------------------------------------------- expected, from tokens.json
type Node = { [k: string]: Node | Value | Value[] }
function expectedFrom(group: Node, prefix: string[] = [], out: Tokens = {}): Tokens {
  for (const [key, node] of Object.entries(group)) {
    if (key.startsWith("$") || typeof node !== "object" || Array.isArray(node)) continue
    const path = [...prefix, key]
    if (!("$value" in node)) {
      expectedFrom(node, path, out)
      continue
    }
    const v = node.$value
    const name = camel(path.join("-"))
    out[name] =
      node.$type === "color" ? rgba(String(v).slice(1))
      : node.$type === "cubicBezier" ? nums((v as number[]).join(","))
      : typeof v === "number" ? v
      : String(v)
  }
  return out
}

const source = JSON.parse(read("tokens/tokens.json"))
const { density, web, ...shared } = source
const expected = {
  shared: expectedFrom(shared),
  web: expectedFrom(web),
  modes: Object.fromEntries(
    Object.keys(density)
      .filter((m) => !m.startsWith("$"))
      .map((m) => [m, expectedFrom(density[m])]),
  ),
}

// ---------------------------------------------------------------- CSS
function parseCss() {
  const css = read("src/styles/tokens.css").replace(/\/\*[\s\S]*?\*\//g, "")
  const blocks = [...css.matchAll(/([^{}]+)\{([^}]*)\}/g)].map(([, selector, body]) => ({
    selector: selector.trim(),
    decls: Object.fromEntries(
      [...body.matchAll(/--ay-([a-z0-9-]+):\s*([^;]+);/g)].map(([, name, raw]) => {
        const v = raw.trim()
        let value: Value = v
        if (/^#[0-9a-f]+$/i.test(v)) value = rgba(v.slice(1))
        else if (v.endsWith("rem")) value = round(parseFloat(v) * 16)
        else if (v.endsWith("ms")) value = parseFloat(v)
        else if (v.startsWith("cubic-bezier(")) value = nums(v.slice(13, -1))
        else if (/^\d+$/.test(v)) value = Number(v)
        return [camel(name), value]
      }),
    ) as Tokens,
  }))
  const root = blocks.find((b) => b.selector === ":root")!.decls
  const webNames = Object.keys(expected.web)
  const modes: Record<string, Tokens> = {}
  for (const b of blocks) {
    for (const [, mode] of b.selector.matchAll(/\[data-density="([a-z]+)"\]/g)) modes[mode] = b.decls
  }
  return {
    shared: Object.fromEntries(Object.entries(root).filter(([k]) => !webNames.includes(k))),
    web: Object.fromEntries(Object.entries(root).filter(([k]) => webNames.includes(k))),
    modes,
    defaultMode: blocks.find((b) => b.selector.startsWith(":root,"))?.selector.match(/data-density="([a-z]+)"/)?.[1],
  }
}

// ---------------------------------------------------------------- SwiftUI
const swiftWeights: Record<string, number> = { regular: 400, medium: 500, semibold: 600, bold: 700 }

function swiftValue(expr: string, type?: string): Value {
  let m: RegExpMatchArray | null
  if ((m = expr.match(/^Color\(ayudhaHex: 0x([0-9A-F]{6}), alpha: 0x([0-9A-F]{2})\)$/i))) return rgba(m[1] + m[2])
  if ((m = expr.match(/^UnitCurve\.bezier\(startControlPoint: UnitPoint\(x: ([\d.]+), y: ([\d.]+)\), endControlPoint: UnitPoint\(x: ([\d.]+), y: ([\d.]+)\)\)$/)))
    return nums(m.slice(1, 5).join(","))
  if ((m = expr.match(/^Font\.Weight\.(\w+)$/))) return swiftWeights[m[1]]
  if (type === "TimeInterval") return round(Number(expr) * 1000)
  if (type === "CGFloat") return Number(expr)
  throw new Error(`unparsed Swift value: ${expr}`)
}

function parseSwift() {
  const swift = read("platforms/swiftui/AyudhaTokens.swift")
  const shared: Tokens = {}
  for (const [, name, type, expr] of swift.matchAll(/^ {4}public static let (\w+)(?:: (\w+))? = (?!DensityTokens\()(.+)$/gm))
    shared[name] = swiftValue(expr, type)
  const struct = swift.match(/public struct DensityTokens[^{]*\{([\s\S]*?)\n {4}\}/)![1]
  const fieldTypes = Object.fromEntries([...struct.matchAll(/public let (\w+): (\w+)/g)].map(([, n, t]) => [n, t]))
  const modes: Record<string, Tokens> = {}
  for (const [, mode, args] of swift.matchAll(/public static let density(\w+) = DensityTokens\((.*)\)$/gm)) {
    modes[mode.toLowerCase()] = Object.fromEntries(
      args.split(", ").map((a) => {
        const [n, v] = a.split(": ")
        return [n, swiftValue(v, fieldTypes[n])]
      }),
    )
  }
  return { shared, modes, fields: Object.keys(fieldTypes) }
}

// ---------------------------------------------------------------- Compose
function kotlinValue(expr: string): Value {
  let m: RegExpMatchArray | null
  if ((m = expr.match(/^Color\(0x([0-9A-F]{2})([0-9A-F]{6})\)$/i))) return rgba(m[2] + m[1])
  if ((m = expr.match(/^CubicBezierEasing\((.+)\)$/))) return nums(m[1])
  if ((m = expr.match(/^FontWeight\((\d+)\)$/))) return Number(m[1])
  if ((m = expr.match(/^([\d.]+)\.(dp|sp)$/))) return Number(m[1])
  if (/^\d+$/.test(expr)) return Number(expr) // durations, milliseconds
  throw new Error(`unparsed Kotlin value: ${expr}`)
}

function parseKotlin() {
  const kt = read("platforms/compose/AyudhaTokens.kt")
  const shared: Tokens = {}
  for (const [, name, expr] of kt.matchAll(/^ {4}(?:const )?val (\w+) = (?!DensityTokens\()(.+?)(?: \/\/.*)?$/gm))
    shared[name] = kotlinValue(expr)
  const dataClass = kt.match(/data class DensityTokens\(([\s\S]*?)\n {4}\)/)![1]
  const fields = [...dataClass.matchAll(/val (\w+): \w+/g)].map(([, n]) => n)
  const modes: Record<string, Tokens> = {}
  for (const [, mode, args] of kt.matchAll(/^ {4}val density(\w+) = DensityTokens\((.*)\)$/gm)) {
    modes[mode.toLowerCase()] = Object.fromEntries(
      args.split(", ").map((a) => {
        const [n, v] = a.split(" = ")
        return [n, kotlinValue(v)]
      }),
    )
  }
  return { shared, modes, fields }
}

describe("token parity (WP6.3)", () => {
  const css = parseCss()
  const swift = parseSwift()
  const kotlin = parseKotlin()

  it("has tokens and at least two density modes to compare", () => {
    expect(Object.keys(expected.shared).length).toBeGreaterThan(40)
    expect(Object.keys(expected.modes).sort()).toEqual(["compact", "cozy"])
  })

  it("exports every shared token with the same name and value on every platform", () => {
    expect(css.shared).toEqual(expected.shared)
    expect(swift.shared).toEqual(expected.shared)
    expect(kotlin.shared).toEqual(expected.shared)
  })

  it("exports every density mode with the same fields and values on every platform", () => {
    expect(css.modes).toEqual(expected.modes)
    expect(swift.modes).toEqual(expected.modes)
    expect(kotlin.modes).toEqual(expected.modes)
    const fields = Object.keys(expected.modes.cozy)
    expect(swift.fields).toEqual(fields)
    expect(kotlin.fields).toEqual(fields)
  })

  it("makes cozy the default density on the web", () => {
    expect(css.defaultMode).toBe("cozy")
  })

  it("keeps web-only tokens out of the native exports", () => {
    expect(css.web).toEqual(expected.web)
    for (const name of Object.keys(expected.web)) {
      expect(swift.shared).not.toHaveProperty(name)
      expect(kotlin.shared).not.toHaveProperty(name)
    }
  })
})
