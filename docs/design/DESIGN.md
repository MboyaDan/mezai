# MezAI design direction

Moved unchanged from Part B of `CLAUDE.md` in Stage 0. Read this in every stage that touches UI. Stage 0 findings that refine it are in `docs/design/concepts.md` and `docs/DECISIONS.md`.

# PART B: DESIGN DIRECTION (move to `docs/design/DESIGN.md` in Stage 0)

## 7. Research references (study, never copy)

| Purpose | References | What to take |
|---|---|---|
| Business and positioning | https://scale.com/, https://outlier.ai/, https://mercor.com/, https://www.rws.com/ | How they explain services, proof and trust; navigation for two audiences (buyers and contributors). |
| African visual identity | https://tavatake.africa/, https://www.afrikadesigners.com/, https://themeshaper.com/sankofa/, https://www.jointabs.com/ | Contemporary African editorial confidence: type, composition, restraint. Not motifs to paste. |
| Task interfaces | https://github.com/HumanSignal/label-studio, https://github.com/cvat-ai/cvat, https://github.com/argilla-io/argilla, https://github.com/doccano/doccano | Task layouts, keyboard shortcuts, review side-by-side, dataset views. |
| Components and layout | https://ui.shadcn.com/, https://tailwindcss.com/plus, https://refero.design/, https://www.figma.com/community | Component behaviour, spacing, real product screens. |
| Motion (used) | https://gsap.com/, https://gsap.com/docs/v3/Plugins/ScrollTrigger/, https://motion.dev/, https://motion.dev/docs/react | Scroll storytelling (GSAP) and UI micro-interactions (Motion). |
| Motion (studied, deferred) | https://rive.app/, https://rive.app/use-cases/websites, https://lottiefiles.com/, https://lenis.darkroom.engineering/ | Study for ideas only. Not installed in the pilot (one fewer library each, native scroll kept). Candidates after the pilot: Rive for an interactive hero illustration. |
| Imagery | https://nappy.co/, https://unsplash.com/, https://www.pexels.com/, https://mixkit.co/ | Licensed African documentary photography; Mixkit only for an optional short, muted video loop (see section 8, Imagery). |
| Type | https://fonts.google.com/, https://www.typewolf.com/, https://fontsinuse.com/ | Pairings in real use. Test Space Grotesk, Inter, Manrope, Sora and Instrument Serif. |

Write a short `docs/design/research-notes.md` in Stage 0: five observations per group, each tied to a decision we make.

## 8. Visual identity: "African Intelligence Infrastructure"

**Brand personality.** Precise, warm, quietly confident. An engineering firm that happens to be African, not an African charity that happens to do engineering. We speak like a senior operator: short sentences, concrete verbs, numbers only when real. We never say "revolutionary", "cutting-edge", "unlock", "empower", "harness" or "seamless".

**The idea behind the identity.** Many careful human judgments converge into one verified answer. Everything visual comes back to that: marks that start scattered and align, lines that converge, a single lime dot where agreement is reached. "Meza" is Kiswahili for table: the place where people sit down and work something out together.

**Signature motif: the Signal Lattice.** A grid of short strokes (like tally marks or annotation ticks) that start at slightly different angles and rotate into alignment as a section comes into view. It is the brand's pattern, derived from the product (agreement), not borrowed from textiles. Use it at low contrast in hero backgrounds, section dividers, empty states and the OG image. Never as wallpaper behind body text.

**Supporting geometry.** The annotation bracket (two corner marks, like a bounding box) frames key headlines, photos and demo windows. Thin 1 px rules divide sections editorially. Corners are mostly square: radius 2 px for inputs and buttons, 6 px for cards at most, full radius only for pills and avatars.

**Colour tokens** (contrast computed with the WCAG 2 formula)

| Token | Hex | Use |
|---|---|---|
| `charcoal-950` | #0E100E | dark sections, public header, hero |
| `charcoal-900` | #171A16 | dark surfaces, workspace sidebar |
| `charcoal-800` | #22261F | dark cards, code and data panels |
| `ivory` | #F4F0E6 | light page background |
| `paper` | #FBF9F4 | light surfaces, task canvas |
| `ink` | #151714 | body text on light |
| `muted` | #5A5F55 | secondary text on light |
| `muted-dark` | #A3A99C | secondary text on dark |
| `line` | #DDD7C9 | borders on light |
| `lime` | #C8F031 | the accent: primary CTA fill, agreement dot, focus ring on dark, key data |
| `lime-ink` | #4A5A00 | accent-coloured text and icons on light |
| `laterite` | #C2522D | secondary accent (Kenyan red soil): data series, editorial highlights, fills |
| `laterite-ink` | #9E3F1F | laterite text on light |
| `ochre` | #D9A441 | data series, warnings on dark |
| `teal` | #2F8F83 | data series only |
| `success` / `warning` / `danger` | #2E7D4F / #9A6200 / #B3261E | status, always with icon and label |

| Pair | Ratio | Rule |
|---|---|---|
| Ivory on charcoal-950 | 16.8:1 | Main dark text |
| Lime on charcoal-950 | 14.5:1 | Accent text and fills on dark |
| Charcoal on lime | 14.5:1 | Text on lime buttons |
| Ink on ivory | 15.8:1 | Body text |
| Muted on ivory | 5.8:1 | Secondary text |
| Lime-ink on ivory | 6.7:1 | Accent text on light |
| Laterite-ink on ivory | 5.8:1 | Laterite text on light |
| Laterite on ivory or charcoal | 4.1:1 | Large text (24 px+) and fills only |
| Teal on ivory | 3.4:1 | Never text; data marks with labels only |
| **Lime on ivory or paper** | **1.2:1** | **Fails. Never use lime as text, icon or thin line on light backgrounds.** |

Primary button: lime fill, charcoal text, on both light and dark. On light pages the lime button sits on a charcoal band or gets a 1 px charcoal border so it does not disappear. Lime covers at most about 5 percent of any screen. No gradients except a very subtle radial glow behind the hero network on dark. No purple, no blue SaaS defaults, no glassmorphism.

**Data visualisation palette** (in order): lime (on dark) or lime-ink (on light), laterite, ochre, teal, muted. Every chart has direct labels or a legend, never colour alone. Agreement is always shown with the number and a plain-language band ("strong", "moderate", "weak").

**Typography** (all via fontsource, variable where available)

| Role | Family | Use |
|---|---|---|
| Display | Space Grotesk 500/600 | Hero, section titles, big numbers. Tight tracking (-0.02em) at large sizes. |
| Editorial accent | Instrument Serif italic | One or two words per headline for warmth ("Human judgment, *verified*."). Never body, never UI. |
| Body and UI | Inter (variable) | Everything else. 16 px minimum body on public pages, 14 px in dense workspace tables. |
| Data and IDs | JetBrains Mono | Item IDs, scores, kappa values, ledger amounts, keyboard hints. Tabular figures. |

Stage 0 tests Manrope and Sora as alternatives to Space Grotesk and records the decision. Type scale on public pages: 72/56/40/28/20/16/14 at desktop, 44/36/28/22/18/16/14 at 390 px. Line length 60 to 72 characters for prose.

**Imagery and art direction**
- Documentary, natural light, real working environments in African cities: people reading, discussing, annotating, teaching, coding, translating. Hands on keyboards and paper notes are as good as faces.
- Rich but restrained colour; slightly warm grade; consistent crops (4:5 portrait, 3:2 landscape, 21:9 bands). No image used twice across the site.
- Optional video: at most one short (under 10 s), muted, looping documentary clip from Mixkit or Pexels in the people section, with a poster image, never autoplayed on mobile or with reduced motion or Save-Data on.
- Sources: Nappy, Unsplash, Pexels, Mixkit, with the licence and photographer recorded in `docs/content/image-credits.md`. Prefer our own photos once we have consented shoots; the layout must work with them.
- Photos never sit next to a quote or claim attributed to that person. People in stock photos are "illustrative", not "our contributors".
- No map of Africa as a logo or decoration, no savanna sunsets, no generic "African pattern" borders, no emoji illustrations, no AI-generated people.
- Diagrams and UI demonstrations carry as much of the story as photos.

**Spacing, borders, shadows.** 4 px base, section padding 96 to 160 px desktop and 64 to 96 px mobile. Borders do the separating, not shadows: 1 px `line` on light, 1 px #2E332B on dark. One shadow level for floating layers (menus, dialogs, the demo window). Layouts are asymmetric on a 12-column grid: text often spans 5 columns, media 6 or 7, with deliberate empty columns.

**Three surfaces, one family**
- **Public site:** editorial, dark hero then alternating dark and ivory bands, big type, motion-led storytelling.
- **Contributor portal:** calm and focused, ivory and paper, larger targets, mobile-first, progress and earnings feel encouraging but honest. Charcoal top bar.
- **Workspace (Ops and client):** dense and data-first, charcoal sidebar with paper content area, tables, filters, charts in mono numerals. Desktop-first but usable at 390.

## 9. Motion language

**Principle.** Motion explains something or confirms something. If it does neither, remove it. The recurring story is convergence: scattered things settling into agreement.

**Libraries**
- **Motion (motion.dev, React)** for all interface interactions: enter and exit, layout changes, dialogs, toasts, tabs, hover and press, task submission feedback.
- **GSAP with ScrollTrigger, SplitText, DrawSVG and MotionPath** only on public marketing pages (home, services, quality), loaded on those routes only. GSAP and all its plugins are free for commercial use.
- No Lenis, Rive, Lottie or WebGL in the pilot. Native scrolling stays native. Revisit after the pilot.

**Timing and easing tokens**

| Token | Value | Use |
|---|---|---|
| `dur-instant` | 100 ms | press, toggle |
| `dur-quick` | 180 ms | hover, focus, small state changes |
| `dur-base` | 280 ms | enter, dialogs, tabs |
| `dur-slow` | 600 ms | section reveals |
| `dur-story` | 1200 to 2400 ms | scroll-driven sequences (scrubbed, not timed) |
| `ease-out` | cubic-bezier(0.16, 1, 0.3, 1) | entering elements |
| `ease-in-out` | cubic-bezier(0.65, 0, 0.35, 1) | moving and morphing |
| `ease-in` | cubic-bezier(0.7, 0, 0.84, 0) | leaving elements |

**Patterns**
1. **Enter:** fade from 0 plus 12 to 24 px rise, `dur-slow`, `ease-out`. Groups stagger by 60 ms, at most 8 items.
2. **Headline reveal:** SplitText by lines (not characters), each line rises from a clip mask, 80 ms stagger. The serif accent word lands last.
3. **Lattice alignment:** the Signal Lattice strokes rotate from random angles (±25°) to aligned as the section scrolls in (scrubbed). Used once per page at most.
4. **Converging lines:** DrawSVG strokes draw from several points into one node, which then fills lime. Used for the network map and the quality diagram.
5. **Hover:** buttons shift background 180 ms and the arrow icon moves 4 px; cards raise their border colour, never scale more than 1.01. No tilt, no glow.
6. **Press:** scale 0.98 for 100 ms.
7. **Section transitions:** dark to light bands use a 1 px rule that draws across the screen before the next section's content enters. No parallax on text.
8. **Data:** bars grow from the baseline, lines draw left to right, numbers count up only for real stats, once, then never again on that visit.
9. **Product feedback:** submitting a task collapses the card into a small tick that travels to the progress bar; a gold item result is never revealed in real time (it would expose which items are gold).
10. **Loading:** skeletons shaped like the content; no spinners longer than 300 ms before a skeleton appears.

**Never:** bouncing, spinning, continuously floating cards, autoplaying audio, scroll hijacking, horizontal scroll sections on mobile, animations that hide essential text, motion on workspace tables beyond row highlight.

**Reduced motion and performance.** With `prefers-reduced-motion: reduce`, every reveal becomes an instant state, the lattice renders aligned, the network renders fully drawn, the demo shows a static annotated sequence with Next/Back buttons. Animate only `transform` and `opacity` (and SVG stroke offsets). GSAP code is lazy-loaded below the hero. Kill ScrollTriggers on route change. Test on a mid-range Android (Chrome DevTools: Moto G Power profile, 4x CPU throttle).

## 10. Visual storyboard: the home page

Each frame is one section. "Sees" is what the visitor sees, "Moves" is the motion, "Says" is the copy direction (final copy goes in `frontend/content/home.ts`, short, no invented claims). Mobile notes follow each frame.

**Frame 1. Hero (charcoal).**
Sees: a large two-line headline on the left 7 columns ("Human judgment, *verified*. / Data for AI, built in Africa."), one-sentence subline, lime "Start a pilot project" button and a text link "Join as a contributor". On the right, the Signal Lattice in muted strokes with a single lime dot. A thin top bar: logo, Services, Quality, Contributors, About, "Talk to us".
Moves: headline lines rise from clip masks; the lattice strokes rotate into alignment over 1.6 s on load (not scroll); the lime dot fades in last. Buttons enter 200 ms after the headline.
Says: what we do in plain words. No numbers.
Mobile: stacked; lattice becomes a band under the headline; both actions full width.

**Frame 2. The network (charcoal, continues).**
Sees: a minimal dotted map with Nairobi as the origin node and a few African and global cities. Lines connect Nairobi and other African hubs to buyer cities. Beside it, three short lines of text: where expertise is, where models are built, how MezAI connects them.
Moves: scrubbed with scroll. DrawSVG draws arcs from African nodes outward, small pulses travel along them (MotionPath), then the arcs converge on one "verified dataset" node that fills lime.
Says: the connection, not a claim about scale. Cities shown are illustrative unless real project locations exist; label it "Illustrative".
Mobile: the map crops to Africa and Europe; arcs draw on enter rather than scrub.

**Frame 3. What we do (ivory).**
Sees: an asymmetric list of four services: AI evaluation and RLHF, African language data, data annotation, expert review. Each row has a short line and a tiny live illustration (not an icon): a pairwise comparison with two responses and a choice, a sentence with a Swahili and English line, a text snippet with label chips, a reviewer's tick.
Moves: rows enter with stagger; hovering a row plays its micro-illustration once (Motion). Bracket corners frame the active row.
Says: what the buyer gets from each service, one sentence each. Closing line: a quiet row of industries served (words, not logos) linking to the Industries page.
Mobile: rows become an accordion; illustrations play when a row opens.

**Frame 4. The people (ivory to charcoal).**
Sees: an editorial photo spread, one large 4:5 image and two smaller 3:2 images in an off-grid layout, with short captions about skills (linguists, engineers, clinicians, teachers). A line: "Vetted by test, measured by results."
Moves: images reveal through a clip-path wipe from the bracket corner; slight scale from 1.04 to 1 while revealing. No parallax.
Says: who contributes and how they are vetted. Photos are illustrative and credited.
Mobile: one large image, two small images in a row under it.

**Frame 5. See it work (charcoal).**
Sees: a framed product window labelled "Demonstration". Left: a contributor completes a Swahili–English translation review (scores adequacy, fixes a phrase, submits). The item moves to "Quality review", a reviewer accepts it, and the project's agreement meter ticks up. A tab switch shows the client view: a project created, progress bar, gold accuracy and agreement panels.
Moves: a scripted sequence driven by Motion with a visible timeline and Pause/Next controls. It plays when in view and pauses when out of view. Each step has a caption.
Says: what the client sees and what the contributor does. Demo data is fictional.
Mobile: same sequence, one panel at a time with step dots; autoplay off, tap to advance.

**Frame 6. How quality works (ivory).**
Sees: a horizontal process diagram: Qualify → Gold checks → Overlap → Agreement → Review → Deliver. Under it, a small live calculator: choose 3 annotators and see how disagreement routes an item to review. Plain-language definitions of gold items and Cohen's/Fleiss' kappa.
Moves: lines draw between steps as each comes into view (DrawSVG); the calculator animates the routing with Motion.
Says: the method, honestly. No accuracy promises.
Mobile: diagram becomes vertical.

**Frame 7. Security and how we work (charcoal band, short).**
Sees: three statements in a single row separated by thin rules: data stays on our platform and is item-scoped, contributors under NDA and contributor terms, data deleted after delivery on your schedule. Each links to the relevant page.
Moves: rule lines draw, text fades.
Says: only practices we actually implement (each tied to a feature in the brief). No certification claims.

**Frame 8. Final call to action (lime band).**
Sees: a full-width lime band with charcoal text: "Run a pilot with us." Two actions: "Start a pilot project" (charcoal button) and "Join as a contributor" (text link). The Signal Lattice aligned and tiny.
Moves: none beyond enter. Calm ending.
Footer (charcoal): logo, "MezAI is a division of ELS Software Solutions, Nairobi", links, legal.

**Other public pages** use the same language with their own compositions: Services (sticky left index with long sections), Quality (a scroll-explained diagram, frame 6 at full length), Contributors (editorial, requirements, how pay works, sign-up), About (short, honest pilot story, the "meza" idea), Contact (split layout: form and what happens next).

## 11. Key product screens

**Contributor dashboard.** Greeting and status line ("Active · Swahili, English · 3 projects available"). Left: available projects as rows (pay per task, estimated time per task, tasks left, required skill), each with "Start". Right column: trust score as a large mono number with a plain explanation and the three inputs (gold accuracy, agreement with consensus, review acceptance); earnings card (pending, approved, paid) with the next payout date if set; latest reviewer feedback. Below: gold accuracy trend (last 30 days, line). Empty state for a new contributor explains the qualification step with one action.

**Task workspace.** Full-screen focus mode. Top: project name, instructions link, progress (item 12 of your 40), timer, Skip, Report. Centre: the task. Pairwise: two responses side by side (stacked on mobile), rubric sliders or radio scales, preference choice, rationale field with minimum length. Classification: text with label chips and keyboard shortcuts 1 to 9. Translation: source and machine translation, adequacy and fluency scales, editable correction with diff view. Bottom bar: Save draft (auto), Submit (Enter with modifier). Shortcuts listed in a `?` dialog. Instructions open in a drawer, never a new page.

**Workspace project overview (Ops and client).** Header with project status and deadline. KPI row (completed / target, throughput per day, gold accuracy, agreement with band, review rate). Charts: progress over time, agreement by label. Tables: items needing review (Ops only), contributors on the project (Ops only), deliverables. Client users never see contributor identities; they see contributor IDs at most.

**Review screen.** Item content on the left, all submissions as columns with contributor ID, trust score, time on task; disagreements highlighted; actions accept consensus, choose one, write final label, reject with reason; keyboard driven.

## 12. Design quality gate (run before any UI stage is "done")

Check every changed page at 1440 and 390, light and dark where relevant, and answer in the stage report:

1. Does every section have a visual purpose, and is no composition repeated twice on one page?
2. Does the type create a clear hierarchy, with the serif accent used sparingly?
3. Is lime used only where the rules allow, and never as text on light?
4. Are images relevant, credited, consistent in grade and crop, and never reused?
5. Is every animation meaningful, smooth (no dropped frames in the performance panel) and fully replaced in reduced-motion mode?
6. Is every number real or clearly a demonstration?
7. Does it look recognisably different from a generic AI SaaS template (no purple gradients, no bento grid, no glass cards, no emoji icons, no three identical feature cards)?
8. Does it communicate enterprise credibility without claiming anything we cannot prove?

If any answer is no, fix it before opening the PR, and say what you changed.

The goal is not maximum animation. The goal is an unmistakable, memorable African technology brand that a careful enterprise buyer trusts.
