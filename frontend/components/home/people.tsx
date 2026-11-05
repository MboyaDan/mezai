import Image, { type StaticImageData } from "next/image";
import { TextLink } from "@/components/ui/text-link";
import { Bracket } from "@/components/brand/bracket";
import { people } from "@/content/home";
import linguist from "@/content/images/people-linguist.jpg";
import engineer from "@/content/images/people-engineer.jpg";
import clinician from "@/content/images/people-clinician.jpg";
import { Container, Eyebrow, SectionTitle } from "@/components/public/layout";

type PhotoInfo = (typeof people.photos)[keyof typeof people.photos];

/**
 * One photo, captioned with the profiles it illustrates. Stock
 * photos are labelled illustrative (DESIGN.md section 8). GSAP wipes
 * [data-photo] in from the top-left corner (home-motion.tsx); without it the
 * photo just shows.
 */
function Photo({
  src,
  info,
  sizes,
  className,
  bracket = false,
}: {
  src: StaticImageData;
  info: PhotoInfo;
  sizes: string;
  className?: string;
  bracket?: boolean;
}) {
  const profiles = people.profiles.filter((p) => info.profiles.includes(p.name));
  const image = (
    <div data-photo className="overflow-hidden rounded-[2px] bg-surface">
      <Image src={src} alt={info.alt} sizes={sizes} placeholder="blur" className="h-auto w-full" />
    </div>
  );
  return (
    <figure className={className}>
      {bracket ? <Bracket>{image}</Bracket> : image}
      <figcaption className="mt-4 grid gap-3">
        {profiles.map((p) => (
          <span key={p.name} className="grid gap-1">
            <span className="font-display text-lg font-medium">{p.name}</span>
            <span className="text-sm text-fg-muted">{p.line}</span>
          </span>
        ))}
        <span className="font-mono text-[0.6875rem] text-fg-muted">Illustrative photo</span>
      </figcaption>
    </figure>
  );
}

/** Frame 4: an editorial spread, one 4:5 photo and two 3:2, off the grid. */
export function People() {
  const { photos } = people;
  return (
    <section aria-labelledby="people-title" className="border-t border-border bg-bg">
      <Container className="grid gap-12 py-20 md:py-32 lg:grid-cols-12 lg:gap-x-12 lg:gap-y-16">
        <div className="lg:col-span-5 lg:row-start-1" data-reveal>
          <Eyebrow>{people.eyebrow}</Eyebrow>
          <SectionTitle id="people-title" className="mt-4">
            {people.title}
          </SectionTitle>
          <p className="mt-6 max-w-md text-lg text-fg-muted">{people.intro}</p>
        </div>

        <Photo
          src={linguist}
          info={photos.main}
          sizes="(min-width: 1024px) 45vw, 100vw"
          className="lg:col-span-6 lg:col-start-7 lg:row-span-2 lg:row-start-1"
          bracket
        />

        <div className="lg:col-span-5 lg:row-start-2 lg:self-end">
          <h3 className="font-mono text-xs tracking-wide text-fg-muted">How vetting works</h3>
          <ol className="mt-4 grid gap-4" data-reveal-group>
            {people.steps.map((step, i) => (
              <li key={i} className="grid grid-cols-[2rem_1fr] items-baseline border-t border-border pt-4">
                <span className="font-mono text-sm text-fg-muted">0{i + 1}</span>
                <span>{step}</span>
              </li>
            ))}
          </ol>
          <TextLink href={people.link.href} className="mt-6 inline-block">
            {people.link.label} →
          </TextLink>
        </div>

        {/* The two smaller photos sit off the grid: one pulled left, one dropped. */}
        <div className="grid grid-cols-2 gap-4 sm:gap-8 lg:col-span-12 lg:grid-cols-12 lg:gap-12">
          <Photo
            src={engineer}
            info={photos.engineer}
            sizes="(min-width: 1024px) 38vw, 50vw"
            className="lg:col-span-5 lg:col-start-1"
          />
          <Photo
            src={clinician}
            info={photos.clinician}
            sizes="(min-width: 1024px) 30vw, 50vw"
            className="lg:col-span-4 lg:col-start-8 lg:mt-24"
          />
        </div>
      </Container>
    </section>
  );
}
