import { LegalDocument, legalMetadata } from "@/components/legal/legal-document";

export const metadata = legalMetadata("privacy");

export default function Page() {
  return <LegalDocument slug="privacy" />;
}
