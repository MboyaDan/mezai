import type { Metadata } from "next";
import { PublicShell } from "@/components/shells/public-shell";
import { ErrorScreen } from "@/components/public/error-screen";
import { ButtonLink } from "@/components/ui/button";

export const metadata: Metadata = { title: "Page not found", robots: { index: false } };

// Unmatched URLs render here inside the root layout, so it brings its own shell.
export default function NotFound() {
  return (
    <PublicShell>
      <ErrorScreen
        code="404"
        title="Nothing lines up here."
        text="The page you asked for does not exist, or it has moved. Check the address, or start again from one of these."
      >
        <ButtonLink href="/" arrow>
          Go to the home page
        </ButtonLink>
        <ButtonLink href="/services" variant="secondary">
          Services
        </ButtonLink>
        <ButtonLink href="/contact" variant="secondary">
          Start a pilot
        </ButtonLink>
      </ErrorScreen>
    </PublicShell>
  );
}
