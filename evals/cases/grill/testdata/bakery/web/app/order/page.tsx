"use client";
import { Stepper } from "../../components/ui/Stepper";
import { ProductTile } from "../../components/ProductTile";
import { BasketSummary } from "../../components/BasketSummary";
import { track } from "../../../lib/analytics";

export default function ChooseItems({ basket = [] as string[] }) {
  track("order_started");
  return (
    <main>
      <Stepper step={1} total={5} />
      <h1>What would you like?</h1>
      <ProductTile name="Sourdough" price="5.00" />
      {basket.length === 0 ? <p>Your basket is empty</p> : <BasketSummary items={basket.length} total="5.00" />}
    </main>
  );
}
