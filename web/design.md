# Design: tenkinow

The locked design system for this app. Every page reads this file before it
changes. Do not regenerate it per page. Extend or amend it when the system has
to grow.

**Theme name:** Nimbus.
**Derived from:** the [together.ai](https://www.together.ai) palette and floating
header, and the `studio` theme in [nutlope/hallmark](https://github.com/nutlope/hallmark)
for structure. Neither is copied. The values below are tuned for a weather
product that has to carry a diverging data ramp and two writing systems.

## Genre

modern-minimal, soft register. It should read as weather: a cool near-white
sky, soft tints for raised surfaces, generous corner radii, one warm accent, and
a nav that floats over the page instead of sitting on a rule.

## Macrostructure family

- **Marketing pages** (`/`, `/ja`): Stat-Led. The live temperature is the
  largest element on the page; a worded headline above it says what the number
  is. Sections that follow qualify it. Varies on: hero copy only.
- **App pages** (`/map`, `/playground`): Split Studio. Controls on one side,
  what they produce on the other. Below 64rem the pair stacks, form first.
  The map is the full-bleed variant: head diptych, control row, then one surface.
- **Content pages**: none yet. The OpenAPI reference is served by the API.

## Theme

| token                 | value                                               | role                                  |
| --------------------- | --------------------------------------------------- | ------------------------------------- |
| `--color-paper`       | `oklch(98% 0.004 240)`                              | page ground                           |
| `--color-paper-2`     | `oklch(95.5% 0.008 242)`                            | recessed                              |
| `--color-paper-3`     | `oklch(92% 0.012 245)`                              | disabled fills                        |
| `--color-card`        | `oklch(99.4% 0.002 240)`                            | raised surface                        |
| `--color-sky`         | `oklch(96.5% 0.020 244)`                            | the wash the page opens under         |
| `--color-mist` / `-2` | `oklch(95.5% 0.030 250)` / `oklch(96.5% 0.026 300)` | the `.tint` gradient pair             |
| `--color-rule`        | `oklch(90% 0.010 240)`                              | hairline                              |
| `--color-rule-2`      | `oklch(79% 0.016 245)`                              | the quiet half of a two-tone headline |
| `--color-muted`       | `oklch(52% 0.020 250)`                              | labels, units, help (7.2:1)           |
| `--color-neutral`     | `oklch(40% 0.026 250)`                              | secondary body (11.3:1)               |
| `--color-ink-2`       | `oklch(28% 0.030 252)`                              | running body (15.1:1)                 |
| `--color-ink`         | `oklch(18% 0.034 255)`                              | display, primary (18.4:1)             |
| `--color-accent`      | `oklch(54% 0.192 40)`                               | the only accent (5.8:1 both ways)     |
| `--color-accent-hi`   | `oklch(47% 0.180 38)`                               | hover                                 |
| `--color-accent-wash` | `oklch(95% 0.042 52)`                               | pressed / selected ground             |
| `--color-focus`       | `oklch(54% 0.192 40)`                               | focus ring                            |

Never pure white, never pure black. The accent is together.ai's `#fc4c02` stepped
down to the lightness where it clears 4.5:1 as text on paper and as paper on
it, so one value serves links, fills and rings. Keep it under 5% of any viewport.

The map's colour ramps are the one place values are written by hand
(`src/lib/map-scales.js`), because a canvas is painted in JS and cannot read a custom
property per dot. They are drawn from the same together.ai palette and must be
kept in step with these tokens.

## Typography

- **Display:** Plus Jakarta Sans, 700, roman. Tracking `-0.03em`.
- **Body:** Geist, 400/500. Base `1.0625rem`, line-height `1.65`.
- **Mono:** Geist Mono, 400/500, for every measurement, label, path and unit.
- **Japanese:** Zen Kaku Gothic New carries both roles, separated by weight.
  Loaded only on `/ja` routes. JA headings drop the negative tracking, take
  `line-height: 1.45`, and wrap with `word-break: auto-phrase`.
- Below 40rem the nav labels step down to `0.84rem` and the language switch
  rides the wordmark row. The nav is sticky, and in Japanese four destinations at
  the desktop size took a third row of a screen that has none to spare.
- Headings are always roman. Emphasis is weight, the accent, or the
  two-tone `.h-quiet` span. Never italic.
- **Two-tone headline:** the statement in `--color-ink`, the line completing it
  in `--color-rule-2`. Hierarchy inside one sentence, carried by colour.

## Spacing

4-point named scale; the values are in `tokens.css`. Pages use Tailwind's own
scale (`mt-7`) or a named token, never a raw magic number. `--section-gap`
separates major sections; `--page-gutter` is the only side padding.

## Shape and depth

`--radius-sm 8` · `--radius-input 10` · `--radius-card 16` · `--radius-lg 22` ·
`--radius-pill 999`. Buttons and chips are pills; cards are 16px.

Depth comes from shadow, not borders: `--shadow-cloud` on nav, cards and resting buttons,
`--shadow-lift` on hover. No hard-edged shadow anywhere.

## Motion

- Easings: `--ease-out cubic-bezier(0.16, 1, 0.3, 1)` and its in/in-out pair.
  Never the browser default, never a bounce on UI state.
- Durations: 140ms micro · 220ms short · 420ms long.
- Three primitives only: `rise` (entrance), the hero figure's count-up
  (first reading only; re-ticking on every ten-minute refresh reads as a
  glitch), and the drawn underline on nav links and `.link-cta`.
- Reduced motion collapses everything to ≤ 0.01ms. The focus ring is never
  animated.

## Microinteractions stance

- Silent success. The only confirmation on the page is the copy button briefly
  becoming its own success state, announced once through a visually-hidden
  `role="status"`.
- No toasts, no modals, no confirmation dialogs.
- Every interactive element ships all eight states: default · hover ·
  `:focus-visible` · `:active` · disabled · loading · error · success. `.is-*`
  classes mirror each pseudo-class so a state can be forced for review.
- Touch pointers get 44px targets via `@media (pointer: coarse)`; a mouse keeps
  the tighter 38px chip.

## CTA voice

- **Primary**, `.btn`: accent-filled pill, `--shadow-cloud` at rest,
  `--shadow-lift` on hover, sentence-case verb phrase ("Read my nearest station").
- **Secondary**, `.chip`: white pill, hairline border, mono label. Used for
  values (an endpoint name, a city, a mode), not for actions.
- **Tertiary**, `.link-cta`: a word, an arrow, an underline drawn from the left.

## Per-page allowances

- Marketing pages may use the `.tint` gradient block and the sky wash.
- App pages must not use enrichment. The live data is enough.
- No page ships invented figures. Every number on this site is a reading from
  the API or it is an em dash.

## What pages MUST share

- The wordmark 天気**now**, with `now` in the accent.
- The floating `.cloud` nav and the `.tint` colophon footer.
- The accent colour and its ≤5% budget.
- Display / body / mono faces and the two-tone headline pattern.
- The CTA voice: pill shape, shadow pair, padding rhythm.

## What pages MAY differ on

- Macrostructure within the page-type family.
- Which surfaces are carded versus laid on the ground.
- Whether the sky wash is visible above the fold.

## Exports

`tokens.css` at the project root is the framework-free copy of this system. The
site itself consumes the same values through Tailwind v4's `@theme` block at the
top of `src/styles/global.css`; that block is the source of truth, and
`tokens.css` mirrors it.
