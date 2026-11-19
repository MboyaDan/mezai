# Contributor pages rendered on the server, Stage 23 (2026-11-19)

Follow-up to `docs/screenshots/stage-18a/performance.md` ("Signed-in pages paint late on a slow phone").

## What changed (D-165)

- The contributor layout reads the session (`/v1/auth/me`) and profile (`/v1/me/contributor`) on the server. The dashboard reads its five data paths there in parallel. The task workspace reads the project there. The first HTML already holds the page.
- Before, the browser ran a chain after hydration: session, then profile, then the page's data. The dashboard made 7 API calls after load; now it makes none. The task workspace now makes one: the reservation `POST .../next`. That is a write, so it stays in the browser with the CSRF token and never runs during rendering or prefetch.
- Every server read times out after 2.5 s. On a timeout, error or missing cookie, the page falls back to loading in the browser, as before. A sleeping free-tier API therefore never blocks the page.

## How this was measured

- Lighthouse 13.5.0, mobile preset, **devtools throttling**, Chromium for Testing 1248, three runs per page. The median is reported.
- Production builds (`next build`, `next start`) of `main` (before) and this branch (after), built side by side in scratch copies.
- Each build had its own API process from the same source, on the local development database, with `APP_ORIGIN` set to that site's port. This means the task reservation succeeds and both runs show a real task.
- Signed in as Sample Contributor 001, an active contributor with one reserved rating task. The local database holds many projects left by earlier test runs, so absolute numbers differ from 18a's fresh database. Before and after used the same data.

## Results (median of 3)

| Page | Performance | LCP | FCP | TBT | CLS |
|---|---|---|---|---|---|
| Dashboard `/c`, before | 49 (49, 49, 53) | 5.31 s | 3.36 s | 11 ms | 0.679 |
| Dashboard `/c`, after | 98 (97, 98, 98) | 1.94 s | 1.94 s | 91 ms | 0.000 |
| Task workspace `/c/work/{id}`, before | 68 (68, 68, 68) | 5.92 s | 4.01 s | 25 ms | 0.000 |
| Task workspace `/c/work/{id}`, after | 86 (86, 85, 86) | 3.85 s | 1.86 s | 117 ms | 0.000 |

- **Layout shift:** before, the dashboard's skeleton was replaced piece by piece as each call returned, which caused the 0.68 layout shift. Now it arrives complete.
- **Blocking time:** it rose by about 80 to 90 ms on both pages, because hydration now runs over a page that already has its content. It is well under the 200 ms "good" threshold.
- **Task workspace:** its LCP is the task itself, which still waits for the reservation after hydration. Shortening that further would mean reserving on the server during rendering, which this stage deliberately does not do.

## Checked with these builds

- The first HTML of `/c` contains the dashboard ("Welcome", "Trust score") with JavaScript off.
- The reservation POST carries the CSRF token from the server-read session, and the API accepts it.
- The 18a sweep of the 9 contributor pages (axe, fit at 390, skip link and focus): passed.
- The sign-up flow (`join.spec.ts`, AC-04): passed.
- `dashboard.spec.ts` could not run with these API copies: they have no storage keys, and the test imports a file. CI runs it.
