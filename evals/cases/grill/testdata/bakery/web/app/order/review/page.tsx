"use client";
import { Stepper } from "../../../components/ui/Stepper";
import { Button } from "../../../components/ui/Button";
import { BasketSummary } from "../../../components/BasketSummary";
import { submitOrder } from "../../../../lib/orders";
import { track } from "../../../../lib/analytics";

export default function Review() {
  async function pay() {
    await submitOrder([], "");
    track("order_submitted");
  }
  return (
    <main>
      <Stepper step={5} total={5} />
      <h1>Check and pay</h1>
      <BasketSummary items={2} total="9.50" />
      <Button onClick={pay}>Pay now</Button>
    </main>
  );
}
