# Visual concepts and recommended direction (Stage 0)

All three concepts use the same brand constraints from [DESIGN.md](DESIGN.md): the palette, the four type families, square corners, borders over shadows, no invented claims. They differ in **which surface leads, what carries the story, and how loud the accent is**.

## Concept 1: African Intelligence Infrastructure (as defined in DESIGN.md)

An engineering firm that happens to be African. Dark charcoal hero and ivory reading bands alternate down the page. The Signal Lattice and the converging-lines diagrams carry the story; photography supports it. Lime appears once per view as the point of agreement. Type is Space Grotesk at large sizes with one Instrument Serif word per headline. The product itself (demonstration window, quality diagram) is the main proof.

| Token sketch | |
|---|---|
| Surface split (public) | ~45% charcoal-950 / 55% ivory, alternating bands |
| Accent | lime ≤ 5% (CTA, agreement dot, key data on dark); laterite-ink for the serif accent on light |
| Display | Space Grotesk 500, 72 / 56 / 40, −0.02em |
| Data | JetBrains Mono for scores, IDs and money |
| Motif | Signal Lattice in hero, empty states, OG image; bracket corners on demos |
| Imagery | supporting, 4:5 and 3:2, one editorial spread |

## Concept 2: The Working Table

Built on the "meza" idea: a research publication more than a tech product. Light-first throughout (ivory and paper); charcoal only for the header, footer and one demonstration band. Documentary photography leads each page in large 21:9 bands. Laterite does the editorial work (rules, section numbers, pull-outs); lime survives only as the agreement dot and the primary button on a charcoal chip. Headlines lean more on Instrument Serif (still one or two words) and are set larger and looser.

| Token sketch | |
|---|---|
| Surface split (public) | ~85% ivory and paper / 15% charcoal |
| Accent | laterite and laterite-ink lead; lime < 2% |
| Display | Space Grotesk 500 at 64 / 48, −0.01em; serif accent at 1.15× |
| Data | Mono only inside product demonstrations |
| Motif | Signal Lattice small, as section dividers only |
| Imagery | leading; needs at least 8 strong, licensed, non-repeating photos at launch |

Strength: warm, human, distinctive among AI vendors. Weakness: depends on photography we do not yet have (Q8), reads softer to a security-minded buyer, and puts people ahead of the quality engine that is the actual product.

## Concept 3: Field Instrument

The platform as a precision instrument. Dark throughout on public pages (charcoal-950 to 800); content sits in thin-ruled panels like an instrument console. Mono type carries far more of the voice (labels, eyebrows, navigation). The hero is a live-looking product panel (labelled Demonstration) rather than a headline. Lime is used as the signal colour for every positive state; ochre for caution. Photography is minimal.

| Token sketch | |
|---|---|
| Surface split (public) | ~90% charcoal / 10% ivory (legal and long prose only) |
| Accent | lime up to ~8% (signal states), ochre secondary |
| Display | Space Grotesk 500 at 56 / 40, mono eyebrows on every block |
| Data | JetBrains Mono for most UI labels |
| Motif | Lattice as measurement ticks along panel edges |
| Imagery | rare; diagrams and UI only |

Strength: credible to technical buyers, cheap to produce (no photos needed). Weakness: cold; dark long-form text is tiring; too close to generic dev-tool and AI SaaS aesthetics (fails gate question 7 easily); breaks the "lime ≤ 5%" rule; hard to carry into the calm, ivory contributor portal.

## Recommendation: Concept 1

1. **It fits the product.** The quality engine is what we sell. Concept 1 makes the lattice, the demonstration and the quality diagram the proof, which also suits a pilot with no case studies.
2. **It works before we have photos.** Diagrams and demonstrations carry the story. Photography slots in later without a redesign (Concept 2 cannot launch well without it).
3. **It maps cleanly onto three surfaces.** The dark hero becomes the charcoal bars and sidebar, and ivory becomes the contributor and task canvas. Concept 3 has no natural light surface for contributors.
4. **It passes the design gate by construction.** Lime stays rare, sections alternate, and nothing depends on gradients, glass or bento grids.

Borrowed into Concept 1 from the others: from Concept 2, laterite-ink as the colour of the serif accent word on ivory, and a single editorial photo spread (Frame 4). From Concept 3, mono eyebrows used sparingly (hero eyebrow, section kickers only).

The high-fidelity concepts in [`concepts/`](concepts/) show Concept 1:

| File | Screens |
|---|---|
| [home.html](concepts/home.html) | Hero and services section: [1440](concepts/home-1440.png), [390](concepts/home-390.png), [390 reduced motion](concepts/home-390-reduced.png) |
| [contributor-dashboard.html](concepts/contributor-dashboard.html) | Dashboard with sample data: [1440](concepts/contributor-dashboard-1440.png), [390](concepts/contributor-dashboard-390.png) |
| [type-test.html](concepts/type-test.html) | Space Grotesk vs Manrope vs Sora: [1440](concepts/type-test-1440.png) |

Open the HTML files directly in a browser; they need no server.

## Type test result (D-005)

Space Grotesk stays as the display face. Manrope is too close to Inter, so display and body lose contrast. Sora is wider, so headlines wrap sooner, and its rounder shapes read friendlier and more consumer-startup. Space Grotesk's slightly quirky details (the flagged 1, the g) give an engineered feel without being cold, and its tabular-looking numerals hold up in big stats.

## Contrast check findings (D-006)

I recomputed every pair in DESIGN.md section 8 with the WCAG 2 formula. All stated ratios are correct. One gap was not covered: the status colours as **text on ivory** fall just short of 4.5:1.

| Token | Hex | On ivory | On paper | Proposal |
|---|---|---|---|---|
| success | #2E7D4F | 4.43 | 4.79 | #2D7A4D (4.61 on ivory, 4.99 on paper) |
| warning | #9A6200 | 4.48 | 4.84 | #976000 (4.63 on ivory, 5.00 on paper) |
| danger | #B3261E | 5.74 | 6.21 | unchanged |

The visual difference is negligible. The concept CSS already uses the proposed values; Stage 2 adopts them unless you object.

## Notes from building the concepts

- At 1440 the hero headline sets in four lines at 72 px (the serif word "verified." breaks onto its own line). It reads well as a deliberate rhythm. Stage 5 decides whether to keep it or widen the copy column.
- The mobile services accordion keeps the first row open so the page never shows a closed list on arrival.
- The dashboard uses `KES 00.00` for pay and `0,000.00` for earnings so the sample cannot be mistaken for real rates (Q3 is open).
