# Decisions

Each decision has an ID, date, status and reason. "Proposed" means waiting for Dan's approval in the stage PR.

| ID | Date | Decision | Status | Reason |
|---|---|---|---|---|
| D-001 | 2026-11-02 | Part B of CLAUDE.md moved unchanged to `docs/design/DESIGN.md`; CLAUDE.md points to it. `docs/claude/MASTER_PROMPT.md` keeps the original full text. | Done | Shorter session context (CLAUDE.md instruction) |
| D-002 | 2026-11-02 | Visual direction: Concept 1, African Intelligence Infrastructure. | Proposed | `docs/design/concepts.md` |
| D-003 | 2026-11-02 | Agreement bands follow the brief: κ < 0.40 weak; 0.40 to 0.60 moderate; above 0.60 to 0.80 substantial; above 0.80 strong. The number is always shown. | Proposed (C2) | Brief section 8.2 wins over CLAUDE.md; closes the 0.40 to 0.41 gap |
| D-004 | 2026-11-02 | Seven pilot roles, including Payout Approver. | Proposed (C1) | Brief section 4 |
| D-005 | 2026-11-02 | Space Grotesk stays as display face (Manrope and Sora tested). | Proposed | `docs/design/concepts/type-test.html` |
| D-006 | 2026-11-02 | `success` #2D7A4D and `warning` #976000 (from #2E7D4F and #9A6200) so status text passes 4.5:1 on ivory. | Proposed | Computed WCAG 2 ratios, `docs/design/concepts.md` |
| D-007 | 2026-11-02 | One public site for both audiences; "Join as a contributor" is always the secondary action. | Proposed | Storyboard and research notes |
| D-008 | 2026-11-02 | No `pages` table; only `stats` in the database. Page content in `frontend/content/`. | Proposed (C3) | Brief section 5.2 (no CMS) |
| D-009 | 2026-11-02 | Route prefixes: public at root, contributor at `/c`, workspace at `/w`, shared auth at `/login`, `/join`, etc. | Proposed | `docs/design/sitemap.md` |
| D-010 | 2026-11-02 | Stage 0 concept fonts are committed as woff2 copies in `docs/design/concepts/fonts/` (SIL OFL); the app uses npm fontsource packages. | Done | Concepts must render offline with real fonts |
| D-011 | 2026-11-02 | Stages 2, 5, 6, 9 and 13 are planned as a/b splits. | Proposed | Usage limits; one sitting per PR |
