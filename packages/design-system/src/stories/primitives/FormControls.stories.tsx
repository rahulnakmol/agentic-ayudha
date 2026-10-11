import type { Meta, StoryObj } from "@storybook/react-vite"

import { Checkbox } from "../../components/ui/checkbox"
import { Input } from "../../components/ui/input"
import { Label } from "../../components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/select"
import { Switch } from "../../components/ui/switch"
import { Textarea } from "../../components/ui/textarea"

const meta = { title: "Primitives/Form controls" } satisfies Meta

export default meta
type Story = StoryObj<typeof meta>

function InputDemo() {
  return (
    <div className="grid max-w-sm gap-2">
      <Label htmlFor="employee">Employee ID</Label>
      <Input id="employee" placeholder="E-000000" />
    </div>
  )
}

export const InvalidInput: Story = {
  render: () => (
    <div className="grid max-w-sm gap-2">
      <Label htmlFor="amount">Amount (EUR)</Label>
      <Input id="amount" defaultValue="-20" aria-invalid aria-describedby="amount-error" />
      <p id="amount-error" className="text-xs text-negative">
        Enter an amount greater than zero.
      </p>
    </div>
  ),
}

export const TextareaField: Story = {
  render: () => (
    <div className="grid max-w-sm gap-2">
      <Label htmlFor="notes">Notes</Label>
      <Textarea id="notes" placeholder="Add context for the reviewer" />
    </div>
  ),
}

function ToggleDemo() {
  return (
    <div className="grid gap-stack">
      <div className="flex items-center gap-2">
        <Checkbox id="terms" defaultChecked />
        <Label htmlFor="terms">I confirm the receipts are original</Label>
      </div>
      <div className="flex items-center gap-2">
        <Switch id="notify" />
        <Label htmlFor="notify">Email me when the status changes</Label>
      </div>
    </div>
  )
}

function SelectDemo() {
  return (
    <div className="grid max-w-sm gap-2">
      <Label htmlFor="plan">Plan</Label>
      <Select defaultValue="dental">
        <SelectTrigger id="plan" className="w-full">
          <SelectValue placeholder="Choose a plan" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="dental">Dental</SelectItem>
          <SelectItem value="vision">Vision</SelectItem>
          <SelectItem value="wellness">Wellness</SelectItem>
        </SelectContent>
      </Select>
    </div>
  )
}

export const InputField: Story = { render: () => <InputDemo /> }
export const CheckboxAndSwitch: Story = { render: () => <ToggleDemo /> }
export const SelectField: Story = { render: () => <SelectDemo /> }

/** Every control at once, in the dense mode. */
export const Compact: Story = {
  globals: { density: "compact" },
  render: () => (
    <div className="grid max-w-sm gap-stack">
      <InputDemo />
      <SelectDemo />
      <ToggleDemo />
    </div>
  ),
}
