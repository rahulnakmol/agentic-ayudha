import { Alert, AlertDescription, AlertTitle, Button } from "@aayudha/design-system"

export function NoOrders() {
  return (
    <section aria-labelledby="no-orders">
      <h2 id="no-orders" className="text-header-3 font-semibold">No orders yet</h2>
      <p className="mb-section text-label">Orders you place appear here, with where each delivery is.</p>
      <Button asChild className="max-sm:w-full"><a href="/">Start shopping</a></Button>
    </section>
  )
}

// No stale rows on failure (#423): the caller renders this instead of the table.
export function OrdersFailed({ onRetry, retrying }: { onRetry: () => void; retrying: boolean }) {
  return (
    <>
      <Alert variant="negative" role="alert" className="mb-section">
        <AlertTitle>We could not load your orders</AlertTitle>
        <AlertDescription>This is on our side, and your orders are safe. Try again in a moment.</AlertDescription>
      </Alert>
      <Button onClick={onRetry} disabled={retrying} className="max-sm:w-full">{retrying ? "Retrying…" : "Retry"}</Button>
    </>
  )
}
