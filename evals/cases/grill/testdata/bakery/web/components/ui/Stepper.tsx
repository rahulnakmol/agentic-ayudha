export function Stepper({ step, total }: { step: number; total: number }) {
  return <p className="crumb-stepper">Step {step} of {total}</p>;
}
