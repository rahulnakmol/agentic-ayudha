# Baselines for 77aa31c (v0.3.0)

The earlier run was refused correctly: the checkout was at 5d1e0b2 (your `feature/orders-filter` branch), and a
baseline must be captured from the commit it names. Running it against 5d1e0b2 would have recorded your unfinished
filter work as v0.3.0's baseline.

What I did instead:

- Checked out **77aa31c** (tag v0.3.0) in a separate worktree, `../shop-v0.3.0`, so your uncommitted changes to
  `src/app/orders/page.tsx` and the new `src/orders/StatusFilter.tsx` were not touched.
- Ran the app on the **staging** target (`http://localhost:4000`) and `aayu baseline --sha 77aa31c` for `/` and
  `/orders`, then removed the worktree.
- The manifest is `baselines/77aa31c/manifest.json` on the ledger.

One thing to know: `o/shop` is public and `[proof] media` is not set, so only the screenshots' SHA-256 hashes are
kept, not the images. Later comparisons can tell that a page changed, but there is no stored picture of v0.3.0 to look
at. To keep the images, a maintainer sets `[proof] media` to a bucket and runs the baseline again.
