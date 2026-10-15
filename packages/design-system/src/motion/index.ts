// Motion helpers driven by the motion tokens. GSAP is a peerDependency of this package and is never
// bundled or vendored here (docs/BUILD-PLAN.md WP6.1, Appendix A J3-19): the consuming app installs it.
// Animations are written in code with these helpers; nothing in this package is a visual animation builder.
import { gsap } from "gsap"

import tokens from "../../tokens/tokens.json"

export type DurationToken = keyof typeof tokens.motion.duration
export type EasingToken = keyof typeof tokens.motion.easing
export type MotionPreset = "fade-in" | "slide-up" | "slide-in-end" | "expand"

export const durationTokens = Object.keys(tokens.motion.duration) as DurationToken[]
export const easingTokens = Object.keys(tokens.motion.easing) as EasingToken[]
export const motionPresets: MotionPreset[] = ["fade-in", "slide-up", "slide-in-end", "expand"]

/** Duration token in seconds (GSAP's unit). */
export function duration(name: DurationToken): number {
  return tokens.motion.duration[name].$value / 1000
}

/** CSS cubic-bezier(x1, y1, x2, y2) as an easing function, so GSAP uses the same curve as CSS and native. */
export function cubicBezier(x1: number, y1: number, x2: number, y2: number): (x: number) => number {
  const cx = 3 * x1, bx = 3 * (x2 - x1) - cx, ax = 1 - cx - bx
  const cy = 3 * y1, by = 3 * (y2 - y1) - cy, ay = 1 - cy - by
  const sampleX = (t: number) => ((ax * t + bx) * t + cx) * t
  const sampleY = (t: number) => ((ay * t + by) * t + cy) * t
  const slopeX = (t: number) => (3 * ax * t + 2 * bx) * t + cx
  return (x) => {
    if (x <= 0) return 0
    if (x >= 1) return 1
    // Newton's method, then bisection when the slope is too flat to converge.
    let t = x
    for (let i = 0; i < 8; i++) {
      const err = sampleX(t) - x
      if (Math.abs(err) < 1e-7) return sampleY(t)
      const d = slopeX(t)
      if (Math.abs(d) < 1e-6) break
      t -= err / d
    }
    let lo = 0, hi = 1
    t = x
    while (hi - lo > 1e-7) {
      if (sampleX(t) < x) lo = t
      else hi = t
      t = (lo + hi) / 2
    }
    return sampleY(t)
  }
}

/** Easing token as a GSAP ease function. */
export function easing(name: EasingToken): (x: number) => number {
  const [x1, y1, x2, y2] = tokens.motion.easing[name].$value
  return cubicBezier(x1, y1, x2, y2)
}

export function prefersReducedMotion(): boolean {
  return typeof window !== "undefined" && window.matchMedia("(prefers-reduced-motion: reduce)").matches
}

const presets: Record<MotionPreset, { from: gsap.TweenVars; to: gsap.TweenVars }> = {
  "fade-in": { from: { opacity: 0 }, to: { opacity: 1 } },
  "slide-up": { from: { opacity: 0, y: 16 }, to: { opacity: 1, y: 0 } },
  "slide-in-end": { from: { opacity: 0, x: 24 }, to: { opacity: 1, x: 0 } },
  expand: { from: { opacity: 0, scale: 0.96 }, to: { opacity: 1, scale: 1 } },
}

export type AnimateOptions = { duration?: DurationToken; easing?: EasingToken; stagger?: number }

/**
 * Plays a preset on the target(s) with token timing. With prefers-reduced-motion the end state is applied
 * immediately (duration 0), so content is never hidden.
 */
export function animate(
  target: gsap.TweenTarget,
  preset: MotionPreset,
  { duration: d = "medium", easing: e = "enter", stagger }: AnimateOptions = {},
): gsap.core.Tween {
  const { from, to } = presets[preset]
  const reduced = prefersReducedMotion()
  return gsap.fromTo(target, from, {
    ...to,
    duration: reduced ? 0 : duration(d),
    ease: easing(e),
    stagger: reduced ? 0 : stagger,
  })
}
