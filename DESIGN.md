# Mediaz visual design

## Direction

Catppuccin Mocha-inspired dark theme for a self-hosted media library, commonly browsed in subdued evening light. Preserve existing layouts, typography, artwork, and interactions; change the color system rather than redesigning screens.

Sources: [official palette](https://catppuccin.com/palette/) and [style guide](https://github.com/catppuccin/catppuccin/blob/main/docs/style-guide.md).

## Palette and roles

- Base `#1e1e2e`: application canvas.
- Mantle `#181825`, Crust `#11111b`: navigation, recessed surfaces, shadows.
- Surface 0 `#313244`, Surface 1 `#45475a`, Surface 2 `#585b70`: controls, hover surfaces, borders.
- Text `#cdd6f4`: body and headings. Subtext 1 `#bac2de` / Subtext 0 `#a6adc8`: secondary copy and placeholders.
- Mauve `#cba6f7`: primary actions, active navigation, focus rings. Use Base text on filled pastel buttons, never white.
- Blue `#89b4fa`: information and links; Green `#a6e3a1`: success; Yellow `#f9e2af`: warning; Red `#f38ba8`: error/destructive; Peach `#fab387`: secondary categories.
- Lavender `#b4befe`, Teal `#94e2d5`: supporting visualization roles only.

Use restrained accents, not rainbow decoration. Keep status labels/icons alongside color. Body and placeholder text require 4.5:1 contrast; focus and meaningful control boundaries require 3:1 where applicable. Low-contrast decorative separators are not control affordances.

## Implementation contract

`frontend/src/index.css` owns palette and semantic CSS variables. Retain the existing HSL-channel format for Tailwind compatibility; derive values from official hex colors rather than using approximate named Tailwind shades. `frontend/tailwind.config.ts` maps semantic utilities to those variables, including opacity modifiers. Prefer semantic success/warning/info colors to arbitrary color utilities.

This is not purely a CSS change: existing hard-coded Tailwind colors in pages, badges, state helpers, charts, toast variants, and artwork overlays need inspection and migration. Do not globally override Tailwind's entire stock palette. Keep white text and black scrims only where intentionally needed over unpredictable poster/backdrop imagery; document these exceptions. Replace hard-coded fallback/404 light surfaces. Hero gradients must terminate in the new Base color.

Keep gradients subtle and palette-based. Preserve layout, fonts, radius, and behavior. No new dependencies or theme switcher are needed.

## Verification

Run frontend build and the complete Playwright E2E suite using its real-backend isolated harness. Capture representative discovery, library, detail, and activity screens; include a narrow viewport and a form or dialog where practical. Inspect screenshots and record any environment limitations honestly. See `docs/design-system.md` for supporting implementation guidance.
