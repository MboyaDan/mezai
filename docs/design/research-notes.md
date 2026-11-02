# Research notes (Stage 0)

**Method and limits.** These notes come from prior familiarity with the references in DESIGN.md section 7. I did **not** browse the sites in this session (to save usage), so details may be out of date. Each note records a pattern, never text or assets, and ends with the decision it supports. Stage 5 should re-check the business and motion references live before building the home page.

## Business and positioning (Scale, Outlier, Mercor, RWS)

1. Two audiences, two doors: buyers and workers get separate top-level paths, often with separate sub-brands. → One site, but "Join as a contributor" is always the secondary action and `/contributors` has its own editorial voice (D-007).
2. Proof is mostly logos, scale numbers and named customers. We have none, so copying the pattern would be empty or dishonest. → Proof by method: the demonstration window and quality diagram replace the logo wall.
3. Large vendors describe services by model lifecycle stage (evaluation, RLHF, safety), not by task mechanics. → Services are named by buyer outcome; task types (classification, pairwise, translation) appear inside each section.
4. Worker-facing pages lead with flexibility and pay, sometimes vaguely. → Our contributor page states how pay works (per approved task, M-Pesa, minimum and schedule from settings) and shows no rates until Q3 is answered.
5. Enterprise trust pages list security practices, often with certifications. → Frame 7 lists only practices tied to built features; no certification language anywhere (AC-03).

## African visual identity (Tavatake, Afrika Designers, Sankofa, Tabs)

1. The strongest contemporary African design work is confident and restrained: big type, generous space, one or two warm accents. → Charcoal and ivory dominate; laterite and ochre are accents, not backgrounds.
2. Pattern is used as structure, not decoration. → The Signal Lattice is derived from the product (agreement) and stays at low contrast.
3. Documentary photography with natural light reads as more credible than staged stock. → Art direction rules in DESIGN.md; image credits file required.
4. Local language appears naturally in UI copy, not as a gimmick. → One Kiswahili greeting ("Habari") in the contributor portal; public copy stays English (Q6).
5. Editorial layouts use asymmetric grids and deliberate empty columns. → 12-column asymmetric layouts (5 / 7 splits), visible in the services concept.

## Task interfaces (Label Studio, CVAT, Argilla, doccano)

1. The best labelling UIs are keyboard-first: number keys for labels, a single submit shortcut, a visible shortcut list. → Keys 1 to 9 for labels, A/T/B for preference, ⌘↵ submit, `?` dialog.
2. Instructions belong beside the task, not on another page. → Instructions drawer in the task workspace.
3. Side-by-side layout is standard for comparison and review; independent scroll per column matters for long responses. → Pairwise and review screens use columns with independent scroll.
4. Dataset views separate "items" from "annotations" and show per-item status. → Workspace items table shows item state from the brief's state machine.
5. Review tools highlight disagreement rather than showing everything equally. → Review screen highlights differing fields and shows the agreement number.

## Components and layout (shadcn/ui, Tailwind Plus, Refero, Figma Community)

1. Accessible primitives (dialog, tabs, popover) are hard to get right by hand. → Stage 2 builds on headless primitives (Radix or React Aria; choose in Stage 2) with our own styling.
2. Real product screens rely on tables, filters and empty states more than on cards. → Workspace is table-first; every table has an empty state.
3. Consistent density modes separate marketing from tools. → Public 16 px body; workspace 14 px tables.
4. Status is shown with text plus icon plus colour. → StatusPill always has an icon and a label.
5. Generic templates are recognisable (bento grids, gradient blobs, three-card rows). → Design gate question 7; services use a ruled list, not cards.

## Motion (GSAP, ScrollTrigger, Motion)

1. Scroll-scrubbed sequences work when they explain a process; they annoy when they only decorate. → Scrubbing is limited to the network (F2) and quality diagram (F6).
2. Line-by-line headline reveals stay readable; character-by-character reveals do not. → SplitText by lines only.
3. Heavy scroll libraries are a performance cost on mid-range phones. → GSAP loads only on marketing routes, after first paint.
4. UI micro-interactions feel best at 150 to 300 ms with strong ease-out. → Motion tokens 100 / 180 / 280 ms.
5. Reduced-motion fallbacks are often forgotten. → Every sequence has a static end state; verified for the hero lattice in the concept (home-390-reduced.png).

## Motion (studied, deferred: Rive, Lottie, Lenis)

1. Rive suits interactive illustrations with state machines. → Candidate for the hero after the pilot.
2. Lottie files are often heavy and hard to theme. → Not used; SVG plus GSAP instead.
3. Smooth-scroll libraries change native scroll feel and can break accessibility tools. → Native scroll stays.
4. Most of what these tools add can be done with SVG for this scope. → Lattice and diagrams are SVG components.
5. Each extra runtime adds bundle weight against the 170 KB home budget. → One fewer library each.

## Imagery (Nappy, Unsplash, Pexels, Mixkit)

1. Free libraries have good African documentary photography, but the same images appear on many sites. → Prefer less-used images and plan a consented shoot (Q8).
2. Licences differ in attribution expectations. → Record photographer, source and licence in `docs/content/image-credits.md`.
3. Stock people photos imply "our team" if captioned carelessly. → Captions describe skills, marked "Illustrative".
4. Video loops are heavy on mobile data. → At most one loop, never on mobile, Save-Data or reduced motion.
5. Mixed grades look cheap. → One warm grade and fixed crops (4:5, 3:2, 21:9).

## Type (Google Fonts, Typewolf, Fonts In Use)

1. Grotesque display plus serif-italic accent is a current editorial pairing; it stays fresh only when the serif is rare. → One or two serif words per headline.
2. Inter is safe and highly legible at small UI sizes. → Inter for body and UI.
3. Mono for numbers signals precision and keeps columns aligned. → JetBrains Mono for scores, IDs, money.
4. Manrope and Sora were tested against Space Grotesk in `concepts/type-test.html`. → Space Grotesk kept (D-005).
5. Self-hosting removes a third-party request and a privacy question. → fontsource packages only.
