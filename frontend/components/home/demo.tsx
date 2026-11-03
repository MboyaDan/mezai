import { demo } from "@/content/home";
import { Container, Eyebrow, SectionTitle } from "./layout";
import { DemoWindow } from "./demo-window";

/** Frame 5. */
export function Demo() {
  return (
    <section data-theme="dark" aria-labelledby="demo-title" className="bg-bg">
      <Container className="py-20 md:py-32">
        <div className="grid gap-4 md:grid-cols-12">
          <Eyebrow className="md:col-span-3 md:pt-3">{demo.eyebrow}</Eyebrow>
          <SectionTitle id="demo-title" className="md:col-span-8 md:col-start-4 lg:text-display-3">
            {demo.title}
          </SectionTitle>
        </div>
        <div className="mt-12 md:mt-16">
          <DemoWindow />
        </div>
        <p className="mt-6 text-sm text-fg-muted">{demo.note}</p>
      </Container>
    </section>
  );
}
