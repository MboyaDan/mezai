# Sitemap (pilot)

One Next.js app, three route groups, one Go API. Paths are proposals for Stage 1 onward. Items marked *hidden* exist in code but are not linked until their condition is met.

## Public website: `app/(public)`

| Path | Page | Notes |
|---|---|---|
| `/` | Home | Storyboard frames 1 to 8 (DESIGN.md section 10) |
| `/services` | Services | Sticky index; sections `#evaluation`, `#african-languages`, `#annotation`, `#expert-review`; language table from `docs/content/` only |
| `/quality` | How quality works | Qualify, gold, overlap, agreement, review, deliver; routing calculator |
| `/industries` | Industries | AI labs and model builders, telcos and banks, health, education and research, public sector and NGOs |
| `/case-studies` | Case studies index | *Hidden from nav* until one published record with client permission exists; shows "first pilots under way" |
| `/case-studies/[slug]` | Case study | Renders published records only |
| `/contributors` | For contributors | Who we look for, vetting, how pay works, terms summary, "Join" leads to `/join` |
| `/about` | About | ELS, the "meza" idea, honest pilot framing |
| `/contact` | Start a pilot / contact | Enquiry form, what happens next |
| `/legal/privacy` | Privacy notice | ELS as operating entity; versioned |
| `/legal/terms` | Terms of use | |
| `/legal/contributor-terms` | Contributor terms | Versioned; consent records point to the version |
| `/legal/acceptable-use` | Acceptable use (contributors) | |
| `/legal/cookies` | Cookie notice | |
| `/sitemap.xml`, `/robots.txt`, OG image | SEO | Concepts and `/design` are noindex |
| 404, 500 | Error pages | Custom, with lattice empty state |

Header nav: Services · Quality · Contributors · About · **Talk to us** (→ `/contact`). Industries and case studies sit in the footer (case studies hidden for now).

## Shared account routes: `app/(auth)`

| Path | Page |
|---|---|
| `/login` | Sign in (all user types; redirect by permissions) |
| `/join` | Contributor sign-up |
| `/verify-email` | Email verification landing |
| `/forgot-password`, `/reset-password` | Password reset |
| `/invite/[token]` | Accept a staff or client invitation |
| `/account` | Name, password, email preferences, sessions (all roles) |

## Contributor portal: `app/(contributor)`, prefix `/c`

| Path | Page | Permission |
|---|---|---|
| `/c` | Dashboard: projects, trust score, earnings, feedback, gold trend | `task.read_assigned` |
| `/c/onboarding` | Consent, then profile steps | `profile.manage` |
| `/c/profile` | Languages, dialects, domains, education, availability, M-Pesa number | `profile.manage` |
| `/c/qualify` | Qualification tests available and results | `qualification.take` |
| `/c/qualify/[testId]` | Take a test (task-workspace layout) | `qualification.take` |
| `/c/projects/[id]` | Project brief, guidelines, pay, start | `task.read_assigned` |
| `/c/work/[projectId]` | Task workspace (focus mode, one item at a time) | `task.submit` |
| `/c/quality` | Trust score detail, gold accuracy trend, feedback history | `earnings.read_own` (own data) |
| `/c/earnings` | Ledger: pending, approved, paid, payout history, statements | `earnings.read_own` |
| `/c/training` | Guidelines, worked examples, FAQs | any contributor |

Top bar: Dashboard · Tasks · Earnings · Training · account menu. On mobile: a bottom bar with the same four items.

## Workspace: `app/(workspace)`, prefix `/w`

The sidebar shows only what the caller's permissions allow, so Ops and client users share the shell and many routes. Client routes are always scoped to the caller's organisation.

| Path | Page | Who (permission) |
|---|---|---|
| `/w` | Overview: Ops gets queues, contributor health and deadlines; client gets their projects | all workspace users |
| `/w/projects` | Project list | Ops `project.manage`; client `org.project.read` |
| `/w/projects/new` | Create project | `project.manage` |
| `/w/projects/[id]` | Project overview: KPIs, progress, agreement by label, deliverables | Ops + client (client view hides contributor identities) |
| `/w/projects/[id]/settings` | Instructions, rubric, k, gold rate, pay, deadline, skills, client visibility, publish/pause/close | `project.manage` |
| `/w/projects/[id]/items` | Items and imports (CSV/JSONL, dry run, validation report) | `item.import` |
| `/w/projects/[id]/gold` | Gold set: import, retire, usage | `gold.manage` |
| `/w/projects/[id]/contributors` | Contributors on the project | Ops only |
| `/w/projects/[id]/datasets` | Source files, versions, import reports | client `org.project.read`; Ops |
| `/w/projects/[id]/deliverables` | Exports with manifest, signed downloads | `deliverable.publish`; client `org.deliverable.download` |
| `/w/requests` | Project requests (client submits; Ops converts) | `org.project.request`; Ops `project.manage` |
| `/w/review` | Review queue with filters | `review.read` |
| `/w/review/[itemId]` | Review and adjudicate (side-by-side) | `review.decide`, `review.adjudicate` |
| `/w/contributors` | Applicants, qualification approvals, status changes | `contributor.approve`, `contributor.pause` |
| `/w/contributors/[id]` | Contributor record, trust history, status log | Ops |
| `/w/organisations` | Organisations and members, invitations | Ops; client admin sees own org via `/w/team` |
| `/w/team` | Own organisation's users and invites | `org.user.invite` |
| `/w/payouts` | Payout batches | `payout.create`, `payout.approve` |
| `/w/payouts/[id]` | Batch review, approval (two-person), CSV export, record results | as above |
| `/w/enquiries` | Enquiries inbox, assign, notes, status | `enquiry.manage` |
| `/w/invoices` | Invoices (PDF upload by Ops; download by client) | Ops; client `org.project.read` |
| `/w/activity` | Audit/activity log (client: own org only) | Ops; client admin |
| `/w/settings` | Roles, permissions, settings keys (trust weights, thresholds, payout minimum) | `system.admin` |

## Dev only

| Path | Page |
|---|---|
| `/design` | Component gallery, light and dark, noindex, excluded from production build |
