import type { Metadata } from "next";
import { Container } from "@/components/public/layout";
import { PageHeader, titleText } from "@/components/public/page-header";
import { EnquiryForm } from "@/components/contact/enquiry-form";
import { TextLink } from "@/components/ui/text-link";
import { contactPage } from "@/content/contact";
import { brand } from "@/lib/brand";

export const metadata: Metadata = {
  title: "Start a pilot",
  description: contactPage.intro,
  alternates: { canonical: "/contact" },
  openGraph: { title: titleText(contactPage.title) },
};

export default function ContactPage() {
  const { next, contributors } = contactPage;
  return (
    <>
      <PageHeader eyebrow={contactPage.eyebrow} title={contactPage.title} intro={contactPage.intro} />

      {/* Split layout (DESIGN.md section 10): the form, then what happens after it. */}
      <section aria-label="Project enquiry" className="bg-bg">
        <Container className="grid gap-14 py-16 md:py-24 lg:grid-cols-12 lg:gap-x-12">
          <div className="relative lg:col-span-7">
            <EnquiryForm />
          </div>

          <aside
            aria-labelledby="next-title"
            className="grid content-start gap-10 lg:col-span-4 lg:col-start-9"
          >
            <div>
              <h2 id="next-title" className="font-display text-xl font-medium">
                {next.title}
              </h2>
              <ol className="mt-6 grid border-t border-border-strong">
                {next.steps.map((s, i) => (
                  <li key={s.name} className="grid grid-cols-[2rem_1fr] gap-x-3 border-b border-border py-5">
                    <span className="font-mono text-xs text-fg-muted tabular-nums">0{i + 1}</span>
                    <div>
                      <p className="font-medium">{s.name}</p>
                      <p className="mt-1 text-fg-muted">{s.text}</p>
                    </div>
                  </li>
                ))}
              </ol>
            </div>

            {/* Contact details come from docs/content/ (Q2); hidden while empty. */}
            {brand.contactEmail && (
              <div className="border-t border-border pt-5">
                <h2 className="font-display text-xl font-medium">Email</h2>
                <TextLink href={`mailto:${brand.contactEmail}`} className="mt-2 inline-block">
                  {brand.contactEmail}
                </TextLink>
              </div>
            )}

            <div className="rounded-card border border-border bg-surface p-5">
              <p className="text-fg-muted">{contributors.text}</p>
              <TextLink href={contributors.link.href} className="mt-2 inline-block">
                {contributors.link.label}
              </TextLink>
            </div>
          </aside>
        </Container>
      </section>
    </>
  );
}
