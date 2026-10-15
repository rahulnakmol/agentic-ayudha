export function track(event: string, props: Record<string, unknown> = {}) {
  window.dispatchEvent(new CustomEvent("crumb-analytics", { detail: { event, props } }));
}
