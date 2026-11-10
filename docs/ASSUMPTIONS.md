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
| A-024 | 2026-11-03 | Photos for frame 4 (the people) | None until credited photos are in `docs/content/` with `image-credits.md`; the frame is typographic and describes who we look for, not who our contributors are | Resolved 2026-11-04: photos arrived; frame 4 uses the photo spread (D-053) |
| A-025 | 2026-11-03 | Cities on the network map | Nairobi, Lagos, Kigali, Johannesburg, London, Berlin, New York, San Francisco, Bengaluru, all labelled illustrative | Open |
| A-026 | 2026-11-03 | Links on the home page to pages not built yet (Services, Quality, Contributors, Industries, Contact, legal) | Kept; they 404 until Stage 6 builds them, and the sitemap lists only built pages | Open |
| A-027 | 2026-11-03 | Headline reveal | Lines rise from clip masks in CSS (not GSAP SplitText), so the hero never waits for a script; the storyboard's lines are fixed, so splitting by line needs no measuring | Open |
| A-028 | 2026-11-04 | Photo originals (19 MB) | Kept out of git (`docs/content/images/` is ignored); credits link to each source page so they can be re-downloaded; only cropped web masters are committed | Open |
| A-029 | 2026-11-05 | Language table | English, Kiswahili and code-switched Kiswahili and English shown as available (code-switching mixes the two confirmed languages, as the home page already says); other African languages "on request". Sheng is not listed separately | Open |
| A-030 | 2026-11-05 | Links to pages not built yet (`/contact`, `/legal/*` in 6b; `/join`, `/login` in Stage 7) | Kept; they 404 until those stages | Closed in 7b (all exist) |
| A-031 | 2026-11-05 | `contributors-phone.jpg` | The phone screen shows a small third-party app logo (the photo's source app). Kept: tiny and not our brand. Replace if Dan prefers | Open |
| A-032 | 2026-11-06 | Retention for enquiries that do not become a project | 24 months after the last contact; stated in the draft privacy notice and listed as an open point for the advocate | Open |
| A-033 | 2026-11-06 | Reply time promised to enquirers | None; the page says a person reads every enquiry and replies by email, with no deadline | Open |
| A-034 | 2026-11-06 | Draft legal text | Written from the brief and from what the software does, shown publicly with a "Draft, not yet in force" banner and open points until DG-05 | Open |
| A-035 | 2026-11-06 | Retaking a qualification test after a rejection | Allowed 7 days after the decision (`contributors.RetakeCooldown`); a passed and approved test cannot be retaken | Open |
| A-036 | 2026-11-06 | Documents a contributor must accept | Privacy notice and contributor terms (brief 6.2). Acceptable use and terms of use can be accepted through the same endpoint but are not required | Open |
| A-037 | 2026-11-06 | Failed attempts | Still go to the Ops queue, so a person writes the note the contributor sees; Ops cannot approve a failed attempt | Open |
| A-038 | 2026-11-06 | M-Pesa numbers | Kenyan mobiles only (07…, 01…, 254…, +254…), stored as E.164, shown masked (`+254 7•• ••• 678`) everywhere including to the owner; entering it twice counts as confirmation (no SMS check until DG-06) | Open |
| A-039 | 2026-11-06 | Profile fields required before qualifying | Display name, country, education level, weekly hours above 0, at least one language and one domain, and a confirmed M-Pesa number. City and dialects are optional | Open |
| A-040 | 2026-11-06 | Language and country choices on the profile | A fixed list in `content/profile.ts` (19 languages, 9 countries, Kenya first). These are choices for a person, not a claim of coverage; more can be added on request | Open |
| A-041 | 2026-11-06 | Sign-in and sign-up pages | `noindex`, like the portals; `/contributors` remains the public entry that links to `/join` | Open |
| A-042 | 2026-11-06 | Qualification UI | One question at a time, autosave 0.8 s after a change, no timer, Ctrl/Cmd+Enter for next or submit, number keys for classification labels | Open |
| A-043 | 2026-11-06 | Import file limits | 10 MB and 20,000 rows per file; split larger datasets into several files | Open |
| A-044 | 2026-11-06 | Gold columns in CSV | Classification `expected_labels` (several separated by `|`); pairwise `expected_preference` (a, b or tie); translation `expected_adequacy`, `expected_fluency`, optional `expected_corrected`, `tolerance_adequacy`, `tolerance_fluency` (0 to 4, default 1). JSONL uses `expected` and `tolerance` objects | Open |
| A-045 | 2026-11-06 | Default QA checks | Warn under 5 seconds (all types); block an unchanged translation (translation review); warn on a rationale that contradicts the preference (pairwise); warn on pasted free text (pairwise, translation review). Ops can change them per project | Open |
| A-046 | 2026-11-06 | Uploaded files | Import files and request samples are kept in object storage as dataset versions and deleted with the project's task content under the retention rule (brief 12, Stage 17). A file stored before a failed database write is left orphaned until the Stage 17 cleanup job | Open |
| A-047 | 2026-11-06 | Client notified of request decisions | The client sees the status and note in the workspace; the email arrives with notifications in Stage 16 | Open |
| A-048 | 2026-11-07 | Client item count | For client users the API's `item_count` includes gold items, because gold rows are hidden from them by RLS so the "not gold" filter cannot see them. The client overview labels it "Items, with gold checks". Default: keep, and decide the client figure in Stage 13 (dashboards). | Open |
| A-049 | 2026-11-07 | Instructions display | Project instructions and request briefs are shown as plain text with line breaks; Markdown is not rendered yet (no renderer dependency). Default: render Markdown in Stage 9 with the task screen. | Open |
| A-050 | 2026-11-08 | National ID format | A Kenyan National ID number is 6 to 9 digits once spaces, dashes, dots and leading zeros are removed. Passports and alien IDs are not accepted in the pilot (contributors need a Kenyan M-Pesa number anyway). | Widen the format if non-citizen residents join. |
| A-051 | 2026-11-08 | Name for the M-Pesa check | The profile's display name is the person's full name as registered with M-Pesa; the form's hint says so. There is no separate legal-name field. | Add a legal-name field if people want a shorter display name. |
| A-052 | 2026-11-08 | Contributors from before 7c | Existing (sample) contributors keep their status with identity pending; there are no real contributors yet. The seed marks its sample contributors past qualifying as verified, with fingerprints of `sample-NNN` (no real ID can produce them). | None needed before launch. |
| A-053 | 2026-11-08 | Fingerprint after closure | The fingerprint and last 3 digits are kept after an account is removed, to stop the same person joining again. The retention period is an open point for the advocate (privacy notice 0.2); Stage 17's deletion tooling must decide whether to keep them. | Set the period with the advocate. |
| A-054 | 2026-11-09 | Late submissions | A submission after the reservation expired is refused (410 `reservation_closed`); 9b shows the time left and autosaves, so little work is lost. | Add a grace period if contributors lose work in practice. |
| A-055 | 2026-11-09 | Paste and timing signals | `time_on_task_ms` and `pasted_chars` come from the contributor's browser, so they only ever produce warnings, never blocks. | None. |
| A-056 | 2026-11-09 | Sample qualifications | The seed approves every sample test for active and paused sample contributors, and the pairwise sample project requires the "pairwise" task-type skill (no test grants English). | Dev data only. |
| A-057 | 2026-11-09 | Contributors from before 7d | Active and paused contributors were marked shortlisted by migration 00013 and keep working without an interview record. The seed gives sample contributors a passed "(Sample)" interview. | Interview them before real payouts if any exist (none do yet). |
| A-058 | 2026-11-09 | Interview languages | Ops scores the languages on the contributor's profile only (at least one). Two everyday questions per language, 1 to 4 with written anchors in the script. | Add fixed question banks per language if scores vary between interviewers. |
| A-059 | 2026-11-09 | No-answer calls | "No answer" calls are recorded with no limit; after three, Ops decides whether to decline. | Add a rule if this is abused. |
| A-060 | 2026-11-10 | Core check content | The seven items in `corecheck.go` (English customer messages, two spot-the-error items, one Swahili translation item, one pairwise comparison, one writing prompt) are a first version for Dan and Ops to review before activating the real core check. | Edit `corecheck.go` and recreate the draft. |
| A-061 | 2026-11-10 | Contributors with skills but no core check | Contributors who are already active are not gated by the core check (they were active before it existed). A qualifying applicant who passed a skill test before 7d-2 still needs the core check to become active. Only sample data is affected. | None. |
| A-062 | 2026-11-10 | Load the pilot needs | 50 contributors who each spend about 20 s on a task make about 2.5 next-and-submit cycles a second. Locally the engine sustained about 350 a second with 60 concurrent workers (Docker PostgreSQL on a laptop, no network). Staging on Supabase will be slower because of network round trips; Stage 18 measures it there. | Re-measure on staging (Stage 18). |
| A-063 | 2026-11-10 | Pool size and pooler | 20 connections per API instance. Supabase's pooler and plan limit total connections; with more than one API instance, set `pool_max_conns` so instances × pool stays under that limit (Stage 19). | Adjust in Stage 19. |
| A-064 | 2026-11-10 | Agreement for translation review | A rater "agrees with the final answer" when their adequacy and fluency are each within 1 point of the consensus mean. Pairwise agreement is the preference only. | Revisit with real data in Stage 11 reviews. |
| A-065 | 2026-11-10 | Reinstating after an automatic pause | Ops can reinstate with a note (existing status endpoint). The next recalculation pauses again if the score is still below the floor, so Ops should look at the work first. | Add a grace period if Ops ask for one. |
| A-066 | 2026-11-10 | Rejected items and pay | An item Ops rejects (bad source data) pays its honest answers, because the contributors did the work. It gets no final answer and is left out of the delivery (11b). | Change to unpaid if Dan prefers. |
| A-067 | 2026-11-10 | Stage 10 dev data | Migration 00017 makes local and staging items left at `consensus` final and accepts and pays their answers and any open gold answers, as the new rules would have. Production has no such rows yet. | None. |
| A-068 | 2026-11-10 | Agreement after review | "Agrees with final" after a review uses brief 8.1's comparison rules against the chosen or written answer (translation: adequacy and fluency within 1; corrected text is not compared). | Revisit with real reviews. |
