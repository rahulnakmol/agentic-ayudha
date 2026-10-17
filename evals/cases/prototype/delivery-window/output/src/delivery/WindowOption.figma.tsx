import figma from "@figma/code-connect"
import { WindowOption } from "./WindowOption"

figma.connect(WindowOption, "https://www.figma.com/design/D3liv3ryWin/Delivery-window?node-id=41-3", {
  props: {
    available: figma.enum("Availability", { Available: true, Full: false }),
    checked: figma.boolean("Selected"),
    time: figma.string("Time"),
  },
  example: ({ available, checked }) => (
    <WindowOption window={{ id: "tue-18", day: "Tue 21 Oct", start: "18:00", end: "20:00", available }} name="window" checked={checked} onChange={() => {}} />
  ),
})
