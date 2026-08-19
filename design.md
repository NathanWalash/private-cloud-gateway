# Design system — Private Cloud Gateway

The dashboard's look is **clean and minimal** (Linear / Vercel lineage):
near-monochrome, generous whitespace, crisp type, one restrained accent. This
document is the source of truth for the UI. Prefer editing tokens here (and the
Tailwind config / `index.css`) over one-off styles in components.

## Principles

1. **Content first.** The apps are the product; the chrome recedes. Fewer boxes,
   lines, and colours — more space.
2. **Near-monochrome + one accent.** Colour is information, not decoration. The
   accent marks the single most important action or a live state — nothing else.
3. **Restraint over flourish.** No gradients, glassmorphism, glows, or drop
   shadows for their own sake. Depth comes from a subtle border and one soft
   shadow, not layers of effects.
4. **Consistency is the aesthetic.** One spacing scale, one radius, one border
   colour, one type scale — applied everywhere.

## Avoiding the "AI-generated" look

These are the tells this refresh removes:

| Tell | Do instead |
| --- | --- |
| Default Tailwind **indigo-500 (`#6366f1`)** accent | A single considered accent, used sparingly |
| Purple/blue **gradients** and glows | Flat fills; depth from a 1px border + one soft shadow |
| Every element in a **rounded card** | Group with space and hairlines; reserve cards for real units |
| **Emoji** as UI icons | Consistent line icons (Lucide), one size/stroke |
| Loud, saturated **status colours** | Muted semantic colours + a small dot, not full-bleed fills |
| Inconsistent, generous **rounding + shadows** | One radius token, one shadow token |

## Colour

Near-neutral greys (very slightly cool) with high-contrast text, plus one accent.
Proposed tokens (replace the current values in `index.css`):

```css
/* Dark (default) */
--color-surface:      #0a0a0b;  /* app background — near-black neutral */
--color-card:         #141518;  /* elevated surfaces */
--color-border:       #24262c;  /* hairlines, 1px */
--color-text-primary: #ededf0;
--color-text-muted:   #9a9ba3;

/* Light */
--color-surface:      #fbfbfc;
--color-card:         #ffffff;
--color-border:       #ececef;
--color-text-primary: #0a0a0b;
--color-text-muted:   #6b6c74;
```

**Accent** — one colour, used only for the primary action and live/active state:

```css
--color-accent:       #5b5bd6;  /* considered indigo-violet, calmer than 6366f1 */
--color-accent-hover: #4f4fc4;
```

**Semantic** (muted; pair with a small dot or text, avoid full-bleed fills):
`success #3fb950`, `warning #d29922`, `danger #f85149`, `neutral #6b6c74`.

## Typography

- **Font:** keep the system stack for UI (`system-ui`); add a **monospace**
  token (`ui-monospace, SFMono-Regular, Menlo, monospace`) for IDs, versions,
  codes, and log output. Optionally bundle **Inter** via `@fontsource/inter` for
  a more distinctive-but-neutral UI face — no external CDN.
- **Scale (rem):** 0.75 / 0.8125 / 0.875 / 1 / 1.125 / 1.375 / 1.75.
- **Weights:** 400 body, 500 UI labels/buttons, 600 headings. Avoid 700+.
- **Headings:** tighten tracking (`-0.01em` to `-0.02em`); don't bold everything.
- **Body/muted:** `text-muted` for secondary text; never grey-on-grey below AA.

## Spacing & layout

- **4px base scale:** 4, 8, 12, 16, 24, 32, 48, 64. Use these, not arbitrary values.
- Page gutter 24–32px; card padding 16–20px; grid gap 16px.
- Max content width ~1200px, centred. Let whitespace do the grouping.

## Shape, border, elevation

- **Radius:** one token — `10px` for cards/inputs/buttons (`rounded-[10px]`).
  Not `xl` on some and `lg` on others.
- **Border:** 1px `--color-border` hairlines. This is the primary separator.
- **Elevation:** at most one soft shadow for genuinely floating elements
  (menus, modals): `0 8px 24px rgba(0,0,0,0.24)` (dark). Cards use border, not shadow.

## Components

- **Buttons:** primary = accent fill; secondary = transparent with border;
  ghost = text only. 36–40px tall, 500 weight, `150ms` colour transition.
- **Cards (app tiles):** border + `--color-card`, no shadow; hover = border
  lightens + 1px lift, not a glow. Icon, name, status dot, one action.
- **Inputs:** surface fill, 1px border, border → accent on focus (no heavy ring).
- **Status:** a 6–8px dot in the semantic colour + a text label. No pill floods.
- **Badges/meta:** monospace for versions/IDs (e.g. the `/api/status` version).

## Motion

Fast and quiet: `120–160ms`, `ease-out`, on colour/opacity/transform only.
No bouncing, no long fades. Respect `prefers-reduced-motion`.

## Iconography

Lucide line icons, one stroke width (1.5px) and a consistent size (16 or 20px).
App icons in blueprints may keep their brand marks; the dashboard chrome uses
Lucide only.

## Applying this

1. Update tokens in `apps/web/src/index.css` and `apps/web/tailwind.config.ts`.
2. Normalise radius/border/shadow in the `.card` / `.btn-*` / `.input-field`
   component layers.
3. Sweep components for hard-coded colours and default indigo; replace with tokens.
4. Review in the running app (`pnpm dev`) at desktop and mobile widths.
