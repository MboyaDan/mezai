"use client";

import { useEffect } from "react";
import { ErrorScreen } from "@/components/public/error-screen";
import { Button, ButtonLink } from "@/components/ui/button";

// A public page failed to render. The public layout (header, footer) still shows.
export default function PublicError({ error, retry }: { error: unknown; retry: () => void }) {
  // Server errors arrive with a digest that matches the server log; the message stays hidden.
  const digest = error instanceof Error && "digest" in error ? String(error.digest) : undefined;
  useEffect(() => {
    console.error(error);
  }, [error]);
  return (
    <ErrorScreen
      code="500"
      title="Something went wrong."
      text="This page could not be shown. It is a fault on our side, not something you did. Try again, or come back in a few minutes."
      reference={digest}
    >
      <Button onClick={() => retry()} arrow>
        Try again
      </Button>
      <ButtonLink href="/" variant="secondary">
        Go to the home page
      </ButtonLink>
    </ErrorScreen>
  );
}
