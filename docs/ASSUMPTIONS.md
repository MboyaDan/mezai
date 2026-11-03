# Assumptions

Defaults in use until Dan answers. When an answer arrives, move it to `DECISIONS.md` and mark the row "Answered".

| ID | Date | Question | Default in use | Status |
|---|---|---|---|---|
| A-001 | 2026-11-02 | Q1 Languages ELS can handle | English and Kiswahili on the public site; others "on request" | Open |
| A-002 | 2026-11-02 | Q2 Public contact channels | Email only, from `docs/content/`; hidden while that folder has none | Open |
| A-003 | 2026-11-02 | Q3 Default pay per task | Configurable per project; no public figures; samples show `KES 00.00` | Open |
| A-004 | 2026-11-02 | Q4 Contributors see client name | No, unless the project enables it | Open |
| A-005 | 2026-11-02 | Q5 Domain | Vercel and Render default domains until DG-01 | Open |
| A-006 | 2026-11-02 | Q6 Kiswahili contributor pages | English only, i18n-ready strings | Open |
| A-007 | 2026-11-02 | Q7 Second payout approver | Test user in development; real person needed before Stage 14 goes live | Open |
| A-008 | 2026-11-02 | Q8 Photography | Licensed stock with credits; layout ready for own photos | Open |
| A-009 | 2026-11-02 | Q9 Retention after close | 90 days, configurable per project | Open |
| A-010 | 2026-11-02 | Q10 pandoc not installed | Brief read by extracting `word/document.xml` from the docx | Open |
| A-011 | 2026-11-02 | Q11 Repo is public | Stays public until before real client data | Open |
| A-012 | 2026-11-02 | Research references | Notes written from prior familiarity, sites not browsed in Stage 0; re-check in Stage 5 | Open |
| A-013 | 2026-11-03 | Brief 8.2 leaves 0.40 to 0.41 (and the other band edges) ambiguous | A boundary value falls in the lower band: 0.40 weak, 0.60 moderate, 0.80 substantial | Open |
| A-014 | 2026-11-03 | Reserved Kenyan test phone numbers | No officially reserved range found; the seed uses `+2547000000NN` placeholders, never sends SMS or payments, and masks numbers wherever shown | Open |
| A-015 | 2026-11-03 | Client Viewer permissions (the brief's table row is cut off) | `org.project.read` and `org.deliverable.download` only | Open |
| A-016 | 2026-11-03 | Contributors see project metadata for every live project | Yes, at the database; eligibility by skill is filtered in the API (Stage 9). Client organisation names stay hidden (no access to `organisations`) | Open |
| A-017 | 2026-11-03 | Can one person be both staff and client, or client in two organisations? | No. Each user has one kind (staff, client or contributor) and a client user belongs to one organisation; mixed grants refuse sign-in with `account_not_ready` | Open |
| A-018 | 2026-11-03 | Lockout rule | 5 wrong passwords lock the account for 15 minutes; a correct sign-in resets the count | Open |
| A-019 | 2026-11-03 | Rate limits | In memory, per API instance: login 20 a minute per IP and 10 per 15 minutes per email. Fine for one instance; move to Postgres if we run more than one | Open |
| A-020 | 2026-11-03 | Session lifetime | 7 days absolute, signed out after 48 hours without a request (`SESSION_TTL`, `SESSION_IDLE`) | Open |
| A-021 | 2026-11-03 | Link lifetimes | Verification 24 hours, password reset 1 hour, invitation 7 days; a new invitation to the same address replaces the pending one | Open |
| A-022 | 2026-11-03 | Inviting someone who already has an account | Refused (409 `email_in_use`); grant them a role instead | Open |
| A-023 | 2026-11-03 | Pages behind the email links (`/reset-password`, `/verify-email`, `/accept-invitation`) and the user administration screens | Built with the first UI stage that needs them (Stage 7 for contributors, Stage 8 for the workspace); the API is ready now | Open |
| A-024 | 2026-11-03 | Photos for frame 4 (the people) | None until credited photos are in `docs/content/` with `image-credits.md`; the frame is typographic and describes who we look for, not who our contributors are | Open |
| A-025 | 2026-11-03 | Cities on the network map | Nairobi, Lagos, Kigali, Johannesburg, London, Berlin, New York, San Francisco, Bengaluru, all labelled illustrative | Open |
| A-026 | 2026-11-03 | Links on the home page to pages not built yet (Services, Quality, Contributors, Industries, Contact, legal) | Kept; they 404 until Stage 6 builds them, and the sitemap lists only built pages | Open |
| A-027 | 2026-11-03 | Headline reveal | Lines rise from clip masks in CSS (not GSAP SplitText), so the hero never waits for a script; the storyboard's lines are fixed, so splitting by line needs no measuring | Open |
