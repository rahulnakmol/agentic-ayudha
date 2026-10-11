import type { Meta, StoryObj } from "@storybook/react-vite"
import { useRef } from "react"
import { PlayIcon } from "@phosphor-icons/react"

import { Button } from "../../components/ui/button"
import {
  animate,
  durationTokens,
  easingTokens,
  motionPresets,
  type DurationToken,
  type EasingToken,
  type MotionPreset,
} from "../../motion"

type Args = { preset: MotionPreset; duration: DurationToken; easing: EasingToken }

function MotionPreview({ preset, duration, easing }: Args) {
  const box = useRef<HTMLDivElement>(null)
  return (
    <div className="grid max-w-md gap-section">
      <Button className="w-fit" onClick={() => box.current && animate(box.current, preset, { duration, easing })}>
        <PlayIcon aria-hidden /> Play {preset}
      </Button>
      <div ref={box} className="grid h-32 place-items-center rounded-xl border bg-card p-section">
        <p className="text-sm">
          <code className="font-mono">animate(el, "{preset}", {"{"} duration: "{duration}", easing: "{easing}" {"}"})</code>
        </p>
      </div>
    </div>
  )
}

const meta = {
  title: "Foundations/Motion",
  component: MotionPreview,
  args: { preset: "slide-up", duration: "medium", easing: "enter" },
  argTypes: {
    preset: { control: "select", options: motionPresets },
    duration: { control: "select", options: durationTokens },
    easing: { control: "select", options: easingTokens },
  },
  parameters: {
    docs: {
      description: {
        component: [
          "Motion uses GSAP through `@aayudha/design-system/motion`. GSAP is a peerDependency: the app installs it, and this package never bundles or vendors it.",
          "**These controls are not a visual animation builder.** They only choose among the existing presets and motion tokens to preview them. They cannot create, record, edit, or export animations; animations are written in code. (GSAP's standard licence prohibits using GSAP in visual, no-code animation builders.)",
          "With `prefers-reduced-motion: reduce`, `animate()` applies the end state immediately.",
        ].join("\n\n"),
      },
    },
  },
} satisfies Meta<typeof MotionPreview>

export default meta
type Story = StoryObj<typeof meta>

export const Preview: Story = {}
