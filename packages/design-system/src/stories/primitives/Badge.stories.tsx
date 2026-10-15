import type { Meta, StoryObj } from "@storybook/react-vite"
import { CheckCircleIcon, InfoIcon, WarningIcon, XCircleIcon } from "@phosphor-icons/react"

import { Badge } from "../../components/ui/badge"

const meta = {
  title: "Primitives/Badge",
  component: Badge,
  args: { children: "Draft" },
} satisfies Meta<typeof Badge>

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}

/** Object status: semantic state colours (positive, negative, critical, informative, neutral). */
export const ObjectStatus: Story = {
  render: () => (
    <div className="flex flex-wrap gap-stack">
      <Badge variant="positive">
        <CheckCircleIcon aria-hidden /> Approved
      </Badge>
      <Badge variant="negative">
        <XCircleIcon aria-hidden /> Rejected
      </Badge>
      <Badge variant="critical">
        <WarningIcon aria-hidden /> Overdue
      </Badge>
      <Badge variant="informative">
        <InfoIcon aria-hidden /> In review
      </Badge>
      <Badge variant="neutral">Draft</Badge>
      <Badge variant="secondary">Secondary</Badge>
      <Badge variant="outline">Outline</Badge>
    </div>
  ),
}
