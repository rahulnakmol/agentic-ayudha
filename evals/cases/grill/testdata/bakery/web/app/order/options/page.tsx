import { Stepper } from "../../../components/ui/Stepper";
import { Button } from "../../../components/ui/Button";

export default function BakeOptions() {
  return (
    <main>
      <Stepper step={2} total={5} />
      <h1>Sliced or whole?</h1>
      <Button>Continue</Button>
    </main>
  );
}
