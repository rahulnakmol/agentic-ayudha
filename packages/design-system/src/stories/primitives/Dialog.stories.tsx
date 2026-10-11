import type { Meta, StoryObj } from "@storybook/react-vite"
import { expect, userEvent, within } from "storybook/test"

import { Button } from "../../components/ui/button"
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "../../components/ui/dialog"

const meta = { title: "Primitives/Dialog", component: Dialog } satisfies Meta<typeof Dialog>

export default meta
type Story = StoryObj<typeof meta>

/** Opened by the play function so axe also checks the open dialog. */
export const Open: Story = {
  render: () => (
    <Dialog>
      <DialogTrigger asChild>
        <Button variant="destructive">Withdraw claim</Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Withdraw claim C-1042?</DialogTitle>
          <DialogDescription>The reviewer is notified. You can submit a new claim later.</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <DialogClose asChild>
            <Button variant="outline">Keep claim</Button>
          </DialogClose>
          <Button variant="destructive">Withdraw</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  ),
  play: async ({ canvasElement }) => {
    await userEvent.click(within(canvasElement).getByRole("button", { name: "Withdraw claim" }))
    const dialog = await within(document.body).findByRole("dialog")
    await expect(dialog).toHaveAccessibleName("Withdraw claim C-1042?")
  },
}
