import { LegalDocument, legalMetadata } from "@/components/legal/legal-document";

export const metadata = legalMetadata("contributor-terms");

export default function Page() {
  return <LegalDocument slug="contributor-terms" />;
}
