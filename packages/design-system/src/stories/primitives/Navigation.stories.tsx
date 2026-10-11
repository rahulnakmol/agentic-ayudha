import type { Meta, StoryObj } from "@storybook/react-vite"

import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "../../components/ui/breadcrumb"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "../../components/ui/tabs"

const meta = { title: "Primitives/Navigation" } satisfies Meta

export default meta
type Story = StoryObj<typeof meta>

export const BreadcrumbTrail: Story = {
  render: () => (
    <Breadcrumb>
      <BreadcrumbList>
        <BreadcrumbItem>
          <BreadcrumbLink href="#claims">Claims</BreadcrumbLink>
        </BreadcrumbItem>
        <BreadcrumbSeparator />
        <BreadcrumbItem>
          <BreadcrumbLink href="#dental">Dental</BreadcrumbLink>
        </BreadcrumbItem>
        <BreadcrumbSeparator />
        <BreadcrumbItem>
          <BreadcrumbPage>C-1042</BreadcrumbPage>
        </BreadcrumbItem>
      </BreadcrumbList>
    </Breadcrumb>
  ),
}

export const TabsDefault: Story = {
  render: () => (
    <Tabs defaultValue="general" className="max-w-lg">
      <TabsList>
        <TabsTrigger value="general">General</TabsTrigger>
        <TabsTrigger value="items">Items</TabsTrigger>
        <TabsTrigger value="history">History</TabsTrigger>
      </TabsList>
      <TabsContent value="general">General information about the claim.</TabsContent>
      <TabsContent value="items">Line items.</TabsContent>
      <TabsContent value="history">Status changes.</TabsContent>
    </Tabs>
  ),
}

/** The line variant is the anchor bar used by the object page floorplan. */
export const TabsLine: Story = {
  render: () => (
    <Tabs defaultValue="general" className="max-w-lg">
      <TabsList variant="line">
        <TabsTrigger value="general">General</TabsTrigger>
        <TabsTrigger value="items">Items</TabsTrigger>
        <TabsTrigger value="history">History</TabsTrigger>
      </TabsList>
      <TabsContent value="general">General information about the claim.</TabsContent>
      <TabsContent value="items">Line items.</TabsContent>
      <TabsContent value="history">Status changes.</TabsContent>
    </Tabs>
  ),
}
