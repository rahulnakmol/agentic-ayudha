import type { Meta, StoryObj } from "@storybook/react-vite"
import { FloppyDiskIcon, PlusIcon, TrashIcon } from "@phosphor-icons/react"

import { Button } from "../../components/ui/button"

const meta = {
  title: "Primitives/Button",
  component: Button,
  args: { children: "Save" },
  argTypes: {
    variant: { control: "select", options: ["default", "secondary", "outline", "ghost", "destructive", "link"] },
    size: { control: "select", options: ["default", "sm", "lg", "icon"] },
  },
} satisfies Meta<typeof Button>

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}

export const Variants: Story = {
  render: () => (
    <div className="flex flex-wrap items-center gap-stack">
      <Button>
        <FloppyDiskIcon aria-hidden /> Save
      </Button>
      <Button variant="secondary">
        <PlusIcon aria-hidden /> Create
      </Button>
      <Button variant="outline">Cancel</Button>
      <Button variant="ghost">Share</Button>
      <Button variant="destructive">
        <TrashIcon aria-hidden /> Delete
      </Button>
      <Button variant="link">Show details</Button>
      <Button size="icon" variant="outline" aria-label="Add item">
        <PlusIcon aria-hidden />
      </Button>
      <Button disabled>Disabled</Button>
    </div>
  ),
}

export const Compact: Story = { ...Variants, globals: { density: "compact" } }
