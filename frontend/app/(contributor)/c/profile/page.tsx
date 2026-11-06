import type { Metadata } from "next";
import { ProfileForm } from "@/components/contributor/profile-form";

export const metadata: Metadata = { title: "Profile", robots: { index: false } };

export default function Page() {
  return <ProfileForm />;
}
