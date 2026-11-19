import { readFileSync } from "node:fs";
import { join } from "node:path";

// The sample delivery on /proof (Stage 21b, D-162). The files in
// public/proof/ are written by the backend's export code from fictional data
// (`make sample-package`); a Go test fails if they drift. The page reads them
// at build time, so every figure it shows is in the files a visitor downloads.

export const proofDir = "/proof";
const stem = "customer-message-intent-sample-v1";
export const proofFiles = {
  manifest: `${stem}-manifest.json`,
  jsonl: `${stem}.jsonl`,
  csv: `${stem}.csv`,
} as const;

export type ProofStat = {
  statistic: "percent" | "fleiss_kappa" | "cohen_kappa" | string;
  label: string | null;
  value: number;
  band: string | null;
  items: number;
  raters: number;
};

export type ProofManifest = {
  schema: string;
  schema_version: number;
  project: { id: string; name: string; task_type: string; organisation: string; overlap_k: number };
  version: number;
  exported_at: string;
  items: {
    total: number;
    delivered: number;
    consensus: number;
    adjudicated: number;
    rejected: number;
    not_final: number;
    complete: boolean;
  };
  gold_accuracy: {
    results: number;
    correct: number;
    accuracy: number | null;
    distribution: { contributors: number; min: number | null; median: number | null; max: number | null };
    by_contributor: { contributor: string; results: number; accuracy: number | null }[];
  };
  agreement: ProofStat[];
  review: { completed: number; routed: number; review_rate: number | null; reasons: Record<string, number> };
  contributors: { count: number; ids: string[] };
  files: { name: string; format: string; records: number; bytes: number; sha256: string }[];
  notes: string[];
};

export type ProofRecord = {
  item_id: string;
  external_id: string | null;
  input: { text: string };
  answer: { labels: string[] };
  decided_by: string;
  revision: number;
  agreement: number | null;
  answers: number;
  contributors: string[];
  finalised_at: string | null;
};

const read = (name: string) => readFileSync(join(process.cwd(), "public", "proof", name), "utf8");

export function loadProof() {
  const manifestText = read(proofFiles.manifest);
  const manifest = JSON.parse(manifestText) as ProofManifest;
  const records = read(proofFiles.jsonl)
    .split("\n")
    .filter((l) => l.trim() !== "")
    .map((l) => JSON.parse(l) as ProofRecord);
  return { manifest, manifestBytes: Buffer.byteLength(manifestText), records };
}

/** 0.8684 → "86.8%" */
export const percent = (v: number | null) =>
  v === null ? "–" : `${(v * 100).toFixed(1).replace(/\.0$/, "")}%`;

/** File size for people: 3853 → "3.8 KB" */
export const fileSize = (bytes: number) => (bytes < 1024 ? `${bytes} B` : `${(bytes / 1024).toFixed(1)} KB`);
