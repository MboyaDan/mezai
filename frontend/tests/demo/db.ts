import { execFileSync } from "node:child_process";
import { createHash, randomBytes } from "node:crypto";

// The demo runner's few direct database steps, through psql in Docker (the
// same image backend/scripts/staging.sh uses). DEMO_DB_URL is set by
// `staging.sh demo`, or by hand for a local run. Values go in as psql
// variables, never pasted into the SQL.

const image = process.env.PG_IMAGE ?? "postgres:17-alpine";

export function dbUrl(): string {
  const url = process.env.DEMO_DB_URL;
  if (!url) throw new Error("DEMO_DB_URL is not set (run through backend/scripts/staging.sh demo)");
  return url;
}

/** Runs SQL and returns the rows as tab-separated lines. */
export function sql(query: string, vars: Record<string, string> = {}): string[] {
  const args = [
    "run",
    "--rm",
    "-i",
    image,
    "psql",
    dbUrl(),
    "-q",
    "-At",
    "-F",
    "\t",
    "-v",
    "ON_ERROR_STOP=1",
  ];
  for (const [k, v] of Object.entries(vars)) args.push("-v", `${k}=${v}`);
  const out = execFileSync("docker", args, { input: query, encoding: "utf8" });
  return out.split("\n").filter((l) => l !== "");
}

/**
 * The demo shows the automatic pause with a 10-item gold set, so trust
 * scores stop "building" after 5 gold results instead of 20 (D-159). Set
 * once, with an audit entry; production keeps 20.
 */
export function useDemoTrustSetting() {
  sql(`
    WITH old AS (SELECT value FROM settings WHERE key = 'trust.min_gold_results'),
    upd AS (
      UPDATE settings SET value = '5'::jsonb WHERE key = 'trust.min_gold_results' AND value <> '5'::jsonb
      RETURNING value
    )
    INSERT INTO audit_log (actor_kind, action, entity_type, reason, before, after)
    SELECT 'system', 'setting.changed', 'setting',
           'Pilot demo on staging: show the automatic pause with a 10-item gold set (D-159)',
           jsonb_build_object('trust.min_gold_results', (SELECT value FROM old)),
           jsonb_build_object('trust.min_gold_results', value)
    FROM upd;`);
}

/**
 * Mail to .invalid addresses is not sent on staging (D-159), so the demo
 * confirms its new contributor the way the email would: it makes a fresh
 * one-time verification token and returns the link path. Only the token's
 * hash is stored, as the API does.
 */
export function verificationPath(email: string): string {
  const token = randomBytes(32).toString("base64url");
  const hash = createHash("sha256").update(token).digest("hex");
  const rows = sql(
    `INSERT INTO email_verifications (user_id, token_hash, expires_at)
     SELECT id, decode(:'hash', 'hex'), now() + interval '1 hour' FROM users WHERE email = :'email'
     RETURNING id;`,
    { hash, email },
  );
  if (rows.length !== 1) throw new Error(`no account for ${email}`);
  return `/verify-email?token=${token}`;
}

/** The newest enquiry with this email, as Ops would see it. */
export function enquiry(email: string) {
  const [row] = sql(
    `SELECT e.id, e.status, (SELECT count(*) FROM email_outbox o WHERE o.template = 'enquiry_received' AND o.created_at >= e.created_at)
     FROM enquiries e WHERE e.email = :'email' ORDER BY e.created_at DESC LIMIT 1;`,
    { email },
  );
  if (!row) return null;
  const [id, status, notices] = row.split("\t");
  return { id, status, notices: Number(notices) };
}

/** A contributor's ID from their sign-in email. */
export function contributorId(email: string): string {
  const [id] = sql(
    `SELECT cp.id FROM contributor_profiles cp JOIN users u ON u.id = cp.user_id WHERE u.email = :'email';`,
    { email },
  );
  if (!id) throw new Error(`no contributor ${email}`);
  return id;
}

/** A contributor's status and their newest trust score. */
export function contributorState(email: string) {
  const [row] = sql(
    `SELECT cp.status, coalesce(t.score::text, ''), coalesce(t.gold_count, 0)
     FROM contributor_profiles cp JOIN users u ON u.id = cp.user_id
     LEFT JOIN LATERAL (SELECT score, gold_count FROM trust_scores WHERE contributor_id = cp.id ORDER BY created_at DESC LIMIT 1) t ON true
     WHERE u.email = :'email';`,
    { email },
  );
  const [status, score, gold] = (row ?? "").split("\t");
  return { status, score: score === "" ? null : Number(score), gold: Number(gold ?? 0) };
}

/**
 * Like verificationPath, for an invitation Ops just sent to a .invalid
 * address: the pending invitation gets a fresh token, and the link path is
 * returned.
 */
export function invitationPath(email: string): string {
  const token = randomBytes(32).toString("base64url");
  const hash = createHash("sha256").update(token).digest("hex");
  const rows = sql(
    `UPDATE invitations SET token_hash = decode(:'hash', 'hex')
     WHERE email = :'email' AND accepted_at IS NULL AND revoked_at IS NULL AND expires_at > now()
     RETURNING id;`,
    { hash, email },
  );
  if (rows.length !== 1) throw new Error(`no pending invitation for ${email}`);
  return `/accept-invitation?token=${token}`;
}

/** Review decisions and reviewer feedback on a project. */
export function reviewCounts(projectId: string) {
  const [row] = sql(
    `SELECT (SELECT count(*) FROM reviews WHERE project_id = :'p'),
            (SELECT count(*) FROM feedback WHERE project_id = :'p' AND author_user_id IS NOT NULL)`,
    { p: projectId },
  );
  const [reviews, feedback] = row.split("\t").map(Number);
  return { reviews, feedback };
}

/**
 * Reconciliation for a settled batch: what the paid lines total, and what
 * the ledger's payout entries for them total (negative).
 */
export function batchReconciliation(batchId: string) {
  const [row] = sql(
    `SELECT coalesce(sum(l.amount_cents) FILTER (WHERE l.status = 'paid'), 0),
            coalesce((SELECT sum(e.amount_cents) FROM earnings_ledger e JOIN payout_lines pl ON pl.id = e.payout_line_id
                      WHERE pl.batch_id = :'b' AND e.entry_type = 'payout'), 0),
            count(*) FILTER (WHERE l.status = 'pending')
     FROM payout_lines l WHERE l.batch_id = :'b';`,
    { b: batchId },
  );
  const [paid, ledger, pending] = row.split("\t").map(Number);
  return { paid, ledger, pending };
}
