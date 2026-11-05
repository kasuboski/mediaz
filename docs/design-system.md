# Mediaz design system

## Direction and ownership

Mediaz uses a Catppuccin Mocha-inspired dark theme for evening media browsing.
[DESIGN.md](../DESIGN.md) defines the design contract. This retheme preserves
layout, typography, radii, artwork, transitions, and interactions; there is no
light theme or theme switcher.

`frontend/src/index.css` is the source of truth for palette, semantic colors,
gradients, and shadows. Official sRGB hex colors are converted to HSL channels
(with precision retained to reproduce the original RGB values).
`frontend/tailwind.config.ts` maps semantic utilities with `<alpha-value>` support;
for example, `bg-success/10` and `border-info/20`. Stock Tailwind palettes are
not overridden. Do not introduce raw color literals or numbered palette utilities
in application components.

## Palette and semantic roles

| Palette | Hex | Application role |
| --- | --- | --- |
| Base | `#1e1e2e` | Canvas/background; foreground on filled pastel actions |
| Mantle | `#181825` | Cards and popovers |
| Crust | `#11111b` | Sidebar, modal scrims, shadow pigment |
| Surface 0 | `#313244` | Secondary/muted surfaces, sidebar hover |
| Surface 1 | `#45475a` | Decorative borders and interactive hover surface |
| Surface 2 | `#585b70` | Reserved supporting surface token |
| Text | `#cdd6f4` | Body, headings, card/popover text |
| Subtext 1 | `#bac2de` | Sidebar copy |
| Subtext 0 | `#a6adc8` | Muted copy, placeholders, meaningful input boundaries |
| Mauve | `#cba6f7` | Primary actions, selected states, focus rings |
| Blue | `#89b4fa` | `info`, links, downloading/in-progress states |
| Green | `#a6e3a1` | `success`, available/completed states |
| Yellow | `#f9e2af` | `warning`, queued/unreleased states |
| Red | `#f38ba8` | `destructive`, failed/missing states |
| Peach | `#fab387` | `category`, secondary media category |
| Lavender | `#b4befe` | Supporting visualization |
| Teal | `#94e2d5` | Supporting visualization |

`primary-foreground` and `destructive-foreground` are Base, not white: pastel
fills need dark text. `accent` is a neutral hover surface, not another brand
color. Semantic badges use tinted backgrounds, pastel text, and subtle borders;
keep their existing icons and labels so color is never the only signal.

The chart series tokens are `--chart-1` (Green), `--chart-2` (Blue),
`--chart-3` (Lavender), and `--chart-4` (Teal). Activity charts reference these
variables directly in SVG strokes; axes/grid use muted foreground/border tokens.
Series names and legends remain intact.

## Components and effects

- Primary buttons use Mauve or the subtle Mauve-to-Lavender primary gradient,
  always with Base text. Links use Blue.
- Cards use Mantle, with a subtle Mantle-to-Base optional gradient.
- Hero gradients end in Base, blending into the application canvas.
- Card/modal shadows use Crust; hover shadows use a faint Mauve tint.
- Forms use Base backgrounds and Subtext 0 boundaries. Decorative Surface 1
  separators are deliberately quieter and must not substitute for a meaningful
  control boundary. Focus rings use Mauve.
- Placeholder and secondary copy use Subtext 0 without additional opacity.
- Error toast controls use Base on Red, including close icons and focus rings.
- Dialog, sheet, drawer, and confirmation scrims use the centralized Crust token.
- The not-found page uses the same dark canvas and semantic text/link colors.

Use semantic utilities or CSS variables:

```tsx
<Badge className="bg-success/10 text-success border-success/20">Available</Badge>
```

```css
.panel {
  background: hsl(var(--card));
  color: hsl(var(--card-foreground));
  border-color: hsl(var(--border));
}
```

## Preserved structure

The system font stack, OpenType ligatures, spacing, responsive breakpoints,
poster aspect ratios, line clamps, radius (`0.75rem`), and existing animation
behavior are unchanged. Components continue to use Radix/shadcn primitives,
Lucide icons, and Tailwind utility layout. Do not use a retheme as an opportunity
to change page structure or interaction behavior.

## Noncentral exceptions

- Movie/TV detail hero text, ratings, genre badges, and controls retain white
  and translucent white utilities over unpredictable backdrop artwork. These are
  image-legibility treatments, not general application text colors.
- `components/ui/chart.tsx` contains literal `#ccc`/`#fff` in selectors that match
  Recharts-generated attributes. These are compatibility selectors, not palette
  declarations; their replacements use semantic stroke/fill utilities.
- Posters, backdrops, provider logos, and other external image assets retain
  their source colors. They are not recolored by the theme.
- Browser-native widgets and third-party generated details may retain their
  own rendering. Sonner toast containers/actions use semantic classes; no global
  stock-palette override is used to recolor dependency internals.

## Verification

Target at least 4.5:1 for body and placeholder copy, and 3:1 for focus indicators
and meaningful control boundaries. The brighter input boundary is intentional;
decorative separators need not have control-level contrast. Existing disabled
opacity and artwork-dependent contrast should be reviewed in context rather
than claiming universal WCAG conformance.

`frontend/e2e/screenshots.spec.ts` captures discovery, movie/TV library, detail,
activity, a download-client form, and narrow discovery/form views using the
real-backend isolated harness. It also asserts computed canvas/text, pastel
button foreground, input/placeholder colors, and focused Mauve ring. Captures
are attached artifacts, not committed golden-image comparisons. Remote artwork
has a bounded load wait so CDN availability does not block screenshots.

Run frontend commands through `nix develop --command` in this environment.
Build before running the complete Playwright suite; inspect screenshots for
artwork overlays, focus/hover states, readable status labels, and narrow layouts.
