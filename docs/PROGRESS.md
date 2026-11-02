# Progress

Read this first after `/clear`. Then read only what the next stage needs.

## Current state

- **Last stage:** 0 (Orientation, design direction and plan). Branch `stage-00-orientation`, PR open, waiting for review.
- **Next stage:** 1 (Repo scaffold and tooling), after the Stage 0 PR is merged and the direction (D-002) is approved.
- **Blocking questions:** none for Stage 1. Open questions Q1 to Q11 and conflicts C1 to C4 are in `docs/design/build-plan.md`; defaults are in `docs/ASSUMPTIONS.md`.

## Where things are

| What | Where |
|---|---|
| Design direction (was CLAUDE.md Part B) | `docs/design/DESIGN.md` |
| Concepts and recommendation | `docs/design/concepts.md`, HTML and screenshots in `docs/design/concepts/` |
| Sitemap and routes | `docs/design/sitemap.md` |
| Wireframes | `docs/design/wireframes.md` |
| Plan, questions, risks | `docs/design/build-plan.md` |
| Research notes | `docs/design/research-notes.md` |
| Decisions / assumptions / AC map | `docs/DECISIONS.md`, `docs/ASSUMPTIONS.md`, `docs/traceability.md` |

## Stage log

| Stage | Status | PR | Notes |
|---|---|---|---|
| 0 | PR open | (see PR list) | No feature code. Brief read by extracting docx XML (pandoc missing). Concept tokens in `docs/design/concepts/tokens.css` are the starting point for Stage 2. |

## Notes for the next session

- The brief can be read without pandoc: `unzip -p docs/brief.docx word/document.xml` and strip tags (see A-010).
- Stage 2 should start from `docs/design/concepts/tokens.css` (includes D-006 status colours).
- Planned splits: 2, 5, 6, 9, 13 (D-011). Confirm with Dan before starting each.
