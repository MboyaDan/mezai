import { LegalDocument, legalMetadata } from "@/components/legal/legal-document";

export const metadata = legalMetadata("terms");

export default function Page() {
  return <LegalDocument slug="terms" />;
}
