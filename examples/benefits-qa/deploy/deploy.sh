#!/bin/sh
# Deploys one revision of the benefits-qa example as a production-like service (Decision #324, WP5.10): builds that
# commit, replaces the running instance, and records a deploy marker beside the service's OpenTelemetry export, so
# `aayu investigate` can tell which revision served which window. It is the command the example's ops catalog
# actions run (`deploy-benefits-qa`, and `rollback-benefits-qa`, which redeploys the previous revision); a person
# approves each run through the GitHub Environment (`aayu respond`), and nothing else calls it in production.
#
#   deploy.sh <revision>    deploy that commit
#   deploy.sh --rollback    redeploy the revision the current one replaced
#
# State lives in BQA_DEPLOY_DIR (default $TMPDIR/benefits-qa-deploy): bin/<revision>, pid, current, previous, log,
# and telemetry/ (traces.jsonl, logs.jsonl, deploys.jsonl). BQA_PORT (default 8788) and BQA_ENV (default production)
# pick the port and the environment name.
set -eu
root=$(cd "$(dirname "$0")/../../.." && pwd)
dir=${BQA_DEPLOY_DIR:-${TMPDIR:-/tmp}/benefits-qa-deploy}
port=${BQA_PORT:-8788}
env=${BQA_ENV:-production}
mkdir -p "$dir/bin" "$dir/telemetry"
current=$(cat "$dir/current" 2>/dev/null || true)

case "${1:-}" in
--rollback)
  rev=$(cat "$dir/previous" 2>/dev/null || true)
  if [ -z "$rev" ]; then
    echo "deploy.sh: nothing to roll back to: no previous revision recorded in $dir" >&2
    exit 2
  fi
  ;;
-* | "")
  echo "usage: deploy.sh <revision> | --rollback" >&2
  exit 2
  ;;
*)
  rev=$(git -C "$root" rev-parse --verify --quiet "$1^{commit}") || {
    echo "deploy.sh: $1 is not a commit in $root" >&2
    exit 2
  }
  ;;
esac

# Build the revision once, from its own tree.
if [ ! -x "$dir/bin/$rev" ]; then
  src="$dir/src-$rev"
  rm -rf "$src"
  git -C "$root" worktree add --quiet --detach "$src" "$rev"
  (cd "$src" && CGO_ENABLED=0 go build -o "$dir/bin/$rev" ./examples/benefits-qa/cmd/benefits-qa)
  git -C "$root" worktree remove --force "$src"
fi

# healthpid prints the pid of the instance answering /healthz on the port, or nothing.
healthpid() {
  curl -fsS -D - -o /dev/null "http://127.0.0.1:$port/healthz" 2>/dev/null | tr -d '\r' |
    sed -n 's/^[Xx]-[Bb]enefits-[Qq]a-[Pp]id: *//p'
}
# Stop the instance this script started, and only that one.
if [ -f "$dir/pid" ]; then
  old=$(cat "$dir/pid")
  if [ "$(healthpid)" = "$old" ]; then
    kill "$old" 2>/dev/null || true
  fi
  i=0
  while kill -0 "$old" 2>/dev/null && [ "$i" -lt 50 ]; do
    i=$((i + 1))
    sleep 0.1
  done
  rm -f "$dir/pid"
fi

nohup "$dir/bin/$rev" -addr "127.0.0.1:$port" -tokens "$root/packages/design-system/src/styles/tokens.css" \
  -telemetry "$dir/telemetry" -revision "$rev" -environment "$env" >>"$dir/log" 2>&1 </dev/null &
new=$!
echo "$new" >"$dir/pid"
i=0
while [ "$i" -lt 100 ]; do
  if ! kill -0 "$new" 2>/dev/null; then
    break
  fi
  if [ "$(healthpid)" = "$new" ]; then
    if [ "$rev" != "$current" ]; then
      printf '%s\n' "$current" >"$dir/previous"
    fi
    printf '%s\n' "$rev" >"$dir/current"
    printf '{"time":"%s","service":"benefits-qa","environment":"%s","revision":"%s","previous":"%s","actor":"%s"}\n' \
      "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$env" "$rev" "$current" "${BQA_DEPLOY_ACTOR:-deploy.sh}" >>"$dir/telemetry/deploys.jsonl"
    echo "benefits-qa $rev serving http://127.0.0.1:$port in $env (pid $new; previous ${current:-none})"
    exit 0
  fi
  i=$((i + 1))
  sleep 0.1
done
echo "benefits-qa $rev did not answer /healthz on port $port:" >&2
tail -20 "$dir/log" >&2
exit 1
