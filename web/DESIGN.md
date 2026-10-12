# Resonance Archive · WuWa "Tacet Resonance" Design System

> Category: Project Design System
> Surface: web
> Source project: `opendesign-wuwa-lore-rag-prompt.md` (`590e3270-e9c2-4fb0-9b28-2b9e59e9fdc4`)
> Skill lineage: `skills/design-md/{notion,wired,theverge,linear.app,vercel,mintlify}/DESIGN.md` (VoltAgent awesome-design-md, MIT)

## 1. Visual Theme & Atmosphere

**Dark-first game codex.** Solaris-3 abyss (`#070B14`) with resonance-teal glow
and brass citations — a Tacet field the reader consults, not scholarly paper.
Reader surfaces keep Wired-style editorial serif + footnote apparatus; admin
keeps Linear-style near-black console panels. Mintlify/Vercel gradient heroes
appear at hero scale only, never miniaturised.

That last decision is the thesis of the design. The product claims *"every answer carries its
receipts."* A side rail is the one layout that divorces the evidence from the claim it supports,
so the sources are set as the apparatus beneath the answer instead — the claim, the superscript
numeral, then the passage it came from. Source cards are now 12px codex panels
(Notion/Mintlify) so passages read as collectible lore entries.

Dark is the default (`data-theme="dark"`); light is a supported Solaris-paper fallback.

## 2. Color

WuWa resonance pair on abyss neutrals. Teal = interactive only; brass = source only.

| Role | Light (Solaris paper) | Dark (Tacet abyss, default) |
|---|---|---|
| `--bg-0` … `--bg-3` | `#EFF2F7` → `#D8E0EE` | `#070B14` → `#1B2745` |
| `--text-1` … `--text-3` | `#101724` → `#6B7A93` | `#E9EEF7` → `#6E7E99` |
| `--border` / `--rule` | `#D8E0EE` / `#DDE4F0` | `#1E2A45` / `#22304D` |
| `--border-input` | `#8A99B5` | `#3D4F74` |
| `--accent` (resonance teal) | `#0B6B5F` | `#2DD4BF` |
| `--gold` (brass citation) | `#7A5200` | `#D9A441` |
| hero atmosphere | `#0B1B33` → `#0E4F4A` → `#C9A227` | `#070B14` → `#0E4F4A` → `#8A6A1B` |
| danger / success / warning / info | `#A4203A` `#17694A` `#8A5D05` `#1B54A8` | lightened |

Three rules, enforced:

- **`--accent` never decorates.** It marks interactive and active state only: primary buttons,
  focus rings, the active tab rule, the active sub-nav rule, a highlighted footnote. If something
  is teal, the user can do something with it.
- **`--gold` means "this is a source."** Citation numerals, footnote markers, the source index
  numeral, the MOCK badge, and nothing else.
- **`--rule` is the workhorse; `--border` is only for controls.** Layout separation comes from
  hairlines and space. A filled or outlined box around a group of content is the failure mode to
  avoid — nine components used to all be "a 1px border on a dark field" and were
  indistinguishable.

Speaker chips take a hue from a stable FNV hash of the name (`oklch(.94 .03 H)` light,
`oklch(.26 .035 H)` dark) so a speaker keeps one colour across every screen.

## 3. Typography

Three families, with narrower jobs than before — this is an editorial system, so the serif does
real work rather than only decorating headings.

- **Newsreader** — display, the answer body, running prose, and form placeholders. Page titles
  `clamp(28px,3.6vw,40px)`, answer body 18.5px/1.68, prose 17.5px/1.72. **Full `opsz` axis is
  retained on purpose** — pinning it saved 68 KB but visibly widened and lowered the contrast of
  the 58px hero. The most prominent element is not where to economise.
- **Inter** — UI chrome: nav, buttons, field labels, panels, suggested-question rows. Italic is
  loaded because every `<em>` in the product resolves to Inter or Mono (Newsreader italic was
  dead weight and has been removed).
- **JetBrains Mono** — data, section labels, the eyebrow, commands, JSON, timestamps. Anything
  that can change value uses `tabular-nums`.

Measure: `--measure: 66ch` for prose and answers, `--measure-tight: 58ch` for page heads.
Headings use `text-wrap: balance`, body copy `text-wrap: pretty`, never both on code or long prose.

## 4. Spacing

Base unit 4px; the working scale is 4 / 8 / 12 / 20 / 36. Gutters are 28px, dropping to 20px
below 900px and 14px inside the top bar below 480px.

**Radii are 8px controls / 12px cards** (Notion/Mintlify), pills stay round.
Primary CTAs are Convene-style pills with resonance glow on hover (Vercel/Mintlify).
The composer stays ruled rather than boxed; source cards and surfaces are 12px
codex panels with stacked subtle shadows; modals keep one soft shadow; the bottom sheet has a
0 radius on its top corners and a 3px grip.

Section separation is `padding-top` plus a `--rule` border, never a box. Sections that would
previously have been separate `.card` elements are now `.rule-block` or just stacked rules.

## 5. Layout & Composition

- **Shell** — sticky masthead with a hairline underside and no fill. Below 760px the primary tabs
  collapse into a fixed bottom nav.
- **Ask** — two columns: history rail `250px`, thread `700px`. **The sources rail is gone.** It
  was reserved whenever sources existed, but as a third column it read as a dashboard panel
  rather than as evidence. Sources now render inside the answer as `.footnotes`.
- **Admin** — page head at measure, sub-nav as tabs over a single rule, then content. KPI tiles
  are **a row of figures divided by vertical hairlines** with rules top and bottom — not six
  cards.
- **Tables** — printed, not gridded: a stronger rule under the header, hairline row separators,
  no vertical lines, no zebra. The header is deliberately **not** sticky, because `.table-wrap`
  is a scroll container and would become the containing block for sticky positioning.
- **Flow rows** (pipeline, topology) are CSS grid with a numbered `data-step`, rendering as a
  vertical list under 900px and horizontally above it. A wrapping flex row stranded its
  connector arrows at line ends and read as a broken chain.

## 6. Components

Buttons, chips, pills, MOCK badge, speaker chips, `.panel`, `.rule-block`, KPI figure row,
`.state` blocks, skeletons, score figure, source/footnote entry, composer, `.seg`, `.switch`,
command palette, modals, bottom sheets, tooltips, data tables, console and JSON blocks.

Domain modules that must not collapse into generic cards: the **answer column** (serif prose with
interactive superscript citation chips), the **footnote block** (hanging numerals in the gutter,
score as figure-over-rule, expand-to-full-chunk, copy, wiki link), the **pending waveform** with
elapsed timer and Cancel, the **numbered topology**, the **CLI mirror** panel on ingest, and the
**history rail**.

Every data view has default, hover, focus-visible, active, disabled, loading, empty, error and
success states.

## 7. Motion & Interaction

120 / 200 / 320ms; `cubic-bezier(.2,.7,.3,1)` out, `cubic-bezier(.5,0,.8,.4)` in. Transitions name
their properties explicitly — never `transition: all`.

Motion only clarifies state. Entrance is opacity plus a small translate; exit is shorter. Buttons
press with `translateY(1px) scale(.985)`. Suggested-question rows shift 8px right on hover, which
in a ruled list reads as "this line is live."

`prefers-reduced-motion` is honoured twice: a media query that removes animations, and a
`body[data-reduced]` attribute driven by the Settings override.

30px icon buttons extend their hit area to 38px with an inset `::after` rather than growing, so
the console stays dense without becoming hard to hit.

**Overlays take focus on open, trap Tab, and restore focus on close.** Single-key shortcuts
(`/`, `?`) do not reach through an open dialog. This was measured, not assumed — see §9.

## 8. Voice & Brand

Plain, technical, specific. Monospace for anything copyable. Hard rules the copy must respect:

- `model` is the LiteLLM **alias** (`wuwa-lore-model`), never "answered by Gemini" or "Groq".
- Relevance scores are **raw cosine similarity**, shown as the number the API returned, never
  relabelled as a percentage.
- `/chat` is stateless and single-turn, and the UI says so.
- No streaming. Pending is an indeterminate waveform with an elapsed timer, never faked progress.
- A missing or empty `model` means **Retrieval-only**, shown as a neutral badge.
- A fetch failure with no HTTP status is a network/CORS problem, not an HTTP error.
- Out-of-scope questions return a thin answer, and the archive says so.

Product is **Resonance Archive**, an unofficial fan project, not affiliated with Kuro Games.

## 9. Performance Contract

Measured on slow 4G (1.6 Mbps, 150 ms RTT) with 4× CPU throttle, not asserted:

- Fonts are **self-hosted and subset** to the glyphs the UI can render — three woff2 files,
  214 KB, no third-party connection. Newsreader keeps its full `opsz` axis (see §3); Newsreader
  italic was removed after instrumenting every `<em>` across all screens and finding none
  resolved to it.
- Fonts are **not preloaded**. Measured: preloading all three cut CLS to 0.0002 but roughly
  doubled FCP (700 → 1370 ms). CLS 0.04 is already inside the "good" threshold, so `swap` wins.
- No render-blocking `@import`. The font chain cost four serialised round trips.
- Scroll is a locked 60 fps with and without the `backdrop-filter` blur on five elements — the
  blur was not a bottleneck and was kept, reduced rather than removed.
- No horizontal scroll and no console errors at 360 / 390 / 430 / 600 / 820 / 1024 / 1366 /
  1440 / 1920, in both themes.

## 10. Anti-patterns

- Purple/indigo gradients, glow blobs, or decorative hero media with no data behind it.
- **Gradient anywhere except the hero band + tacet line.** Vercel rule: atmosphere lives at hero scale only.
- Large radii on containers holding text. Only pills and dots are round.
- Cards inside cards; nested panels as a substitute for layout.
- `transition: all`, `will-change: all`, positive `tabindex`, `outline: none` with no replacement.
- A button that never got an explicit `background` — the UA default then shows through under
  `color-scheme: dark`. Reset `button { background: none; border: 0 }` and set it per component.
- `position: sticky` inside an `overflow` container — the container becomes the containing block.
- Colour used decoratively, or gold on anything that is not a source reference.
- Shorthand UI text in user-facing copy ("chips carry a stable hue per name").
- Placeholder text or lorem-shaped statistics. Use the real figures: 5 pages, 222 chunks,
  71 speakers, ≈6 s, ≈2.7k tokens.