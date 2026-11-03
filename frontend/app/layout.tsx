import type { Metadata } from "next";
import "@fontsource-variable/inter";
import "@fontsource-variable/space-grotesk";
import "@fontsource-variable/jetbrains-mono";
import "@fontsource/instrument-serif/400-italic.css";
import { ToastProvider } from "@/components/ui/toast";
import { brand } from "@/lib/brand";
import { indexable, siteUrl } from "@/lib/site";
import { MotionProvider } from "@/motion/provider";
import "./globals.css";

export const metadata: Metadata = {
  metadataBase: new URL(siteUrl),
  title: { default: brand.name, template: `%s · ${brand.name}` },
  description: `${brand.name}, ${brand.descriptor}.`,
  applicationName: brand.name,
  openGraph: { type: "website", siteName: brand.name, locale: "en_KE" },
  twitter: { card: "summary_large_image" },
  robots: indexable ? undefined : { index: false, follow: false },
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="en" className="h-full antialiased">
      <body className="min-h-full">
        <MotionProvider>
          <ToastProvider>{children}</ToastProvider>
        </MotionProvider>
      </body>
    </html>
  );
}
