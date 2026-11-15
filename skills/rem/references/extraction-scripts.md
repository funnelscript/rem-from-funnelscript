# Browser Extraction Scripts & DOM Contracts

REM embeds 4 specialized browser extraction scripts designed for safe, clean-room DOM analysis and visual asset capture.

## 1. `inventory.js` — Tech Stack & Element Inventory

Examines the runtime document, scripts, custom elements, and global variables to uncover the product's underlying architecture:

- **Framework Detection**: Identifies React, Next.js, Vue, Angular, Svelte, Remix, Astro, Tailwind CSS, Radix UI, Headless UI.
- **Global SDKs**: Checks 20+ enterprise monitoring, tracking, and telemetry SDKs (Sentry, Datadog, LaunchDarkly, Statsig, Segment, Amplitude, PostHog, Mixpanel, Intercom, Stripe).
- **Custom Tags**: Discovers all registered Web Components and framework custom tag names.
- **Test Attributes**: Discovers `data-testid`, `data-test`, `data-qa`, `data-cy`, `data-e2e`, and `data-component` attributes.
- **Forms & Inputs**: Catalogs form controls, input types, and submission endpoints.
- **Routing Structure**: Templates route paths into normalized `:id` patterns.

---

## 2. `tokens.js` — Design Tokens & OKLCH Color Space

Extracts the full design system directly from computed styles and CSS custom properties:

- **Color Palette**: Discovers all unique background, text, and border colors. Converts raw RGB/RGBA values to OKLCH color space for perceptually uniform palette clustering.
- **Typography**: Catalogs active font families, font sizes (in rem and px), font weights, line heights, and letter spacing.
- **Border Radii**: Identifies border radius scales used across containers, buttons, and inputs.
- **Shadows & Elevation**: Extracts `box-shadow` values to determine elevation hierarchies.
- **CSS Variables**: Dumps all `--*` root custom properties defined on `:root` and `body`.

---

## 3. `mask.js` — Two-Tier Physical DOM PII Redaction

Guarantees clean-room privacy and compliance before capturing screenshots:

- **Tier 1 (Physical Text Substitution)**: Replaces DOM text nodes with deterministic redacted character blocks (`██████`) or placeholder patterns. Replaces phone numbers, email addresses, usernames, and monetary quantities.
- **Tier 2 (CSS Blur & Visual Obfuscation)**: Applies CSS filters (`backdrop-filter: blur(8px)`) over user avatar images, profile banners, customer logos, and canvas viewports.
- **Input Value Wiping**: Clears sensitive values from `<input>`, `<textarea>`, and `<select>` elements.
- **Deterministic Placeholder Mode**: Preserves UI text flow length while scrubbing all private customer data.

---

## 4. `annotate.js` — Numbered Visual UI Anatomy

Overlays numbered outline badges on interactive elements:

- **Interactive Target Discovery**: Identifies buttons, links, inputs, dropdown triggers, and modal actions.
- **Badge Injection**: Attaches high-contrast circular badges `[1]`, `[2]`, `[3]` with z-index fencing.
- **Coordinate Mapping**: Records element bounding boxes (`x`, `y`, `width`, `height`), ARIA roles, accessible names, and CSS selectors.
- **Annotation Index**: Outputs a structured JSON mapping each badge index to its technical element contract.
