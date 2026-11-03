import { PublicShell } from "@/components/shells/public-shell";

export default function PublicLayout({ children }: LayoutProps<"/">) {
  return <PublicShell>{children}</PublicShell>;
}
