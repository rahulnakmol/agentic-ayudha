import type { Meta, StoryObj } from "@storybook/react-vite"

import { Badge } from "../../components/ui/badge"
import { Table, TableBody, TableCaption, TableCell, TableHead, TableHeader, TableRow } from "../../components/ui/table"
import { claims, statusVariant } from "../fixtures"

const meta = { title: "Primitives/Table", component: Table } satisfies Meta<typeof Table>

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  render: () => (
    <Table>
      <TableCaption>Recent claims (synthetic data)</TableCaption>
      <TableHeader>
        <TableRow>
          <TableHead>Claim</TableHead>
          <TableHead>Employee</TableHead>
          <TableHead>Plan</TableHead>
          <TableHead className="text-right">Amount (EUR)</TableHead>
          <TableHead>Status</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {claims.slice(0, 5).map((c) => (
          <TableRow key={c.id}>
            <TableCell className="font-medium">{c.id}</TableCell>
            <TableCell>{c.employee}</TableCell>
            <TableCell>{c.plan}</TableCell>
            <TableCell className="text-right tabular-nums">{c.amount.toFixed(2)}</TableCell>
            <TableCell>
              <Badge variant={statusVariant[c.status]}>{c.status}</Badge>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  ),
}

export const Compact: Story = { ...Default, globals: { density: "compact" } }
