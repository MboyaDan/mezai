# Lighthouse, home page (Stage 5b)

Lighthouse 13.5.0, mobile preset (Moto G Power emulation, 4x CPU slowdown, slow 4G), production build (`next start`), Chromium for Testing 1248, median of 3 runs, measured 2026-11-03 on a MacBook (Apple silicon). Local server, so network figures are optimistic compared with a hosted site.

| | Performance | Accessibility | Best practices | SEO | LCP | FCP | TBT | CLS | JS transferred |
|---|---|---|---|---|---|---|---|---|---|
| 5a (no motion) | 94 | 100 | 96 | 66* | 2.71 s | 2.11 s | 91 ms | 0.001 | 186 KB |
| 5b (motion) | 93 (runs: 93, 90, 94) | 100 | 96 | 66* | 2.62 s | 2.11 s | 96 ms | 0.000 | 210 KB, of which 54 KB is GSAP loaded after idle |

\* SEO is capped because non-production builds send `noindex` on purpose (`SITE_INDEXABLE` unset, D-048). With indexing allowed the robots audit would pass; not measured.

**JavaScript budget (170 KB on first load, lazy GSAP excluded):** about 156 KB on first load after moving Motion off public pages (D-050), down from 186 KB.

**Frames while scrolling** (headless Chromium, CPU throttled 4x, `requestAnimationFrame` intervals over a full scroll): desktop with motion, mobile with motion and mobile with reduced motion all held a 16.7 ms median and p95, with no frame over 33 ms. Synthetic; not yet checked on a real mid-range Android.
