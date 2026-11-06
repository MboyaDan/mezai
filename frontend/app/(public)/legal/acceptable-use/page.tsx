import { LegalDocument, legalMetadata } from "@/components/legal/legal-document";

export const metadata = legalMetadata("acceptable-use");

export default function Page() {
  return <LegalDocument slug="acceptable-use" />;
}
