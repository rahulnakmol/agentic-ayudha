# The refusal is right: the checkout is 5d1e0b2, not 77aa31c. Baseline 77aa31c itself, in its own worktree, so the
# developer's uncommitted filter work stays where it is.
git fetch origin --tags
git worktree add --detach ../shop-v0.3.0 77aa31c
cd ../shop-v0.3.0
pnpm install --frozen-lockfile
AAYU_ENV=staging pnpm start > /tmp/shop-staging.log 2>&1 &
npx --yes wait-on http://localhost:4000
AAYU_ENV=staging aayu baseline --sha 77aa31c
kill %1
cd -
git worktree remove ../shop-v0.3.0
