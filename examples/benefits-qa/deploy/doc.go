// Package deploy holds how the benefits-qa example is run (layer L09,
// docs/BUILD-PLAN.md WP8.1): serve.sh builds it and starts it detached on
// its synthetic seed data, the `[verify] seed` command aayudha's web floors
// run before they capture the Ask page. The package has no Go code of its
// own; its test runs the script.
package deploy
