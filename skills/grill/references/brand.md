# grill brand: question set

`aayu grill brand` writes `design/brand.yaml`: the product's name, voice (`tell` writes in it, WP7.5), palette, and
the colour pairs the product puts text or controls on. Every pair is checked against WCAG 2.2 AA: text 4.5:1, large
text 3:1, user-interface components 3:1. A pair below its ratio is refused; suggest a darker or lighter shade and ask
again. Ask; do not invent colours the human did not choose.

## Questions

### Name (required)

- What is the product called, exactly as users see it?

### Voice (required)

- Which few words describe how it speaks (`plain`, `warm`, `direct`)? Which words or habits should it avoid?

### Palette (required)

- Which colours does the brand use? One per line as `name #rrggbb` (`ink #1a1a1a`, `surface #ffffff`).

### Pairs (required)

- Which colour is used on which, and for what? One per line as `fg on bg, text|large-text|ui`
  (`ink on surface, text`). Name at least one text pair.

## The YAML (`--body`)

```yaml
version: 1
name: Benefits QA
voice:
  tone: [plain, warm]
  avoid: [jargon]
palette:
  ink: "#1a1a1a"
  surface: "#ffffff"
  primary: "#0b5fff"
pairs:
  - {fg: ink, bg: surface, use: text}
  - {fg: primary, bg: surface, use: ui}
```
