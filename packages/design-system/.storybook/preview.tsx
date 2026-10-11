import type { Decorator, Preview } from "@storybook/react-vite"

import "../src/styles/index.css"

// Density is set on <html> so portalled content (dialogs, select menus) follows it too.
const withDensity: Decorator = (Story, { globals, parameters, viewMode }) => {
  document.documentElement.dataset.density = globals.density ?? "cozy"
  const height = viewMode === "docs" ? "" : "min-h-screen"
  return (
    <div className={`${height} bg-page ${parameters.fullBleed ? "" : "p-page"}`}>
      <Story />
    </div>
  )
}

const preview: Preview = {
  tags: ["autodocs"],
  globalTypes: {
    density: {
      description: "Density mode (data-density)",
      toolbar: {
        title: "Density",
        icon: "component",
        items: [
          { value: "cozy", title: "Cozy (default, touch)" },
          { value: "compact", title: "Compact (dense, pointer)" },
        ],
        dynamicTitle: true,
      },
    },
  },
  initialGlobals: { density: "cozy" },
  decorators: [withDensity],
  parameters: {
    // Padding comes from the density token --ay-space-page; full-page stories set parameters.fullBleed.
    layout: "fullscreen",
    // axe runs on every story; any violation fails the story's test (pnpm test:storybook) and shows in the
    // Accessibility panel.
    a11y: { test: "error" },
    controls: { matchers: { color: /(background|color)$/i } },
  },
}

export default preview
