"use client";

import { useState, type ReactNode } from "react";
import { Plus, Search, SlidersHorizontal } from "lucide-react";
import { Logo } from "@/components/brand/logo";
import { SignalLattice } from "@/components/brand/signal-lattice";
import { Button, ButtonLink, IconButton } from "@/components/ui/button";
import { Checkbox, ChoiceGroup, Radio, Switch } from "@/components/ui/choice";
import { Dialog, Drawer } from "@/components/ui/dialog";
import {
  Accordion,
  Alert,
  Badge,
  Breadcrumb,
  Card,
  CardHeader,
  EmptyState,
  KeyboardHint,
  Skeleton,
  StatusPill,
  Tag,
} from "@/components/ui/display";
import { Field } from "@/components/ui/field";
import { Input, Select, Textarea } from "@/components/ui/input";
import { Tabs } from "@/components/ui/tabs";
import { TextLink } from "@/components/ui/text-link";
import { useToast } from "@/components/ui/toast";
import { Tooltip } from "@/components/ui/tooltip";

function Section({ id, title, children }: { id: string; title: string; children: ReactNode }) {
  return (
    <section aria-labelledby={id} className="grid gap-6 border-t border-border py-12 [&>*]:min-w-0">
      <h2
        id={id}
        className="font-display text-display-3/tight font-medium tracking-[-0.02em] max-md:text-[1.75rem]"
      >
        {title}
      </h2>
      {children}
    </section>
  );
}

/** Renders the same specimen on a light and a dark surface. */
function Both({ children }: { children: (theme: "light" | "dark") => ReactNode }) {
  return (
    <div className="grid gap-4 xl:grid-cols-2 [&>*]:min-w-0">
      {(["light", "dark"] as const).map((t) => (
        <div
          key={t}
          data-theme={t}
          className="grid gap-6 rounded-card border border-border bg-bg p-5 sm:p-6 [&>*]:min-w-0"
        >
          <p className="font-mono text-xs tracking-wide text-fg-muted uppercase">{t}</p>
          {children(t)}
        </div>
      ))}
    </div>
  );
}

const swatches: Array<[string, string, string]> = [
  ["charcoal-950", "#0E100E", "bg-charcoal-950"],
  ["charcoal-900", "#171A16", "bg-charcoal-900"],
  ["charcoal-800", "#22261F", "bg-charcoal-800"],
  ["ivory", "#F4F0E6", "bg-ivory"],
  ["paper", "#FBF9F4", "bg-paper"],
  ["ink", "#151714", "bg-ink"],
  ["muted", "#5A5F55", "bg-muted"],
  ["muted-dark", "#A3A99C", "bg-muted-dark"],
  ["line", "#DDD7C9", "bg-line"],
  ["lime", "#C8F031", "bg-lime"],
  ["lime-ink", "#4A5A00", "bg-lime-ink"],
  ["laterite", "#C2522D", "bg-laterite"],
  ["laterite-ink", "#9E3F1F", "bg-laterite-ink"],
  ["ochre", "#D9A441", "bg-ochre"],
  ["teal", "#2F8F83", "bg-teal"],
  ["success", "#2D7A4D", "bg-success"],
  ["warning", "#976000", "bg-warning"],
  ["danger", "#B3261E", "bg-danger"],
];

export function Showcase() {
  const toast = useToast();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [notify, setNotify] = useState(true);

  return (
    <div className="mx-auto max-w-[82rem] px-4 pb-24 sm:px-8 lg:px-16">
      <header className="grid gap-3 py-12">
        <p className="font-mono text-xs tracking-wide text-fg-muted uppercase">
          Development only · not indexed
        </p>
        <h1 className="font-display text-[2.75rem] leading-[1.04] font-medium tracking-[-0.02em] md:text-display-1">
          Design <span className="font-serif text-serif-accent">system</span>
        </h1>
        <p className="max-w-2xl text-lg text-fg-muted">
          Tokens and components from docs/design/DESIGN.md. Every specimen renders on a light and a dark
          surface.
        </p>
        <nav aria-label="Sections" className="mt-2 flex flex-wrap gap-x-5 gap-y-2 text-sm">
          {["Colour", "Type", "Brand", "Actions", "Forms", "Display", "Disclosure", "Overlays", "Shells"].map(
            (s) => (
              <a
                key={s}
                href={`#${s.toLowerCase()}`}
                className="text-fg-muted underline-offset-4 hover:text-fg hover:underline"
              >
                {s}
              </a>
            ),
          )}
        </nav>
      </header>

      <Section id="colour" title="Colour">
        <ul className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
          {swatches.map(([name, hex, cls]) => (
            <li key={name} className="overflow-hidden rounded-card border border-border bg-surface">
              <div className={`h-16 ${cls}`} />
              <div className="px-3 py-2 text-sm">
                <p className="font-medium">{name}</p>
                <p className="font-mono text-xs text-fg-muted">{hex}</p>
              </div>
            </li>
          ))}
        </ul>
        <Alert tone="warning" title="Lime is never text, icon or thin line on light surfaces (1.2:1).">
          Use lime-ink for accent text on light. The primary button keeps lime with charcoal text and a
          charcoal border.
        </Alert>
      </Section>

      <Section id="type" title="Type">
        <Both>
          {() => (
            <div className="grid gap-5">
              <p className="font-display text-[2.75rem] leading-[1.04] font-medium tracking-[-0.02em] md:text-display-1">
                Human judgment, <span className="font-serif text-serif-accent">verified.</span>
              </p>
              <p className="font-display text-[2.25rem] leading-[1.05] font-medium tracking-[-0.02em] md:text-display-2">
                Display 56 / 36
              </p>
              <p className="font-display text-[1.75rem] font-medium md:text-display-3">Display 40 / 28</p>
              <p className="font-display text-[1.375rem] font-semibold md:text-[1.75rem]">Heading 28 / 22</p>
              <p className="max-w-[68ch] text-base">
                Body is Inter at 16 px on public pages. Gold items have a known correct answer and are mixed
                into a contributor&apos;s queue at the project&apos;s gold rate.
              </p>
              <p className="text-sm text-fg-muted">Secondary 14 px in muted.</p>
              <p className="font-mono text-sm tabular">ITEM I-0412 · κ 0.64 · KES 1,240.00</p>
            </div>
          )}
        </Both>
      </Section>

      <Section id="brand" title="Brand">
        <Both>
          {() => (
            <div className="grid gap-6">
              <div className="flex flex-wrap items-center gap-8 text-fg">
                <Logo className="h-8" />
                <Logo variant="symbol" className="h-8" />
              </div>
              <div className="grid grid-cols-3 gap-4">
                {(["scattered", "align-on-load", "aligned"] as const).map((s) => (
                  <figure key={s} className="grid gap-2">
                    <SignalLattice rows={5} cols={5} accent={[3, 2]} state={s} />
                    <figcaption className="font-mono text-[0.6875rem] text-fg-muted">{s}</figcaption>
                  </figure>
                ))}
              </div>
            </div>
          )}
        </Both>
      </Section>

      <Section id="actions" title="Actions">
        <Both>
          {() => (
            <div className="grid gap-5">
              <div className="flex flex-wrap items-center gap-3">
                <Button arrow>Start a pilot project</Button>
                <Button variant="secondary">Secondary</Button>
                <Button variant="ghost">Ghost</Button>
                <Button variant="inverse">Inverse</Button>
                <Button variant="danger">Reject</Button>
              </div>
              <div className="flex flex-wrap items-center gap-3">
                <Button size="sm">Small</Button>
                <Button size="lg" arrow>
                  Large
                </Button>
                <Button loading>Saving</Button>
                <Button disabled>Disabled</Button>
                <IconButton label="Search" icon={<Search />} variant="secondary" />
                <IconButton label="Add item" icon={<Plus />} />
              </div>
              <p className="flex flex-wrap gap-x-6 gap-y-2">
                <TextLink href="/design#actions">Internal link</TextLink>
                <TextLink href="/design#actions" tone="accent">
                  Accent link
                </TextLink>
                <TextLink href="https://www.odpc.go.ke/">External link</TextLink>
                <ButtonLink href="/design#actions" variant="secondary" size="sm" arrow>
                  Button link
                </ButtonLink>
              </p>
            </div>
          )}
        </Both>
      </Section>

      <Section id="forms" title="Forms">
        <Both>
          {(t) => (
            <form className="grid gap-5" onSubmit={(e) => e.preventDefault()}>
              <Field label="Work email" hint="We reply within two working days." required>
                <Input type="email" autoComplete="email" placeholder="name@company.invalid" />
              </Field>
              <Field label="Organisation" error="Enter your organisation's name.">
                <Input defaultValue="" />
              </Field>
              <Field label="Project type" required>
                <Select defaultValue="">
                  <option value="" disabled>
                    Choose one
                  </option>
                  <option>AI evaluation and RLHF</option>
                  <option>African language data</option>
                  <option>Data annotation</option>
                  <option>Expert review</option>
                </Select>
              </Field>
              <Field label="Message">
                <Textarea placeholder="What do you need labelled or evaluated?" />
              </Field>
              <ChoiceGroup legend="Languages">
                <Checkbox label="Kiswahili" defaultChecked />
                <Checkbox label="English" hint="Working proficiency or better" />
              </ChoiceGroup>
              <ChoiceGroup legend="Overall preference">
                <Radio name={`pref-${t}`} value="a" label="Response A is better" defaultChecked />
                <Radio name={`pref-${t}`} value="tie" label="Tie" />
                <Radio name={`pref-${t}`} value="b" label="Response B is better" />
              </ChoiceGroup>
              <Switch
                checked={notify}
                onCheckedChange={setNotify}
                label="Email me when new projects match my skills"
              />
              <div>
                <Button type="submit">Send enquiry</Button>
              </div>
            </form>
          )}
        </Both>
      </Section>

      <Section id="display" title="Display">
        <Both>
          {() => (
            <div className="grid gap-5">
              <Breadcrumb
                items={[
                  { label: "Projects", href: "/design" },
                  { label: "Pairwise: farming advice (Sample)" },
                ]}
              />
              <div className="flex flex-wrap items-center gap-2">
                <StatusPill status="success">Active</StatusPill>
                <StatusPill status="warning">Paused</StatusPill>
                <StatusPill status="danger">Rejected</StatusPill>
                <StatusPill status="info">In review</StatusPill>
                <StatusPill status="neutral">Draft</StatusPill>
                <Tag>Kiswahili · native</Tag>
                <Badge>12</Badge>
              </div>
              <Card>
                <CardHeader title="Trust score" meta="Last 30 days" />
                <div className="flex items-baseline gap-3 px-5 pt-3 pb-5">
                  <span className="font-mono text-5xl tabular">0.87</span>
                  <span className="text-sm text-fg-muted">Sample · above the 0.70 floor</span>
                </div>
              </Card>
              <Alert tone="success" title="Submission saved">
                Your answer is in. Quality checks run later, so results never appear here.
              </Alert>
              <Alert
                tone="danger"
                title="Upload failed"
                action={
                  <Button size="sm" variant="secondary">
                    Try again
                  </Button>
                }
              >
                Row 14: the label “Billng” is not in this project&apos;s label set.
              </Alert>
              <div className="grid gap-2" aria-hidden="true">
                <Skeleton className="h-5 w-2/3" />
                <Skeleton className="h-4 w-full" />
                <Skeleton className="h-4 w-5/6" />
              </div>
              <EmptyState
                title="No projects yet"
                action={<Button size="sm">Take a qualification test</Button>}
              >
                Projects appear here once you pass a qualification test for one of your skills.
              </EmptyState>
              <p className="flex flex-wrap gap-4">
                <KeyboardHint keys={["1"]} label="first label" />
                <KeyboardHint keys={["⌘", "Enter"]} label="submit" />
                <KeyboardHint keys={["?"]} label="shortcuts" />
              </p>
            </div>
          )}
        </Both>
      </Section>

      <Section id="disclosure" title="Disclosure">
        <Both>
          {() => (
            <div className="grid gap-8">
              <Tabs
                label="Project views"
                tabs={[
                  {
                    id: "contributor",
                    label: "Contributor view",
                    content: <p className="text-fg-muted">What contributors see.</p>,
                  },
                  {
                    id: "client",
                    label: "Client view",
                    content: <p className="text-fg-muted">What the client sees.</p>,
                  },
                  {
                    id: "ops",
                    label: "Ops view",
                    content: <p className="text-fg-muted">Queues and contributor health.</p>,
                  },
                ]}
              />
              <Accordion
                items={[
                  {
                    id: "gold",
                    title: "What is a gold item?",
                    content: "An item with a known correct answer, mixed in to measure accuracy.",
                    open: true,
                  },
                  {
                    id: "overlap",
                    title: "What is overlap?",
                    content: "The number of different contributors who answer the same item.",
                  },
                ]}
              />
            </div>
          )}
        </Both>
      </Section>

      <Section id="overlays" title="Overlays">
        <div className="flex flex-wrap items-center gap-3">
          <Button variant="secondary" onClick={() => setDialogOpen(true)}>
            Open dialog
          </Button>
          <Button
            variant="secondary"
            icon={<SlidersHorizontal className="size-4" aria-hidden="true" />}
            onClick={() => setDrawerOpen(true)}
          >
            Open drawer
          </Button>
          <Button
            variant="secondary"
            onClick={() => toast({ tone: "success", title: "Draft saved", body: "Saved 2 seconds ago." })}
          >
            Show toast
          </Button>
          <Button
            variant="secondary"
            onClick={() =>
              toast({
                tone: "danger",
                title: "Could not submit",
                body: "Check your connection and try again.",
              })
            }
          >
            Show error toast
          </Button>
          <Tooltip content="Agreement corrected for chance. 0.61 to 0.80 is substantial.">
            <Button variant="ghost">Hover or focus for tooltip</Button>
          </Tooltip>
        </div>
        <Dialog
          open={dialogOpen}
          onClose={() => setDialogOpen(false)}
          title="Reject this submission?"
          description="The contributor sees your reason. Rejected work earns nothing."
          footer={
            <>
              <Button variant="secondary" onClick={() => setDialogOpen(false)}>
                Cancel
              </Button>
              <Button variant="danger" onClick={() => setDialogOpen(false)}>
                Reject
              </Button>
            </>
          }
        >
          <Field label="Reason" required>
            <Textarea rows={3} />
          </Field>
        </Dialog>
        <Drawer
          open={drawerOpen}
          onClose={() => setDrawerOpen(false)}
          title="Instructions"
          description="Pairwise: farming advice (Sample)"
        >
          <div className="grid gap-3 text-fg-muted">
            <p>Read the prompt, then both responses in full.</p>
            <p>Score each response on the rubric before choosing a preference.</p>
            <p>Write at least 80 characters explaining your choice.</p>
          </div>
        </Drawer>
      </Section>

      <Section id="shells" title="Shells">
        <ul className="grid gap-3 sm:grid-cols-3">
          {[
            ["public", "Public site", "Charcoal header and footer, editorial bands"],
            ["contributor", "Contributor portal", "Charcoal top bar, ivory canvas, bottom tabs on mobile"],
            ["workspace", "Workspace", "Charcoal sidebar, paper content, drawer under 1024 px"],
          ].map(([slug, name, desc]) => (
            <li key={slug}>
              <Card interactive className="h-full">
                <a href={`/design/shells/${slug}`} className="grid gap-1 p-5">
                  <span className="font-display text-lg font-semibold">{name} →</span>
                  <span className="text-sm text-fg-muted">{desc}</span>
                </a>
              </Card>
            </li>
          ))}
        </ul>
      </Section>
    </div>
  );
}
