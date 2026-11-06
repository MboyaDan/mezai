"use client";

import "@fontsource-variable/inter";
import "@fontsource-variable/space-grotesk";
import "@fontsource-variable/jetbrains-mono";
import "./globals.css";
import { ErrorScreen } from "@/components/public/error-screen";
import { Button } from "@/components/ui/button";
import { brand } from "@/lib/brand";

// The root layout itself failed, so this page renders its own document.
export default function GlobalError({ error, retry }: { error: unknown; retry: () => void }) {
  const digest = error instanceof Error && "digest" in error ? String(error.digest) : undefined;
  return (
    <html lang="en" className="h-full antialiased">
      <body className="min-h-full bg-bg text-fg">
        <title>{`Something went wrong · ${brand.name}`}</title>
        <main id="main">
          <ErrorScreen
            code="500"
            title="Something went wrong."
            text={`${brand.name} could not load this page. It is a fault on our side. Try again in a few minutes.`}
            reference={digest}
          >
            <Button onClick={() => retry()} arrow>
              Try again
            </Button>
            {/* A full page load: the app's router may be what failed. */}
            {/* eslint-disable-next-line @next/next/no-html-link-for-pages */}
            <a href="/" className="inline-flex min-h-11 items-center px-2 underline underline-offset-4">
              Go to the home page
            </a>
          </ErrorScreen>
        </main>
      </body>
    </html>
  );
}
