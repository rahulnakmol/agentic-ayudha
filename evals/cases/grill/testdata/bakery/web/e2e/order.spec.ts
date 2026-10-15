import { test, expect } from "@playwright/test";

test("an order reaches the review screen", async ({ page }) => {
  await page.goto("/order");
  await page.getByRole("button", { name: "Continue" }).click();
  await expect(page).toHaveURL("/order/options");
});
