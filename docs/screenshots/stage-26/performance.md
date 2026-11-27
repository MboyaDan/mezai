# Workspace pages rendered on the server, Stage 26 (2026-11-27)

This applies the Stage 23 pattern (D-165, `docs/screenshots/stage-23/performance.md`) to the staff and client workspace (D-168).

## What changed

- **Workspace layout:** it reads the session (`/v1/auth/me`) on the server, so the shell and navigation are in the first HTML.
- **`/w`:** it reads what that person's home shows.
  - Ops: `/v1/ops/overview`.
  - A client: their projects, then the progress of the projects the home lists (at most eight, drafts left out).
  - A reviewer: the review queue.
- **Project pages:** the project layout reads the project for every tab. The overview tab reads progress, quality, workforce, datasets and retention in parallel. It does not ask for panels Ops holds back from a client (D-167).
- **`/w/projects`:** it reads the unfiltered list.
- **Shared reads:** every server read is cached for the length of one request (`cache` in `lib/api/server.ts`), so the layout and page share the session and project reads.
- **Unchanged rules from D-165:**
  - GET only.
  - Only the session cookie is forwarded.
  - 2.5 s timeout.
  - On failure, the page loads in the browser as before.
  - Writes stay in the browser.

## How this was measured

- **Tool:** Lighthouse 13.5.0, mobile preset, devtools throttling, Chromium for Testing 1248. Three runs per page; the median is reported.
- **Builds:** production builds (`next build`, `next start`) of `main` at `aab6547` (before) and this branch (after), on ports 3101 and 3102.
- **API and data:** both builds used the same local API (Docker, same code on both sides) and the same development database. Lighthouse sent the session cookie as an extra header and made only GET requests, so one API served both builds.
- **Accounts:**
  - Ops pages: signed in as the Super Admin.
  - Client pages: signed in as Sample Client Admin.
  - Project: "Customer message routing (Sample)".
- **Caveat:** the local database holds about 160 projects left by earlier test runs. This makes the Ops list and Ops home heavier than a fresh database, so the absolute numbers differ from a clean install. Before and after used the same data.

## Results (median of 3)

| Page | Performance | LCP | FCP | TBT | CLS |
|---|---|---|---|---|---|
| Ops home `/w`, before | 79 (78, 79, 79) | 4.25 s | 3.36 s | 38 ms | 0.000 |
| Ops home `/w`, after | 97 (97, 97, 97) | 2.03 s | 2.03 s | 87 ms | 0.000 |
| Client home `/w`, before | 61 (61, 61, 62) | 4.11 s | 3.40 s | 0 ms | 0.365 |
| Client home `/w`, after | 97 (97, 97, 98) | 1.95 s | 1.95 s | 84 ms | 0.018 |
| Client project `/w/projects/{id}`, before | 62 (63, 62, 62) | 4.98 s | 4.24 s | 4 ms | 0.212 |
| Client project `/w/projects/{id}`, after | 97 (97, 97, 97) | 2.06 s | 2.06 s | 88 ms | 0.000 |
| Ops projects `/w/projects`, before | 69 (68, 69, 69) | 4.90 s | 3.77 s | 264 ms | 0.000 |
| Ops projects `/w/projects`, after | 95 (93, 95, 95) | 2.13 s | 2.13 s | 168 ms | 0.000 |

- **Layout shift:** before, the client pages replaced their skeletons piece by piece as each call returned, which shifted the layout (CLS 0.37 and 0.21). Now the page arrives whole. The remaining 0.018 on the client home is under the 0.1 "good" threshold.
- **Blocking time:** it rose by about 50 to 90 ms on three pages, because hydration now runs over a page that already holds its content. This is the same trade-off as Stage 23. All results are under the 200 ms "good" threshold.
- **Projects list:** its blocking time fell. The browser no longer renders a skeleton and then the full table.

## Checked with these builds

- **First HTML:** fetched without JavaScript, `/w` for Ops contains the home ("What needs attention today", "New enquiries to assign", "Open projects") after this change and none of it before.
- **Full-stack specs:** client portal, Ops overview, projects, client visibility and enquiries passed (10 tests).
- **Page sweep:** all four kinds of user, every page at 1440, 390 and dark (axe, fit at 390, skip link and focus), passed.

## Not measured

- **Other workspace pages** (review, payouts, contributors and the rest) still load in the browser. They are used by a few staff on desktop, and the same pattern can follow page by page if wanted.
- **Staging:** not measured. The free-tier API's cold start adds its own delay. When the API is asleep, the server read times out at 2.5 s and the page then loads in the browser, so the first view is no worse than before.
