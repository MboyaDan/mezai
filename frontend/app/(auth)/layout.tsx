import { AuthFrame } from "@/components/auth/auth-frame";
import { SessionProvider } from "@/lib/session";

export default function AuthLayout({ children }: { children: React.ReactNode }) {
  return (
    <SessionProvider>
      <AuthFrame>{children}</AuthFrame>
    </SessionProvider>
  );
}
