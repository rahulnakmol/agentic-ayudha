# Baseline the release candidate 4f2c1a9 on the preview target (never production), then return to dev.
git fetch origin
git switch --detach 4f2c1a9
pnpm install --frozen-lockfile
# Start the app on the preview target in the background and wait for it to answer.
AAYU_ENV=preview pnpm start > /tmp/shop-preview.log 2>&1 &
npx --yes wait-on http://localhost:3000
AAYU_ENV=preview aayu baseline --sha 4f2c1a9
kill %1
git switch dev
