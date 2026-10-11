import type { Meta, StoryObj } from "@storybook/react-vite"

import { Badge } from "../../components/ui/badge"
import { Button } from "../../components/ui/button"
import { Card, CardAction, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "../../components/ui/card"

const meta = { title: "Primitives/Card", component: Card } satisfies Meta<typeof Card>

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  render: () => (
    <Card className="max-w-sm">
      <CardHeader>
        <CardTitle>Dental claim C-1042</CardTitle>
        <CardDescription>Submitted 3 October by Asha Rao</CardDescription>
        <CardAction>
          <Badge variant="informative">In review</Badge>
        </CardAction>
      </CardHeader>
      <CardContent>
        <p>Routine cleaning and X-rays. Amount claimed 180.00 EUR against an annual limit of 1,500.00 EUR.</p>
      </CardContent>
      <CardFooter className="gap-stack">
        <Button>Approve</Button>
        <Button variant="outline">Request info</Button>
      </CardFooter>
    </Card>
  ),
}

export const Compact: Story = { ...Default, globals: { density: "compact" } }
