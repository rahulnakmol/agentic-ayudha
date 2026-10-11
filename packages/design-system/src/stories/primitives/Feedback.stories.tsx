import type { Meta, StoryObj } from "@storybook/react-vite"
import { InfoIcon, WarningCircleIcon } from "@phosphor-icons/react"

import { Alert, AlertDescription, AlertTitle } from "../../components/ui/alert"
import { Avatar, AvatarFallback } from "../../components/ui/avatar"
import { Progress } from "../../components/ui/progress"
import { Separator } from "../../components/ui/separator"

const meta = { title: "Primitives/Feedback and display" } satisfies Meta

export default meta
type Story = StoryObj<typeof meta>

export const Alerts: Story = {
  render: () => (
    <div className="grid max-w-lg gap-stack">
      <Alert>
        <InfoIcon aria-hidden />
        <AlertTitle>Plan year closes 31 December</AlertTitle>
        <AlertDescription>Claims submitted after that date count against next year's limit.</AlertDescription>
      </Alert>
      <Alert variant="destructive">
        <WarningCircleIcon aria-hidden />
        <AlertTitle>Receipt missing</AlertTitle>
        <AlertDescription>Attach an itemised receipt before you submit.</AlertDescription>
      </Alert>
    </div>
  ),
}

export const ProgressBar: Story = {
  render: () => (
    <div className="grid max-w-sm gap-2">
      <div className="flex justify-between text-sm">
        <span id="limit-label">Annual limit used</span>
        <span>64 %</span>
      </div>
      <Progress value={64} aria-labelledby="limit-label" />
    </div>
  ),
}

export const AvatarsAndSeparator: Story = {
  render: () => (
    <div className="flex h-10 items-center gap-stack">
      <Avatar>
        <AvatarFallback>AR</AvatarFallback>
      </Avatar>
      <span>Asha Rao</span>
      <Separator orientation="vertical" />
      <Avatar>
        <AvatarFallback>JM</AvatarFallback>
      </Avatar>
      <span>Jonas Meyer</span>
    </div>
  ),
}
