# More workspace pages read on the server, Stage 27 (2026-11-27)

Stage 26 (D-168) moved the workspace frame, the homes and the project pages to server reads. This stage does the same for the pages Ops and reviewers work in most (D-169).

## What changed

| Page | Read on the server |
|---|---|
| `/w/review` | the queue for the page's filters (`?project=`, `?reason=`) |
| `/w/review/{id}` | the item with every answer |
| `/w/payouts` | the batch list, next payout date and B2C mode |
| `/w/payouts/{id}` | the batch and its payment states, in parallel; used only if both arrive |
| `/w/contributors` | the tab the page opens on: the qualification queue for approvers, everyone for others; the other tabs load when opened |

`lib/workspace-reads.ts` holds the paths and the opening-tab rule, so the server and the browser pick the same data. D-165's rules are unchanged: GET only, session cookie only, 2.5 s timeout, browser fallback, and decisions, approvals and payments stay in the browser.

## First HTML

Production builds of `main` (`73e0a7a`) and this branch, signed in as the Super Admin, fetched with curl (no JavaScript). Count of `<table>` elements in the first HTML:

| Page | main | branch |
|---|---|---|
| `/w/review` | 0 | 1 |
| `/w/review/{id}` | no decision controls | answers and decision controls |
| `/w/payouts` | 0 | 1 |
| `/w/payouts/{id}` | 0 | 1 |
| `/w/contributors` (queue) | 0 | 1 |
| `/w/contributors?tab=all` | 0 | 1 |

## Lighthouse

Lighthouse 13.5.0, mobile preset, devtools throttling, Chromium for Testing 1248, three runs per page, session cookie as an extra header. Both builds ran on port 3000 in turn against the same local API and development database.

| Page | Performance, before → after | LCP | TBT | CLS |
|---|---|---|---|---|
| Review queue | 92 → 92 | 3.15 → 3.18 s | 100 → 97 ms | 0.001 → 0 |
| Review item | 91 → 92 | 3.31 → 3.16 s | 94 → 51 ms | 0 → 0 |
| Contributors | 92 → 92 | 3.16 → 3.16 s | 97 → 101 ms | 0 → 0 |
| Payouts | 91 → 92 | 3.39 → 3.15 s | 56 → 42 ms | 0 → 0 |
| Payout batch | 89 → 92 | 3.62 → 3.15 s | 97 → 48 ms | 0.001 → 0.003 |

Medians. The gains are small because Stage 26 already put the frame and page title in the first HTML, and Lighthouse's largest paint on these pages is that frame, not the table. What changes for a person is that the table arrives with the page instead of one round trip later; on staging the API is in Frankfurt, so that round trip is longer than it is locally.

Not measured: staging.
