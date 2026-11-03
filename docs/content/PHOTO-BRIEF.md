# Photo brief

What to find, where it goes, and how to hand it over. Rules come from `docs/design/DESIGN.md` section 8.

## The look, for every photo

- **Documentary, not posed.** Real people doing real work in African cities: reading, discussing, annotating, teaching, coding, translating. Nobody smiling at the camera or pointing at a screen.
- **Natural light**, slightly warm, rich but not saturated colour. Indoor daylight from a window is ideal.
- **Hands and work count as much as faces.** Hands on a keyboard, a pen on notes, a finger tracing a line of text: all good.
- **Ordinary modern settings:** an office, a co-working space, a university library, a home desk, a clinic office, a classroom.
- **Never:** savanna or sunsets, safari animals, maps of Africa, "African pattern" borders, people holding hands in a circle, handshakes, thumbs up, staged "diverse team high-five" shots, VR headsets, glowing brains or robots, AI-generated people.
- **No visible brands or logos** (laptops, T-shirts, mugs), **no readable screens** with someone else's data, **no children**.
- Each photo is used **once** across the site.

## Where to get them

Free licences that allow commercial use without payment:

| Site | Search tips |
|---|---|
| [Nappy](https://nappy.co) | Strong on Black people at work; try "office", "laptop", "studying", "meeting" |
| [Unsplash](https://unsplash.com) | Add a city: "Nairobi office", "Lagos coworking", "Kigali laptop", "Accra student" |
| [Pexels](https://www.pexels.com) | "African woman working laptop", "Kenyan student library", "doctor notes tablet" |

Prefer photographers based in Kenya, Nigeria, Ghana, Rwanda, Uganda, South Africa or Ethiopia; their portfolios usually look like the real places. Our own consented photos will replace these later, and the layouts are built to take them.

## The photos, by page

Sizes are minimums for the long edge. Download the largest version available.

### Home page, "The people" section (needed now)

| # | File name | Shape | What it shows | Search ideas |
|---|---|---|---|---|
| H1 | `home-people-main.jpg` | Portrait 4:5, 2000 px tall | **The hero shot.** One person concentrating on a laptop or notes, natural window light, face partly visible or in profile. Linguist or translator mood: reading, a notebook beside the laptop. | "woman reading laptop window light", "man writing notes laptop Nairobi" |
| H2 | `home-people-engineer.jpg` | Landscape 3:2, 2400 px wide | An engineer or developer at work: hands on keyboard, code on screen out of focus, or two people looking at one screen. | "developer laptop Africa", "coding hands keyboard Lagos" |
| H3 | `home-people-review.jpg` | Landscape 3:2, 2400 px wide | Careful checking: a health worker or teacher reading a printed page or tablet, pen in hand, marking something. | "nurse reading notes", "teacher marking papers", "woman reviewing documents pen" |

Captions will name the skill ("Linguists and translators", "Engineers", "Clinicians and teachers") and the photos are labelled **Illustrative**. They will not be called "our contributors".

### Contributors page (Stage 6)

| # | File name | Shape | What it shows |
|---|---|---|---|
| C1 | `contributors-band.jpg` | Wide 21:9, 2800 px wide | A calm wide shot of a co-working space or library with several people working separately at their own laptops. |
| C2 | `contributors-detail.jpg` | Portrait 4:5, 2000 px tall | Close-up: hands typing on a laptop with a phone beside it (people work from phones and laptops). |

### About page (Stage 6)

| # | File name | Shape | What it shows |
|---|---|---|---|
| A1 | `about-meza.jpg` | Landscape 3:2, 2400 px wide | A table ("meza") with two or three people working through something together: notes, laptops, discussion. Matches the "meza" idea in the brand story. |

### Industries page (Stage 6, optional)

One landscape 3:2 photo each, only if a good one exists. Text-only works too.

| # | File name | What it shows |
|---|---|---|
| I1 | `industry-health.jpg` | A clinic office or health worker with a tablet or file, no patients identifiable |
| I2 | `industry-education.jpg` | A lecture hall, library or teacher at a whiteboard |
| I3 | `industry-telco-bank.jpg` | A customer service desk or call-centre agent with a headset |

### Optional short video (home "people" section)

| # | File name | What it shows |
|---|---|---|
| V1 | `home-people-loop.mp4` plus a poster `home-people-loop-poster.jpg` | Under 10 seconds, no sound: hands typing, a page turning, someone writing a note. From [Mixkit](https://mixkit.co) or Pexels Videos. It never autoplays on mobile or with reduced motion. |

## How to hand them over

1. Put the files in `docs/content/images/` with the file names above.
2. For every file, add a row to `docs/content/image-credits.md`: file name, photographer, source site, the page link, licence, and the date you downloaded it.
3. If a person is clearly identifiable and the photo is **ours** (not stock), we also need their written consent, stored outside the repo.
4. Tell me in the next message. I will crop to the shapes above, compress, add alt text, and switch the people section to the photo layout.

You do not need all of them at once. H1 to H3 unlock the home page; the rest can come with Stage 6.
