#!/bin/sh
# The control tool of this product's verify skill (docs/BUILD-PLAN.md WP4.5b), a stub: aayu init --verify-skill
# found neither go.mod nor package.json, so it could not pick a stack. Replace this file with a control tool written
# in the product's own stack (a shell or PTY driver for a CLI, plain HTTP for an API, the test framework for a
# library), and point the map block's "control" at it. The contract aayu's behaviour floor holds it to:
#
#   <control> check
#     stdin:  {"product", "feature", "check": {...}, "out": "<dir>", "base_url": "<url>", "root": "<repository>"}
#     stdout: {"status": "passed"|"failed"|"unreachable", "reason", "artifacts": [{"path", "kind"}]}
#
# Until then every check is blocked, never passed.
echo "control.sh: no control tool yet; write one in this product's stack (see the comment at the top)" >&2
exit 2
