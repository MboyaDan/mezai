import type { Metadata } from "next";
import { Hero } from "@/components/home/hero";
import { Network } from "@/components/home/network";
import { Services } from "@/components/home/services";
import { People } from "@/components/home/people";
import { Demo } from "@/components/home/demo";
import { Quality } from "@/components/home/quality";
import { FinalCta, Practices } from "@/components/home/closing";
import { hero } from "@/content/home";
import { brand } from "@/lib/brand";

export const metadata: Metadata = {
  title: { absolute: `${brand.name}: Human judgment, verified. Data for AI, built in Africa.` },
  description: hero.subline,
  alternates: { canonical: "/" },
};

// The storyboard home page (DESIGN.md section 10), frames 1 to 8. The footer
// is in the public shell.
export default function Home() {
  return (
    <>
      <Hero />
      <Network />
      <Services />
      <People />
      <Demo />
      <Quality />
      <Practices />
      <FinalCta />
    </>
  );
}
