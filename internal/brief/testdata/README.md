# brief test fixtures

Synthetic, made for these tests; no real product, person, or customer data.

- `screen.png`: a screenshot of a hand-written HTML mock of a claims list (`Member A` … `Member E` are placeholders),
  standing in for a screenshot artifact of a proof pack.
- `sample/illustration.png`: a labelled fixture standing in for an image a host's image tool (Painter or Gemini)
  would make. It was drawn by hand as SVG and rasterised; CI never calls an image model.
- `sample/brief.md` and `sample/flow.diagram`: an `engineer`/`explain` draft that uses every visual kind, on the
  grounding built by `sampleGrounding` in `brief_test.go` (Feature #7, Story #12, pack P1, Result R, snapshot D1
  built from `internal/measure/testdata`).
