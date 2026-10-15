# UI review fixtures (WP6.6, synthetic)

- `clean.html`: a delivery-preferences page in design-system token colours, with a visible focus indicator and
  44 px targets. Both review lanes must return `ship`.
- `planted.html`: the same page with three planted defects: no focus indicator (`outline: none` on focus),
  body text at 3.8:1 contrast (`#838383` on white), and a 32 px help button.

The pages carry no comments, so nothing in the markup tells lane A where the defects are.
