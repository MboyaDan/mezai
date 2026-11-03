import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { ImageResponse } from "next/og";
import { brand } from "@/lib/brand";

export const alt = `${brand.name}: Human judgment, verified. Data for AI, built in Africa.`;
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";

const font = (pkg: string, file: string) => readFile(join(process.cwd(), "node_modules", pkg, "files", file));

// Signal Lattice: aligned strokes with one lime dot (DESIGN.md section 8).
function Lattice() {
  const rows = 7;
  const cols = 5;
  const cells = [];
  for (let r = 0; r < rows; r++) {
    for (let c = 0; c < cols; c++) {
      const accent = r === 3 && c === 2;
      cells.push(
        <div
          key={`${r}-${c}`}
          style={{ width: 44, height: 44, display: "flex", alignItems: "center", justifyContent: "center" }}
        >
          {accent ? (
            <div style={{ width: 16, height: 16, borderRadius: 8, background: "#C8F031" }} />
          ) : (
            <div style={{ width: 2, height: 22, background: "#3B4237", transform: "rotate(30deg)" }} />
          )}
        </div>,
      );
    }
  }
  return <div style={{ display: "flex", flexWrap: "wrap", width: cols * 44 }}>{cells}</div>;
}

export default async function Image() {
  const [grotesk, serif] = await Promise.all([
    font("@fontsource/space-grotesk", "space-grotesk-latin-500-normal.woff"),
    font("@fontsource/instrument-serif", "instrument-serif-latin-400-italic.woff"),
  ]);
  return new ImageResponse(
    <div
      style={{
        width: "100%",
        height: "100%",
        background: "#0E100E",
        color: "#F4F0E6",
        display: "flex",
        justifyContent: "space-between",
        alignItems: "center",
        padding: "72px 80px",
        fontFamily: "Space Grotesk",
      }}
    >
      <div style={{ display: "flex", flexDirection: "column", maxWidth: 760 }}>
        <div style={{ fontSize: 28, color: "#A3A99C", letterSpacing: -0.5 }}>{brand.name}</div>
        <div
          style={{
            display: "flex",
            flexDirection: "column",
            marginTop: 40,
            fontSize: 64,
            lineHeight: 1.04,
            letterSpacing: -1.5,
          }}
        >
          <div style={{ display: "flex" }}>
            Human judgment,&nbsp;
            <span style={{ fontFamily: "Instrument Serif", fontStyle: "italic" }}>verified.</span>
          </div>
          <div style={{ color: "#A3A99C" }}>Data for AI, built in Africa.</div>
        </div>
        <div style={{ marginTop: 40, fontSize: 24, color: "#A3A99C" }}>{brand.descriptor}</div>
      </div>
      <Lattice />
    </div>,
    {
      ...size,
      fonts: [
        { name: "Space Grotesk", data: grotesk, weight: 500, style: "normal" },
        { name: "Instrument Serif", data: serif, weight: 400, style: "italic" },
      ],
    },
  );
}
