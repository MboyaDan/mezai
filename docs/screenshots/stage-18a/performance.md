# Performance and accessibility, Stage 18a (2026-11-17)

## How this was measured

- Lighthouse 13.5.0, mobile preset (Moto G Power, 4x CPU slowdown, slow 4G), Chromium for Testing 1248 (the newer 1254 build fails with `NO_NAVSTART`). Production build (`next build`, `next start`).
- Data: a separate, freshly seeded database (`mezai_measure`), so pages show pilot-sized data (3 projects for the contributor), not the hundreds of projects that earlier test runs left in the local database.
- Machine: MacBook (Apple silicon); API and site on localhost, so network times are optimistic compared with a hosted site.
- Signed-in pages: Sample Contributor 007, with the session cookie passed to Lighthouse.
- Three runs per page; the median by performance score is reported.

Lighthouse's default **simulated** throttling estimates timings from a fast local load. Here it proved unstable: the same build gave home LCP 2.94 s in one batch and 3.58 s in another. **Devtools** throttling applies the same profile to a real load and gave consistent runs (within 0.1 s), so it is the primary figure below. Both are recorded.

## Results (devtools throttling, median of 3)

| Page | Performance | Accessibility | Best practices | LCP | FCP | TBT | CLS |
|---|---|---|---|---|---|---|---|
| Home `/` | 97 (96, 97, 97) | 100 | 100 | 1.69 s | 1.69 s | 143 ms | 0.000 |
| Services `/services` | 98 (98, 98, 99) | 100 | 100 | 1.68 s | 1.68 s | 103 ms | 0.005 |
| Contributor dashboard `/c` | 80 (79, 80, 80) | 100 | 100 | 4.13 s | 3.42 s | 0 ms | 0.014 |
| Task workspace `/c/work/{id}` (response comparison) | 74 (74, 74, 74) | 100 | 100 | 4.87 s | 4.18 s | 0 ms | 0.014 |

Simulated throttling, same build, for comparison: home 88 / LCP 3.58 s, services 91 / 3.23 s, dashboard 91 / 3.46 s, task 90 / 3.63 s.

SEO is not in the table: non-production builds send `noindex` on purpose (D-048), which caps the score.

## Against the brief (section 17)

| Target | Result |
|---|---|
| Public pages: LCP under 2.5 s on a mid-range phone on 4G | **Met** with devtools throttling: home 1.69 s, services 1.68 s. Not met in the simulated mode (3.2 to 3.6 s), which counts every byte requested before the LCP against it. Not yet checked on a real phone over a real network: do that on staging (Stage 19). |
| Home page JavaScript under about 170 KB compressed on first load, GSAP after first paint excluded | **175 KB**, about 3% over. React DOM (72 KB) and the Next.js runtime (47 KB) are 119 KB of it. GSAP and the scroll stories (about 75 KB) load after first paint and are excluded. |
| Task workspace submit under 300 ms at p95 on staging | Not measurable locally; Stage 19 on staging. Stage 9c measured about 350 next-and-submit cycles a second locally. |

## What changed in 18a

1. **Legal text off every page** (home first-load JavaScript 181 to 175 KB). The footer, sitemap and sign-up consent imported the full legal documents (about 14 KB compressed) to read their titles and versions. They now import `content/legal-index.ts`; the full text loads only on the legal pages. A test keeps the index equal to the documents.
2. **Hero subline visible from first paint** (home LCP 3.31 s to 1.69 s with devtools throttling). The subline is the page's largest text. It faded in from opacity 0 after a delay, and Chrome only counts a visible element, so on a slow phone LCP waited for the first frame of the fade behind hydration work. It now settles in from 60% opacity with the same rise (D-149).
3. **Skip links move focus.** Every shell had a "Skip to content" link, but `<main>` could not take focus, so screen readers stayed at the top. `<main>` now has `tabIndex={-1}`. The sign-in pages had no skip link; they have one now.

Tried and dropped: inlining CSS (`experimental.inlineCss`) gave no measurable gain (home LCP 3.08 s before, 3.09 s after, simulated), so it was reverted.

## Still below where it should be

- **Signed-in pages paint late on a slow phone.** The dashboard and task workspace show nothing until the browser has checked the session and fetched the page's data after hydration (first paint 3.4 s and 4.2 s). The brief sets no LCP target for them, but contributors will mostly work on phones. Recommended fix: render the session check and the first data on the server (Next.js server components reading the session cookie), so the first HTML already holds the page. That is a larger change across the contributor surface; proposed as its own stage.
- **14 KB of polyfills** (`Array.prototype.at`, `Object.hasOwn` and others) ship inside the Next.js runtime chunk for older browsers. They cannot be removed without patching the framework.
- **About 27 KB of JavaScript is unused at load** on the home page (Lighthouse estimate), mostly in framework chunks.

## Images and fonts

- Home: one raster image above the lazy threshold (AVIF, 27 KB); the rest are inline SVG. Services: no raster images. Photos are served as AVIF or WebP through `next/image` (Stage 5c).
- Fonts: four self-hosted woff2 files on the home page, 133 KB in all: Inter (variable, 48 KB), JetBrains Mono (variable, 40 KB), Space Grotesk (variable, 23 KB), Instrument Serif italic (23 KB). All four are used above the fold, so none is deferred. Variable files are kept because each family is used at two or three weights.

## Accessibility and keyboard (all pages)

`tests/full/sweep.spec.ts` signs in as each kind of user and visits every page they can open, with real IDs from the API: 19 public pages, 9 contributor pages, 20 Ops pages, 9 client pages (57 pages). Each page is checked at 1440 light, 390 light and 1440 dark:

- axe (WCAG 2.0, 2.1 and 2.2 A and AA rules): no serious or critical violation;
- no sideways page scroll;
- at 1440 light, the keyboard: the first Tab stop is "Skip to content" and Enter moves focus to `<main>`; then the first 15 Tab stops each show a visible focus indicator (outline or ring).

Correction (Stage 18b): in this run the 9 contributor pages showed only the "review the updated documents" prompt, because seed contributors had accepted older legal versions, so the real contributor pages were not checked. From 18b the sweep accepts the current versions first and fails any page that shows the prompt.

Result (2026-11-17, fresh seed, production build): **all four passed in 14.3 minutes**: 57 pages × 3 views, no serious or critical axe issue, no sideways scroll, the skip link first on every page and moving focus to `<main>`, and a visible focus indicator on every checked Tab stop. Before the skip-link fix the same check found `<main>` could not take focus (fixed in this stage). Pages the fresh data could not reach (a project request, a review item, a payout batch) are covered by their own specs.

### Manual checklist

| Check | How | Result |
|---|---|---|
| Every action reachable by keyboard | Sweep (first 15 stops per page) and the feature specs, which drive forms and dialogs by role and label | Passed where automated; a full manual pass on every page was not done |
| Visible focus | Sweep focus check; tokens give a 2 px outline on links and buttons | Passed |
| Labels on every field | axe `label` rule on every page | Passed |
| Dialogs trap and return focus | Native `<dialog>` with `showModal()` | By design; not separately tested |
| Reduced motion | `tests/e2e/home-motion.spec.ts` and `design.spec.ts` check that reduced motion shows full content without animation | Passed (rerun after the hero change) |
| Screen reader (VoiceOver, TalkBack) | Not run: needs a person with the screen reader | **Not run** |
