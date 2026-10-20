# Iconography

aayudha's icons say what a thing does, in the language of the Indian epics and of classical Indian craft and science,
drawn as clean geometric lines. This note says where the icons live, the rules they follow, and how to change one.

## The two App marks

| Mark | Use | Files |
|---|---|---|
| ![आयु, the bot mark](icons/app/aayu-bot.svg) | The avatar of the bot GitHub App that agents write under. The word आयु (aayu), drawn by hand; the u-sign under य closes into a gold loop, the delivery loop. | `icons/app/aayu-bot.svg`, `icons/app/aayu-bot.png` (512 px, for GitHub) |
| ![aayu Sakshi, the witness eye](icons/app/aayu-sakshi.svg) | The logo of **aayu Sakshi**, the GitHub App behind the presence check. Sakshi (साक्षी) means witness: the eye sees that a person is here, and its pupil is the one-time code. | `icons/app/aayu-sakshi.svg`, `icons/app/aayu-sakshi.png` (512 px), `icons/app/aayu-sakshi-24.svg` (small sizes) |

GitHub takes an App's logo only as an upload on the App's settings page, so the PNGs are there for that step. Below
32 px, use `aayu-sakshi-24.svg`: it drops the four arcs and shows the code as one dot.

## Seats

Each twin's emblem is an object, place, animal, or star tied to its name, never the figure.

| Icon | Twin and seat | Emblem |
|---|---|---|
| ![Arjun](icons/seats/product-strategist.svg) | Arjun, Product Strategist | An arrow in the centre of the target |
| ![Vishwa](icons/seats/code-architect.svg) | Vishwa, Code Architect | A set-square and a plumb line |
| ![Shuka](icons/seats/data-ai-architect.svg) | Shuka, Data and AI Architect | A parrot, which is what Shuka means: it repeats its source faithfully |
| ![Drona](icons/seats/engineering-manager.svg) | Drona, Engineering Manager | A drawn bow, the teacher's test |
| ![Nakul](icons/seats/ai-native-engineer.svg) | Nakul, AI Native Engineer | A horse, for the master of horses |
| ![Shesha](icons/seats/production-engineer.svg) | Shesha, Production Engineer | An endless coil that holds the world up |
| ![Hanu](icons/seats/ux-researcher-evolutionist.svg) | Hanu, UX Researcher and Evolutionist | The leap across the sea to find what people need |
| ![Nala](icons/seats/devops-devex-tinkerer.svg) | Nala, DevOps and DevEx Tinkerer | A bridge of floating stones |
| ![Yama](icons/seats/quality-architect.svg) | Yama, Quality Architect | The gateway every change passes through |
| ![Sanjay](icons/seats/mentalist.svg) | Sanjay, Mentalist | Far sight carried into words |
| ![Vidhi](icons/seats/auditor.svg) | Vidhi, Auditor | A palm-leaf record with its seal |
| ![Sauti](icons/seats/marketer-documenter.svg) | Sauti, Marketer and Documenter | The storyteller's open book |
| ![Kuber](icons/seats/revenue-architect.svg) | Kuber, Revenue Architect | The treasury, rising |
| ![Atri](icons/seats/founder.svg) | Atri, Founder's Sage | His star in the Saptarishi (the Big Dipper), where the handle meets the bowl |

## Verbs

| Icon | Verb | What it shows |
|---|---|---|
| ![grill](icons/verbs/grill.svg) | `grill` | A chisel shaping a rough idea into a clear block |
| ![slice](icons/verbs/slice.svg) | `slice` | A blade cutting a Feature into stories |
| ![ready](icons/verbs/ready.svg) | `ready` | A raised flag with a check: the story meets the bar to start |
| ![plan](icons/verbs/plan.svg) | `plan` | A path through a grid, one checkpoint per layer |
| ![build](icons/verbs/build.svg) | `build` | A stepwell, built one terrace at a time |
| ![verify](icons/verbs/verify.svg) | `verify` | A balance that weighs the change against the floors |
| ![inspect](icons/verbs/inspect.svg) | `inspect` | A lens over the lines of a pull request |
| ![proof](icons/verbs/proof.svg) | `proof` | A seal pressed with a check |
| ![ship](icons/verbs/ship.svg) | `ship` | A boat under sail |
| ![release](icons/verbs/release.svg) | `release` | A kite let out on its string |
| ![measure](icons/verbs/measure.svg) | `measure` | The sundial of Jantar Mantar reading the sun |
| ![tell](icons/verbs/tell.svg) | `tell` | A tutari horn announcing what is proven |
| ![decide](icons/verbs/decide.svg) | `decide` | A fork in the path, one branch chosen |
| ![counsel](icons/verbs/counsel.svg) | `counsel` | Atri's star over the road ahead |
| ![inbox](icons/verbs/inbox.svg) | `inbox` | A tray of cards that wait for a person |

## Respect

The twins are figures of the Hindu epics and the rishi tradition, and several are worshipped. The icons show an
attribute, a place, an animal, or a star linked to each name, never a deity, a face, or a body. They use none of these
symbols: Om, the trishul, the chakra discus, the conch, the lotus, the third eye, the tilak, or the swastika. Devanagari
is drawn by hand from geometric strokes and checked against Noto Sans Devanagari, so every letter is the right letter:
य, not प.

## Drawing rules

- **Grid.** Icons are drawn on a 24 px grid with 2 px round strokes, inside a 2 px margin. The App marks are drawn on
  a 256 px grid as rounded tiles (radius 56).
- **Exceptions.** A thinner secondary line, or a butt cap such as the stones of Nala's bridge, goes in an inline
  `style` attribute. The style block's class rules beat presentation attributes such as `stroke-width="1.25"`, so a
  browser ignores those.
- **Colour.** One main colour and one gold accent per icon, from `design/brand.yaml`: `accent` and `done-edge` on light
  grounds, `icon-night` and `icon-gold-night` on dark ones. Each pair is in the brand's pairs list, so it passes WCAG
  2.2 AA for graphics (3:1).
- **Light and dark.** Each icon carries a small style block. It switches colours with `prefers-color-scheme`, and a
  page that inlines an icon can set `--aayu-icon` and `--aayu-icon-accent` to recolour it. Its classes start with
  `aayu-` (`aayu-s` main stroke, `aayu-a` accent stroke, `aayu-f` and `aayu-fa` fills): an inlined style block applies
  to the whole page, so the prefix keeps the icon and the page from restyling each other. The App tiles carry their
  own background and look the same on either ground.
- **Accessible.** Every file is `role="img"` with a `<title>` that says what the icon shows and does.
- **In the terminal.** A terminal cannot draw these files, so `aayu inbox` keeps its emoji (`inboxKinds` and
  `seatIcons` in `internal/cli/inbox.go`).

## Changing an icon

Edit the SVG by hand. Then look at it at 16, 24, and 32 px on a white and on a dark ground (`#0d1117`) before you
commit it. Keep the comment at the top of each file.

## Where they go next

These surfaces can use the set. None of them is wired up in this change:

- the GitHub App logo uploads, for the bot App and aayu Sakshi;
- the `logo` field of `.cursor-plugin/plugin.json` and `interface.logo` of `.codex-plugin/plugin.json`;
- the MCP server's `Icons` on the server, its tools, and the persona prompt (`internal/mcp/server.go`);
- the docs site's favicon and header (`internal/tell/site.go`), the seat pages, and the skill pages;
- the README and the repository's social preview.
