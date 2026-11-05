import { network } from "@/content/home";
import { Container, Eyebrow, SectionTitle } from "@/components/public/layout";
import { StatBand } from "./stat-band";

// Illustrative cities (labelled as such). Positions use an equirectangular
// projection of real coordinates; no continent outlines are drawn. Cities
// close together on the map are left out so labels never collide.
type City = {
  name: string;
  lon: number;
  lat: number;
  kind: "hub" | "africa" | "buyer";
  anchor?: "start" | "end";
  dy?: number; // label below (+) or above (-) the node instead of beside it
};
const cities: City[] = [
  { name: "Nairobi", lon: 36.8, lat: -1.3, kind: "hub" },
  { name: "Lagos", lon: 3.4, lat: 6.5, kind: "africa", anchor: "end" },
  { name: "Kigali", lon: 30.1, lat: -1.9, kind: "africa", anchor: "end", dy: 16 },
  { name: "Johannesburg", lon: 28.0, lat: -26.2, kind: "africa" },
  { name: "London", lon: -0.1, lat: 51.5, kind: "buyer", anchor: "end" },
  { name: "Berlin", lon: 13.4, lat: 52.5, kind: "buyer" },
  { name: "New York", lon: -74.0, lat: 40.7, kind: "buyer", dy: 20 },
  { name: "San Francisco", lon: -122.4, lat: 37.8, kind: "buyer", dy: 20 },
  { name: "Bengaluru", lon: 77.6, lat: 13.0, kind: "buyer", anchor: "end", dy: -14 },
];

type View = { lon0: number; lon1: number; lat0: number; lat1: number; w: number; h: number; text: number };
const wide: View = { lon0: -130, lon1: 90, lat0: 64, lat1: -36, w: 1100, h: 500, text: 15 };
const narrow: View = { lon0: -25, lon1: 60, lat0: 62, lat1: -36, w: 425, h: 490, text: 13 };

function project(v: View, lon: number, lat: number) {
  return { x: ((lon - v.lon0) / (v.lon1 - v.lon0)) * v.w, y: ((v.lat0 - lat) / (v.lat0 - v.lat1)) * v.h };
}

function arc(a: { x: number; y: number }, b: { x: number; y: number }) {
  const mx = (a.x + b.x) / 2;
  const my = (a.y + b.y) / 2 - Math.hypot(b.x - a.x, b.y - a.y) * 0.22;
  return `M${a.x.toFixed(1)},${a.y.toFixed(1)} Q${mx.toFixed(1)},${my.toFixed(1)} ${b.x.toFixed(1)},${b.y.toFixed(1)}`;
}

function Map({ view, className, ...rest }: { view: View; className?: string; "data-map": string }) {
  const hub = cities.find((c) => c.kind === "hub")!;
  const h = project(view, hub.lon, hub.lat);
  const step = 18;
  const dots: Array<[number, number]> = [];
  for (let y = step / 2; y < view.h; y += step)
    for (let x = step / 2; x < view.w; x += step) dots.push([x, y]);
  const visible = cities.filter((c) => c.lon >= view.lon0 && c.lon <= view.lon1);
  return (
    <svg
      {...rest}
      viewBox={`0 0 ${view.w} ${view.h}`}
      className={className}
      role="img"
      aria-labelledby={`network-map-title-${rest["data-map"]}`}
    >
      <title id={`network-map-title-${rest["data-map"]}`}>
        Illustrative map: African cities connect through Nairobi to cities where AI models are built.
      </title>
      <g className="fill-[var(--lattice-stroke)]">
        {dots.map(([x, y]) => (
          <circle key={`${x}-${y}`} cx={x} cy={y} r={1.1} />
        ))}
      </g>
      {/* Arcs and pulses are drawn by GSAP on scroll (home-motion.tsx); the static
          render shows them complete. Arcs from African cities run into Nairobi,
          arcs to buyer cities run out of it. */}
      <g fill="none" strokeWidth={1.5}>
        {visible
          .filter((c) => c.kind !== "hub")
          .map((c) => {
            const p = project(view, c.lon, c.lat);
            return (
              <path
                key={c.name}
                d={c.kind === "africa" ? arc(p, h) : arc(h, p)}
                data-arc={c.kind}
                data-city={c.name}
                className={
                  c.kind === "africa" ? "stroke-[var(--lattice-stroke-near)]" : "stroke-muted-dark/60"
                }
              />
            );
          })}
      </g>
      {/* Pulses that travel along the arcs; invisible unless animated. */}
      <g aria-hidden="true">
        {visible
          .filter((c) => c.kind !== "hub")
          .map((c) => (
            <circle
              key={c.name}
              r={3.5}
              cx={0}
              cy={0}
              opacity={0}
              className="fill-lime"
              data-pulse={c.name}
            />
          ))}
      </g>
      {visible.map((c) => {
        const p = project(view, c.lon, c.lat);
        const anchor = c.anchor ?? "start";
        return (
          <g key={c.name}>
            {c.kind === "hub" ? (
              <>
                <circle cx={p.x} cy={p.y} r={11} className="fill-none stroke-lime" strokeWidth={1} />
                <circle cx={p.x} cy={p.y} r={5} className="fill-lime" data-hub />
              </>
            ) : (
              <circle
                cx={p.x}
                cy={p.y}
                r={3}
                className={c.kind === "africa" ? "fill-ivory" : "fill-muted-dark"}
              />
            )}
            <text
              x={c.dy ? p.x : p.x + (anchor === "end" ? -10 : 10)}
              y={p.y + (c.dy ?? 0) + view.text * 0.35}
              textAnchor={anchor}
              className={c.kind === "hub" ? "fill-ivory font-mono" : "fill-muted-dark font-mono"}
              fontSize={c.kind === "hub" ? view.text + 2 : view.text}
            >
              {c.name}
            </text>
          </g>
        );
      })}
    </svg>
  );
}

/** Frame 2. Static, fully drawn; Stage 5b adds the scrubbed arcs and pulses. */
export function Network() {
  return (
    <section data-theme="dark" aria-labelledby="network-title" className="bg-bg">
      <Container className="grid gap-10 py-20 md:py-28 lg:grid-cols-12 lg:gap-12">
        <div className="lg:col-span-4" data-reveal>
          <Eyebrow>{network.eyebrow}</Eyebrow>
          <SectionTitle id="network-title" className="mt-4 text-[2rem] md:text-[2.5rem] lg:text-[2.5rem]">
            {network.title}
          </SectionTitle>
          <ol className="mt-8 grid gap-5 border-l border-border pl-5">
            {network.lines.map((line, i) => (
              <li key={i} className="text-fg-muted">
                <span className="mr-2 font-mono text-xs text-fg">0{i + 1}</span>
                {line}
              </li>
            ))}
          </ol>
        </div>
        <figure className="lg:col-span-8">
          <Map view={wide} className="hidden h-auto w-full md:block" data-map="wide" />
          <Map view={narrow} className="h-auto w-full md:hidden" data-map="narrow" />
          <figcaption className="mt-3 font-mono text-[0.6875rem] text-fg-muted">{network.note}</figcaption>
        </figure>
      </Container>
      <StatBand />
    </section>
  );
}
