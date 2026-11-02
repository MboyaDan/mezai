# Decisions

Each decision has an ID, date, status and reason. "Proposed" means waiting for Dan's approval in the stage PR. Stage 0 decisions were approved when PR #1 was merged without changes.

| ID | Date | Decision | Status | Reason |
|---|---|---|---|---|
| D-001 | 2026-11-02 | Part B of CLAUDE.md moved unchanged to `docs/design/DESIGN.md`; CLAUDE.md points to it. `docs/claude/MASTER_PROMPT.md` keeps the original full text. | Done | Shorter session context (CLAUDE.md instruction) |
| D-002 | 2026-11-02 | Visual direction: Concept 1, African Intelligence Infrastructure. | Approved, PR #1 | `docs/design/concepts.md` |
| D-003 | 2026-11-02 | Agreement bands follow the brief: κ < 0.40 weak; 0.40 to 0.60 moderate; above 0.60 to 0.80 substantial; above 0.80 strong. The number is always shown. | Approved, PR #1 (C2) | Brief section 8.2 wins over CLAUDE.md; closes the 0.40 to 0.41 gap |
| D-004 | 2026-11-02 | Seven pilot roles, including Payout Approver. | Approved, PR #1 (C1) | Brief section 4 |
| D-005 | 2026-11-02 | Space Grotesk stays as display face (Manrope and Sora tested). | Approved, PR #1 | `docs/design/concepts/type-test.html` |
| D-006 | 2026-11-02 | `success` #2D7A4D and `warning` #976000 (from #2E7D4F and #9A6200) so status text passes 4.5:1 on ivory. | Approved, PR #1 | Computed WCAG 2 ratios, `docs/design/concepts.md` |
| D-007 | 2026-11-02 | One public site for both audiences; "Join as a contributor" is always the secondary action. | Approved, PR #1 | Storyboard and research notes |
| D-008 | 2026-11-02 | No `pages` table; only `stats` in the database. Page content in `frontend/content/`. | Approved, PR #1 (C3) | Brief section 5.2 (no CMS) |
| D-009 | 2026-11-02 | Route prefixes: public at root, contributor at `/c`, workspace at `/w`, shared auth at `/login`, `/join`, etc. | Approved, PR #1 | `docs/design/sitemap.md` |
| D-010 | 2026-11-02 | Stage 0 concept fonts are committed as woff2 copies in `docs/design/concepts/fonts/` (SIL OFL); the app uses npm fontsource packages. | Done | Concepts must render offline with real fonts |
| D-011 | 2026-11-02 | Stages 2, 5, 6, 9 and 13 are planned as a/b splits. | Approved, PR #1 | Usage limits; one sitting per PR |
| D-012 | 2026-11-02 | Local S3 storage is SeaweedFS 4.50 (`weed mini`), Compose service `storage`, instead of MinIO. | Proposed | MinIO images are no longer published (`minio/minio` and `quay.io/minio/minio` pulls fail: repository does not exist). SeaweedFS is Apache-2.0, S3-compatible with presigned URLs, and needs no setup script. The API only speaks S3, so R2 or Supabase Storage in staging is unaffected. CLAUDE.md and the brief still say MinIO. |
| D-013 | 2026-11-02 | Go 1.26 (go.mod and Docker build image). | Proposed | Current stable; goose v3.29.0 requires it. The `go` command fetches the toolchain automatically on machines with 1.25. |
| D-014 | 2026-11-02 | The rewrite strips the prefix: `/api/:path*` → `${API_ORIGIN}/:path*`. API routes live at the root (`/healthz`, later `/v1/...`). | Proposed | One prefix owned by the frontend; the API stays usable directly. |
| D-015 | 2026-11-02 | PostgreSQL 17 (meets "16+"), published on host port 5433 by default. | Proposed | Port 5432 is in use on the dev machine; configurable in `backend/.env`. |
| D-016 | 2026-11-02 | `/readyz` checks the database and object storage and returns 503 naming the failing check, without error details. | Proposed | Useful for Render health checks; avoids leaking internals. |
