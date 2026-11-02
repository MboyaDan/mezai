import Image from "next/image";
import { ApiStatusBadge } from "@/components/api-status-badge";
import { brand } from "@/lib/brand";

// Stage 1 placeholder. The storyboard home page replaces this in Stage 5.
export default function Home() {
  return (
    <main className="flex min-h-dvh flex-col justify-between px-4 py-10 sm:px-16 sm:py-16">
      <Image src="/brand/mezai-logo-reversed.svg" alt={brand.name} width={160} height={65} priority />
      <div className="max-w-2xl">
        <h1 className="text-4xl font-medium tracking-tight sm:text-6xl">{brand.name}</h1>
        <p className="mt-4 text-lg text-[#A3A99C]">
          {brand.name} is {brand.descriptor}, {brand.city}. The pilot platform is under construction.
        </p>
      </div>
      <ApiStatusBadge />
    </main>
  );
}
