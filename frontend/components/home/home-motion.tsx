"use client";

import { useEffect } from "react";

// Scroll motion for the home page (DESIGN.md section 9). GSAP and its plugins
// load after the browser is idle, only on this page, and never with reduced
// motion. The server-rendered page is complete without it: GSAP only animates
// what is still below the viewport, so nothing visible ever flickers.
// Eases approximate the motion tokens: expo.out ≈ ease-out, power3.inOut ≈ ease-in-out.

type Cleanup = () => void;

export function HomeMotion() {
  useEffect(() => {
    if (matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    let cleanup: Cleanup | undefined;
    let cancelled = false;

    const start = async () => {
      const [{ gsap }, { ScrollTrigger }, { DrawSVGPlugin }, { MotionPathPlugin }] = await Promise.all([
        import("gsap"),
        import("gsap/ScrollTrigger"),
        import("gsap/DrawSVGPlugin"),
        import("gsap/MotionPathPlugin"),
      ]);
      if (cancelled) return;
      gsap.registerPlugin(ScrollTrigger, DrawSVGPlugin, MotionPathPlugin);

      const below = (el: Element) => el.getBoundingClientRect().top > window.innerHeight;
      const mm = gsap.matchMedia();

      // Pattern 1: fade and rise, groups 60 ms apart (at most 8 staggered).
      const reveal = () => {
        gsap.utils
          .toArray<HTMLElement>("[data-reveal]")
          .filter(below)
          .forEach((el) => {
            gsap.from(el, {
              opacity: 0,
              y: 24,
              duration: 0.6,
              ease: "expo.out",
              scrollTrigger: { trigger: el, start: "top 85%", once: true },
            });
          });
        gsap.utils
          .toArray<HTMLElement>("[data-reveal-group]")
          .filter(below)
          .forEach((group) => {
            const items = Array.from(group.children);
            gsap.from(items, {
              opacity: 0,
              y: 16,
              duration: 0.6,
              ease: "expo.out",
              stagger: { each: 0.06, amount: Math.min(items.length, 8) * 0.06 },
              scrollTrigger: { trigger: group, start: "top 85%", once: true },
            });
          });
      };

      // Pattern 4: arcs draw into Nairobi, pulses travel, then the hub fills.
      const network = (map: "wide" | "narrow", scrub: boolean) => {
        const svg = document.querySelector<SVGSVGElement>(`[data-map="${map}"]`);
        if (!svg) return;
        const inbound = svg.querySelectorAll('[data-arc="africa"]');
        const outbound = Array.from(svg.querySelectorAll<SVGPathElement>('[data-arc="buyer"]'));
        const hub = svg.querySelector("[data-hub]");
        const tl = gsap.timeline({
          scrollTrigger: scrub
            ? { trigger: svg, start: "top 75%", end: "bottom 45%", scrub: 0.6 }
            : { trigger: svg, start: "top 80%", once: true },
          defaults: { ease: "power3.inOut" },
        });
        tl.from(inbound, { drawSVG: "100% 100%", duration: 1, stagger: 0.12 })
          .from(hub, { opacity: 0, scale: 0.3, transformOrigin: "50% 50%", duration: 0.4, ease: "expo.out" })
          .from(outbound, { drawSVG: 0, duration: 1, stagger: 0.1 }, "<0.1");
        if (scrub) {
          outbound.forEach((path) => {
            const pulse = svg.querySelector(`[data-pulse="${path.getAttribute("data-city") ?? ""}"]`);
            if (!pulse) return;
            tl.to(
              pulse,
              { motionPath: { path, align: path, alignOrigin: [0.5, 0.5] }, duration: 0.8, ease: "none" },
              "<0.15",
            ).to(pulse, { keyframes: { opacity: [0, 1, 1, 0] }, duration: 0.8, ease: "none" }, "<");
          });
        }
      };

      // Pattern 4 again, for the quality steps: connectors draw as steps enter.
      const steps = (axis: "x" | "y") => {
        const list = document.querySelector("[data-steps]");
        if (!list || !below(list)) return;
        gsap.from(list.querySelectorAll("[data-connector]"), {
          [axis === "x" ? "scaleX" : "scaleY"]: 0,
          transformOrigin: axis === "x" ? "left center" : "center top",
          duration: 0.5,
          ease: "power3.inOut",
          stagger: 0.15,
          scrollTrigger: { trigger: list, start: "top 75%", once: true },
        });
      };

      // Pattern 7: a 1 px rule draws across before the next content enters.
      const rules = () => {
        gsap.utils
          .toArray<HTMLElement>("[data-rule]")
          .filter(below)
          .forEach((rule) => {
            gsap.from(rule, {
              scaleX: 0,
              duration: 0.8,
              ease: "power3.inOut",
              scrollTrigger: { trigger: rule, start: "top 85%", once: true },
            });
          });
      };

      mm.add("(min-width: 1024px)", () => {
        network("wide", true);
        steps("x");
      });
      mm.add("(min-width: 768px) and (max-width: 1023.98px)", () => {
        network("wide", false);
        steps("y");
      });
      mm.add("(max-width: 767.98px)", () => {
        network("narrow", false);
        steps("y");
      });
      // The context records every tween and ScrollTrigger made here, so
      // leaving the page reverts exactly those (DESIGN.md: kill on route change).
      const ctx = gsap.context(() => {
        reveal();
        rules();
      });

      cleanup = () => {
        mm.revert();
        ctx.revert();
      };
    };

    const idle = window.requestIdleCallback
      ? window.requestIdleCallback(() => void start(), { timeout: 2500 })
      : window.setTimeout(() => void start(), 1200);
    return () => {
      cancelled = true;
      if (window.cancelIdleCallback) window.cancelIdleCallback(idle);
      else window.clearTimeout(idle);
      cleanup?.();
    };
  }, []);

  return null;
}
