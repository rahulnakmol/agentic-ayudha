# Baselines for 4f2c1a9

Baselines captured for the release candidate **4f2c1a9** (on `origin/rolling`):

- Checked out 4f2c1a9 (detached; dev at 9b1e7d0 was clean, so nothing was set aside) and ran the app on the
  **preview** target, `http://localhost:3000`. Production was not touched.
- `aayu baseline --sha 4f2c1a9` captured the four `[review] baseline` paths: `/`, `/orders`, `/orders?seed=empty`,
  and `/preferences`.
- The manifest is `baselines/4f2c1a9/manifest.json` on the ledger. The screenshots went to the `[proof] media`
  bucket, `s3://shop-proof-media`, since the repository is private and the bucket is set.
- Back on `dev` afterwards.

Next: these are the baselines visual checks compare against once rolling is promoted. If a path's capture failed,
`aayu baseline` would have exited non-zero and written nothing; it did not.
