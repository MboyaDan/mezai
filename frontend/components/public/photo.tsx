import Image, { type StaticImageData } from "next/image";
import type { ReactNode } from "react";
import { Bracket } from "@/components/brand/bracket";
import { cn } from "@/lib/cn";

/**
 * A stock photo, labelled illustrative (DESIGN.md section 8; source and
 * licence in docs/content/image-credits.md). [data-photo] lets the page
 * motion wipe it in; without JavaScript it simply shows.
 */
export function IllustrativePhoto({
  src,
  alt,
  sizes,
  caption,
  bracket = false,
  className,
  imageClassName,
}: {
  src: StaticImageData;
  alt: string;
  sizes: string;
  caption?: ReactNode;
  bracket?: boolean;
  className?: string;
  imageClassName?: string;
}) {
  const image = (
    <div data-photo className="overflow-hidden rounded-[2px] bg-surface">
      <Image
        src={src}
        alt={alt}
        sizes={sizes}
        placeholder="blur"
        className={cn("h-auto w-full", imageClassName)}
      />
    </div>
  );
  return (
    <figure className={className}>
      {bracket ? <Bracket>{image}</Bracket> : image}
      <figcaption className="mt-3 grid gap-1">
        {caption}
        <span className="font-mono text-[0.6875rem] text-fg-muted">Illustrative photo</span>
      </figcaption>
    </figure>
  );
}
