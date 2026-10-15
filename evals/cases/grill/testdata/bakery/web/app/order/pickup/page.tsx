import { Stepper } from "../../../components/ui/Stepper";
import { Button } from "../../../components/ui/Button";

export default function PickupTime() {
  return (
    <main>
      <Stepper step={3} total={5} />
      <h1>When will you collect?</h1>
      <Button>Continue</Button>
    </main>
  );
}
