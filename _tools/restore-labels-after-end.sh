#!/usr/bin/env bash
# Restore the status labels that `aayu mission stop 1170 --end` removed on 2026-11-07 (about 12:45 UTC).
# The end said "merged stories stay merged", but it set 16 merged Stories and Bugs to status:ready or to no status,
# and it took status:planned off 3 planned Bugs. Every "merged" row below has its squash commit on dev.
# The in-progress ones that went back to status:ready are left alone: their PRs closed, so ready is right.
#
# Run it in your own Terminal (a person's write with gh). It shows the plan first and asks y/N.
# The repository halt does not block it: the halt stops workflows and aayu verbs, not gh label edits.
set -euo pipefail
R=rahulnakmol/agentic-ayudha

# issue  label  evidence
rows=(
  "1172 status:merged c858a80 #1219"
  "1173 status:merged eb51d1b #1225"
  "1174 status:merged 322e2ac #1216"
  "1175 status:merged 4faab61 #1235"
  "1176 status:merged 188bbb0 #1213"
  "1177 status:merged 66397d3 #1237"
  "1178 status:merged bc78349 #1238"
  "1179 status:merged 197ec9d #1217"
  "1182 status:merged c64bccf #1188"
  "1193 status:merged 26537bd #1194"
  "1207 status:merged a17d469 #1224"
  "1208 status:merged 932851d #1209"
  "1210 status:merged bed5524 #1215"
  "1211 status:merged 7d3ec79 #1218"
  "1226 status:merged e39b5cb #1228"
  "1230 status:merged 0c4abee #1232"
  "1280 status:planned planned-before-the-end -"
  "1281 status:planned planned-before-the-end -"
  "1283 status:planned planned-before-the-end -"
)

echo "Dry run: what each issue has now, and what it gets."
for r in "${rows[@]}"; do
  set -- $r
  now=$(gh issue view "$1" --repo "$R" --json labels --jq '[.labels[].name|select(startswith("status:"))]|join(",")')
  echo "  #$1: ${now:-no status} -> $2   ($3 $4)"
done
read -r -p "Apply these label changes? [y/N] " ok
[ "$ok" = y ] || { echo "Nothing changed."; exit 0; }

for r in "${rows[@]}"; do
  set -- $r
  now=$(gh issue view "$1" --repo "$R" --json labels --jq '[.labels[].name|select(startswith("status:"))]|join(",")')
  args=(--add-label "$2")
  [ -n "$now" ] && [ "$now" != "$2" ] && args+=(--remove-label "$now")
  gh issue edit "$1" --repo "$R" "${args[@]}" >/dev/null && echo "  #$1 -> $2"
done
echo "Done."
