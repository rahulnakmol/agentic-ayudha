#!/bin/sh
# Builds the benefits-qa example and starts it detached on its embedded, synthetic seed data (docs/BUILD-PLAN.md
# WP8.1, V3), then waits until /healthz answers. aayudha's `[verify] seed` runs it before the web floors capture;
# run it by hand to look at the app. BQA_PORT (default 8787) picks the port; a second run replaces the first.
set -eu
root=$(cd "$(dirname "$0")/../../.." && pwd)
port=${BQA_PORT:-8787}
run="${TMPDIR:-/tmp}/benefits-qa-$port"
mkdir -p "$run"
# Replace the instance this script started on this port, and only that one: the pid in this run directory must be
# the pid the app on the port reports on /healthz (so a reused pid is never signalled). Then wait until it has
# exited so the new one can bind the port.
if [ -f "$run/pid" ]; then
  old=$(cat "$run/pid")
  live=$(curl -fsS -D - -o /dev/null "http://127.0.0.1:$port/healthz" 2>/dev/null | tr -d '\r' |
    sed -n 's/^[Xx]-[Bb]enefits-[Qq]a-[Pp]id: *//p')
  if [ "$live" = "$old" ]; then
    kill "$old" 2>/dev/null || true
  fi
  i=0
  while kill -0 "$old" 2>/dev/null && [ "$i" -lt 50 ]; do
    i=$((i + 1))
    sleep 0.1
  done
  rm -f "$run/pid"
fi
cd "$root"
CGO_ENABLED=0 go build -o "$run/benefits-qa" ./examples/benefits-qa/cmd/benefits-qa
nohup "$run/benefits-qa" -addr "127.0.0.1:$port" -tokens "$root/packages/design-system/src/styles/tokens.css" \
  >"$run/log" 2>&1 </dev/null &
echo $! >"$run/pid"
i=0
while [ "$i" -lt 100 ]; do
  if ! kill -0 "$(cat "$run/pid")" 2>/dev/null; then
    break # it exited (the port may be taken); the log says why
  fi
  if curl -fsS "http://127.0.0.1:$port/healthz" >/dev/null 2>&1; then
    echo "benefits-qa: http://127.0.0.1:$port (pid $(cat "$run/pid"), log $run/log)"
    exit 0
  fi
  i=$((i + 1))
  sleep 0.1
done
echo "benefits-qa did not answer /healthz on port $port:" >&2
cat "$run/log" >&2
exit 1
