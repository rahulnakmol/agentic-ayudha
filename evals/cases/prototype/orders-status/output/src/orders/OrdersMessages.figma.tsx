import figma from "@figma/code-connect"
import { Alert, AlertTitle, Button } from "@aayudha/design-system"

figma.connect(Button, "https://www.figma.com/design/Ord3rsSt4tus/Orders-delivery-status?node-id=3-2", {
  props: { label: figma.string("Label") },
  example: ({ label }) => <Button>{label}</Button>,
})

figma.connect(Alert, "https://www.figma.com/design/Ord3rsSt4tus/Orders-delivery-status?node-id=3-40", {
  props: { title: figma.string("Title") },
  example: ({ title }) => <Alert variant="negative" role="alert"><AlertTitle>{title}</AlertTitle></Alert>,
})
