// Contributor sign-in with Google (Stage 29, D-171). The button is a plain
// navigation to the API, which redirects to Google and back; no Google
// script runs on our pages.

export type GoogleIntent = "signin" | "signup";

/** The API route the button opens. Sign-up carries the accepted versions. */
export function googleStartUrl(intent: GoogleIntent, consents: Record<string, string> = {}): string {
  const q = new URLSearchParams({ intent });
  if (intent === "signup") for (const [k, v] of Object.entries(consents)) q.set(k, v);
  return `/api/v1/auth/google/start?${q}`;
}

/** What the page says when the API sends the person back with ?error=. */
export const googleErrors: Record<string, string> = {
  google_off: "Sign-in with Google is not available at the moment. Use your email address and password.",
  google_failed: "Google sign-in did not work. Try again, or use your email address and password.",
  google_expired: "That Google sign-in took too long or was already used. Try again.",
  google_cancelled: "Google sign-in was cancelled. Nothing was changed.",
  google_no_account:
    "No MezAI account uses that Google account yet. Create an account first, or sign in with your email address and password.",
  google_use_password:
    "This account signs in with its email address and password. Google sign-in is for contributor accounts without two-step sign-in.",
  google_unverified:
    "Google has not confirmed the email address on that account. Use another Google account, or join with your email address.",
  consent_required: "Accept the privacy notice and contributor terms, then continue with Google.",
};

/** The message for an error code from the address bar, or null for none or an unknown one. */
export function googleError(code: string | null | undefined): string | null {
  return (code && Object.hasOwn(googleErrors, code) && googleErrors[code]) || null;
}
