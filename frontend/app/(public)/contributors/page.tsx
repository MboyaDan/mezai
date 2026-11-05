import type { Metadata } from "next";
import { Container, Eyebrow, SectionTitle } from "@/components/public/layout";
import { PageHeader, titleText } from "@/components/public/page-header";
import { CtaBand } from "@/components/public/cta-band";
import { IllustrativePhoto } from "@/components/public/photo";
import { ButtonLink } from "@/components/ui/button";
import { TextLink } from "@/components/ui/text-link";
import { contributorsPage } from "@/content/contributors";
import room from "@/content/images/contributors-room.jpg";
import phone from "@/content/images/contributors-phone.jpg";

export const metadata: Metadata = {
  title: "For contributors",
  description: contributorsPage.intro,
  alternates: { canonical: "/contributors" },
  openGraph: { title: titleText(contributorsPage.title) },
};

export default function ContributorsPage() {
  const { lookFor, steps, pay, measured, terms, photos, join, signIn } = contributorsPage;
  return (
    <>
      <PageHeader
        eyebrow={contributorsPage.eyebrow}
        title={contributorsPage.title}
        intro={contributorsPage.intro}
      >
        <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:gap-6">
          <ButtonLink href={join.href} size="lg" arrow className="max-sm:w-full">
            {join.label}
          </ButtonLink>
          <TextLink href={signIn.href} className="max-sm:text-center">
            {signIn.label}
          </TextLink>
        </div>
      </PageHeader>

      {/* 21:9 band: the one wide photo on the page. */}
      <div data-theme="dark" className="bg-bg">
        <Container className="pb-12 md:pb-20">
          <IllustrativePhoto src={room} alt={photos.room.alt} sizes="(min-width: 1312px) 1184px, 100vw" />
        </Container>
      </div>

      <section aria-labelledby="look-title" className="bg-bg">
        <Container className="grid gap-12 py-16 md:py-24 lg:grid-cols-12">
          <div className="lg:col-span-4">
            <SectionTitle id="look-title" className="lg:text-display-3">
              {lookFor.title}
            </SectionTitle>
          </div>
          <dl className="grid gap-0 lg:col-span-7 lg:col-start-6">
            {lookFor.items.map((it) => (
              <div key={it.name} className="grid gap-1 border-t border-border py-5 sm:grid-cols-7 sm:gap-6">
                <dt className="font-display text-xl font-medium sm:col-span-3">{it.name}</dt>
                <dd className="text-fg-muted sm:col-span-4">{it.text}</dd>
              </div>
            ))}
          </dl>
          <div className="lg:col-span-7 lg:col-start-6">
            <h3 className="font-mono text-xs tracking-wide text-fg-muted">You will also need</h3>
            <ul className="mt-3 flex flex-wrap gap-2">
              {lookFor.basics.map((b) => (
                <li key={b} className="rounded-full border border-border-strong px-3 py-1.5 text-sm">
                  {b}
                </li>
              ))}
            </ul>
          </div>
        </Container>
      </section>

      <section aria-labelledby="steps-title" className="border-t border-border bg-surface">
        <Container className="grid gap-12 py-16 md:py-24 lg:grid-cols-12 lg:gap-x-12">
          <div className="lg:col-span-7">
            <Eyebrow>How joining works</Eyebrow>
            <SectionTitle id="steps-title" className="mt-4 lg:text-display-3">
              {steps.title}
            </SectionTitle>
            <ol className="mt-10 grid gap-0">
              {steps.items.map((s, i) => (
                <li key={s.name} className="grid grid-cols-[3rem_1fr] gap-x-4 border-t border-border py-5">
                  <span className="font-mono text-2xl text-fg-muted tabular">0{i + 1}</span>
                  <span>
                    <span className="block font-display text-xl font-medium">{s.name}</span>
                    <span className="mt-1 block text-fg-muted">{s.text}</span>
                  </span>
                </li>
              ))}
            </ol>
          </div>
          <IllustrativePhoto
            src={phone}
            alt={photos.phone.alt}
            sizes="(min-width: 1024px) 30vw, 100vw"
            className="max-w-md lg:col-span-4 lg:col-start-9 lg:mt-24"
            bracket
          />
        </Container>
      </section>

      <section data-theme="dark" aria-labelledby="pay-title" className="bg-bg">
        <Container className="grid gap-12 py-16 md:py-24 lg:grid-cols-12">
          <div className="lg:col-span-4">
            <SectionTitle id="pay-title" className="lg:text-display-3">
              {pay.title}
            </SectionTitle>
          </div>
          <dl className="grid gap-x-10 sm:grid-cols-2 lg:col-span-8">
            {pay.items.map((p) => (
              <div key={p.name} className="border-t border-border py-5">
                <dt className="font-display text-xl font-medium">{p.name}</dt>
                <dd className="mt-2 text-fg-muted">{p.text}</dd>
              </div>
            ))}
          </dl>
          <div className="border-l-2 border-lime pl-5 lg:col-span-8 lg:col-start-5">
            <h3 className="font-display text-xl font-medium">{measured.title}</h3>
            <p className="mt-2 text-fg-muted">{measured.text}</p>
            <TextLink href={measured.link.href} className="mt-3 inline-block">
              {measured.link.label} →
            </TextLink>
          </div>
        </Container>
      </section>

      <section aria-labelledby="terms-title" className="bg-bg">
        <Container className="grid gap-10 py-16 md:py-24 lg:grid-cols-12">
          <div className="lg:col-span-4">
            <SectionTitle id="terms-title" className="lg:text-display-3">
              {terms.title}
            </SectionTitle>
            <p className="mt-4 text-sm text-fg-muted">{terms.note}</p>
          </div>
          <div className="lg:col-span-7 lg:col-start-6">
            <ul className="grid">
              {terms.items.map((t) => (
                <li key={t} className="border-t border-border py-4">
                  {t}
                </li>
              ))}
            </ul>
            <TextLink href={terms.link.href} className="mt-4 inline-block">
              {terms.link.label} →
            </TextLink>
          </div>
        </Container>
      </section>

      <CtaBand
        title="Ready to qualify?"
        text="Apply online. Your qualification test opens once you have confirmed your email and finished your profile."
        primary={join}
      />
    </>
  );
}
