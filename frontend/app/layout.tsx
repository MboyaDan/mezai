import type { Metadata } from "next";
import "@fontsource-variable/inter";
import "@fontsource-variable/space-grotesk";
import "@fontsource-variable/jetbrains-mono";
import "@fontsource/instrument-serif/400-italic.css";
import { ToastProvider } from "@/components/ui/toast";
import { brand } from "@/lib/brand";
import "./globals.css";

export const metadata: Metadata = {
  title: { default: brand.name, template: `%s · ${brand.name}` },
  description: `${brand.name}, ${brand.descriptor}.`,
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="en" className="h-full antialiased">
      <body className="min-h-full">
        <ToastProvider>{children}</ToastProvider>
      </body>
    </html>
  );
}
