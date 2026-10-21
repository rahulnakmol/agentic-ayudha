import figma from "@figma/code-connect"
import { Alert, AlertTitle, Button, Tabs, TabsList, TabsTrigger } from "@aayudha/design-system"

figma.connect(Tabs, "https://www.figma.com/design/D3liv3ryWin/Delivery-window?node-id=3-50", {
  example: () => (
    <Tabs value="Tue 21 Oct"><TabsList aria-label="Delivery day"><TabsTrigger value="Tue 21 Oct">Tomorrow, Tue 21 Oct</TabsTrigger></TabsList></Tabs>
  ),
})

figma.connect(Button, "https://www.figma.com/design/D3liv3ryWin/Delivery-window?node-id=3-2", {
  props: { label: figma.string("Label") },
  example: ({ label }) => <Button>{label}</Button>,
})

figma.connect(Alert, "https://www.figma.com/design/D3liv3ryWin/Delivery-window?node-id=3-44", {
  props: { title: figma.string("Title") },
  example: ({ title }) => <Alert variant="critical" role="alert"><AlertTitle>{title}</AlertTitle></Alert>,
})

figma.connect(Alert, "https://www.figma.com/design/D3liv3ryWin/Delivery-window?node-id=3-46", {
  props: { title: figma.string("Title") },
  example: ({ title }) => <Alert variant="positive" role="status"><AlertTitle>{title}</AlertTitle></Alert>,
})
