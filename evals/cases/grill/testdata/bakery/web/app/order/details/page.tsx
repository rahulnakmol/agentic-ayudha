"use client";
import { useState } from "react";
import { Stepper } from "../../../components/ui/Stepper";
import { Button } from "../../../components/ui/Button";

export default function Details() {
  const [error, setError] = useState("");
  return (
    <main>
      <Stepper step={4} total={5} />
      <h1>Your details</h1>
      <input aria-label="Phone" onBlur={(e) => setError(e.target.value ? "" : "We need a phone number")} />
      {error && <p role="alert">{error}</p>}
      <Button>Continue</Button>
    </main>
  );
}
