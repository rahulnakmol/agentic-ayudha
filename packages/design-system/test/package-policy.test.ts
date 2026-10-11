// Licensing guards from docs/BUILD-PLAN.md WP6.1 and Appendix A J3-19: GSAP is a peerDependency that the
// consuming app installs, never a dependency of this BSD-3-Clause package and never copied into it.
import { readdirSync, readFileSync, statSync } from "node:fs"
import { join } from "node:path"
import { fileURLToPath } from "node:url"
import { describe, expect, it } from "vitest"

const root = fileURLToPath(new URL("..", import.meta.url))
const pkg = JSON.parse(readFileSync(join(root, "package.json"), "utf8"))

function files(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    if (name === "node_modules" || name === "storybook-static" || name.startsWith(".")) return []
    const path = join(dir, name)
    return statSync(path).isDirectory() ? files(path) : [path]
  })
}

describe("package policy", () => {
  it("declares GSAP only as a peer dependency", () => {
    expect(pkg.peerDependencies?.gsap).toBeDefined()
    expect(pkg.dependencies?.gsap).toBeUndefined()
  })

  it("contains no copy of GSAP source", () => {
    const self = fileURLToPath(import.meta.url)
    const vendored = files(root).filter((f) => f !== self && /GreenSock|GSAP \d+\.\d+\.\d+/.test(readFileSync(f, "utf8")))
    expect(vendored).toEqual([])
  })

  it("lists GSAP, Phosphor, shadcn/ui, and the OpenUI5-inspired tokens in THIRD_PARTY_NOTICES.md", () => {
    const notices = readFileSync(join(root, "../../THIRD_PARTY_NOTICES.md"), "utf8")
    for (const name of ["GSAP", "Phosphor", "shadcn/ui", "OpenUI5"]) expect(notices).toContain(name)
  })
})
