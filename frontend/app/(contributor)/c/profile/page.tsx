import type { Metadata } from "next";
import { EmailChoices } from "@/components/contributor/email-choices";
import { ProfileForm } from "@/components/contributor/profile-form";

export const metadata: Metadata = { title: "Profile", robots: { index: false } };

export default function Page() {
  return (
    <div className="grid gap-8">
      <ProfileForm />
      <EmailChoices />
    </div>
  );
}
