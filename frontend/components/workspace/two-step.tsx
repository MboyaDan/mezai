"use client";

import { useEffect, useState, type FormEvent } from "react";
import QRCode from "qrcode";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/choice";
import { Dialog } from "@/components/ui/dialog";
import { Alert, Card, Skeleton, StatusPill } from "@/components/ui/display";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { useToast } from "@/components/ui/toast";
import { ApiError, api } from "@/lib/api/client";
import { errorMessage } from "@/lib/onboarding";
import { groupSecret, recoveryFile } from "@/lib/two-step";
import { useLoad } from "@/lib/use-load";
import { useUser } from "@/lib/session";
import { shortDate } from "./contributors";
import { PageTitle } from "./workspace";

type Status = {
  available: boolean;
  enabled: boolean;
  enabled_at: string | null;
  recovery_codes_left: number;
};
type Setup = { secret: string; uri: string };

const asError = (e: unknown) => (e instanceof ApiError ? e : new ApiError(0, "network"));

/** Account page for staff (Stage 17d): optional two-step sign-in. */
export function AccountPage() {
  const user = useUser();
  return (
    <>
      <PageTitle title="Account">
        {user.display_name} · {user.email}
      </PageTitle>
      <TwoStep />
    </>
  );
}

function TwoStep() {
  const toast = useToast();
  const status = useLoad(() => api<Status>("GET", "/v1/me/two-step"));
  const [setup, setSetup] = useState<Setup | null>(null);
  const [codes, setCodes] = useState<string[] | null>(null);
  const [off, setOff] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);

  async function start() {
    setBusy(true);
    setError(null);
    try {
      setSetup(await api<Setup>("POST", "/v1/me/two-step/setup"));
    } catch (e) {
      setError(asError(e));
    } finally {
      setBusy(false);
    }
  }

  if (status.status === "loading") return <Skeleton className="h-48 w-full" />;
  if (status.status === "error") return <Alert tone="danger" title={errorMessage(status.error)} />;
  const s = status.data;

  return (
    <Card className="grid max-w-2xl gap-4 p-5" role="region" aria-labelledby="two-step-h">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 id="two-step-h" className="font-display font-semibold">
          Two-step sign-in
        </h2>
        {s.enabled ? (
          <StatusPill status="success">On</StatusPill>
        ) : (
          <StatusPill status="neutral">Off</StatusPill>
        )}
      </div>
      <p className="text-sm text-fg-muted">
        After your password, MezAI also asks for a 6-digit code from an authenticator app on your phone (for
        example Google Authenticator, Microsoft Authenticator or 1Password). Recommended for anyone who
        approves payouts or manages people.
      </p>
      {error && <Alert tone="danger" title={errorMessage(error)} />}

      {codes ? (
        <RecoveryCodes
          codes={codes}
          onDone={() => {
            setCodes(null);
            status.reload();
          }}
        />
      ) : setup ? (
        <SetupSteps
          setup={setup}
          onCancel={() => setSetup(null)}
          onDone={(c) => {
            setSetup(null);
            setCodes(c);
            // Show "On" at once; a reload would hide the codes behind a skeleton.
            status.set({
              ...s,
              enabled: true,
              enabled_at: new Date().toISOString(),
              recovery_codes_left: c.length,
            });
            toast({ tone: "success", title: "Two-step sign-in is on" });
          }}
        />
      ) : s.enabled ? (
        <>
          <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2 text-sm">
            <dt className="text-fg-muted">On since</dt>
            <dd>{shortDate(s.enabled_at)}</dd>
            <dt className="text-fg-muted">Recovery codes left</dt>
            <dd>{s.recovery_codes_left} of 10</dd>
          </dl>
          {s.recovery_codes_left <= 3 && (
            <Alert tone="warning" title="Few recovery codes left">
              Turn two-step sign-in off and on again to get ten new codes.
            </Alert>
          )}
          <div>
            <Button variant="secondary" size="sm" onClick={() => setOff(true)}>
              Turn off
            </Button>
          </div>
        </>
      ) : s.available ? (
        <div>
          <Button size="sm" onClick={() => void start()} loading={busy}>
            Turn on two-step sign-in
          </Button>
        </div>
      ) : (
        <p className="text-sm">Two-step sign-in is not set up on this server yet.</p>
      )}

      <TurnOff
        open={off}
        onClose={() => setOff(false)}
        onDone={() => {
          setOff(false);
          toast({ tone: "success", title: "Two-step sign-in is off" });
          status.reload();
        }}
      />
    </Card>
  );
}

function SetupSteps({
  setup,
  onCancel,
  onDone,
}: {
  setup: Setup;
  onCancel: () => void;
  onDone: (codes: string[]) => void;
}) {
  const [qr, setQr] = useState<string | null>(null);
  const [code, setCode] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);

  useEffect(() => {
    let live = true;
    QRCode.toDataURL(setup.uri, { margin: 1, width: 192, errorCorrectionLevel: "M" })
      .then((u) => live && setQr(u))
      .catch(() => live && setQr(null));
    return () => {
      live = false;
    };
  }, [setup.uri]);

  async function confirm(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const out = await api<{ recovery_codes: string[] }>("POST", "/v1/me/two-step/confirm", { code });
      onDone(out.recovery_codes);
    } catch (err) {
      setError(asError(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={confirm} className="grid gap-4 border-t border-border pt-4" noValidate>
      <ol className="grid list-decimal gap-4 pl-5 text-sm">
        <li>
          <p>Scan this code with your authenticator app.</p>
          {qr ? (
            // eslint-disable-next-line @next/next/no-img-element -- a data URL made here, not a remote image
            <img
              src={qr}
              width={192}
              height={192}
              alt="QR code for adding MezAI to an authenticator app"
              className="mt-2 rounded-control border border-border bg-white p-1"
            />
          ) : (
            <Skeleton className="mt-2 size-48" />
          )}
          <p className="mt-2 text-fg-muted">
            Cannot scan? Enter this key instead:{" "}
            <span className="font-mono text-fg" aria-label={`Key ${setup.secret.split("").join(" ")}`}>
              {groupSecret(setup.secret)}
            </span>
          </p>
        </li>
        <li>
          <Field
            label="Type the 6-digit code the app shows"
            error={error?.code === "invalid_code" ? errorMessage(error) : undefined}
            required
          >
            <Input
              value={code}
              onChange={(e) => setCode(e.target.value)}
              inputMode="numeric"
              autoComplete="one-time-code"
              maxLength={7}
              className="max-w-40 font-mono"
            />
          </Field>
        </li>
      </ol>
      {error && error.code !== "invalid_code" && <Alert tone="danger" title={errorMessage(error)} />}
      <div className="flex flex-wrap gap-2">
        <Button type="submit" size="sm" loading={busy}>
          Turn on
        </Button>
        <Button type="button" variant="ghost" size="sm" onClick={onCancel}>
          Cancel
        </Button>
      </div>
    </form>
  );
}

function RecoveryCodes({ codes, onDone }: { codes: string[]; onDone: () => void }) {
  const [saved, setSaved] = useState(false);
  const download = () => {
    const url = URL.createObjectURL(new Blob([recoveryFile(codes, new Date())], { type: "text/plain" }));
    const a = document.createElement("a");
    a.href = url;
    a.download = "mezai-recovery-codes.txt";
    a.click();
    URL.revokeObjectURL(url);
  };
  return (
    <div className="grid gap-4 border-t border-border pt-4">
      <Alert tone="warning" title="Save these recovery codes now">
        If you lose your phone, each code signs you in once. They are shown only this time. Keep them
        somewhere safe, such as a password manager.
      </Alert>
      <ul className="grid grid-cols-2 gap-2 font-mono text-sm sm:grid-cols-5" aria-label="Recovery codes">
        {codes.map((c) => (
          <li key={c} className="rounded-control border border-border px-2 py-1 text-center">
            {c}
          </li>
        ))}
      </ul>
      <div className="flex flex-wrap items-center gap-3">
        <Button variant="secondary" size="sm" onClick={download}>
          Download as a text file
        </Button>
        <Checkbox checked={saved} onChange={(e) => setSaved(e.target.checked)} label="I have saved them" />
        <Button size="sm" disabled={!saved} onClick={onDone}>
          Done
        </Button>
      </div>
    </div>
  );
}

function TurnOff({ open, onClose, onDone }: { open: boolean; onClose: () => void; onDone: () => void }) {
  const [password, setPassword] = useState("");
  const [code, setCode] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);
  async function submit() {
    setBusy(true);
    setError(null);
    try {
      await api("POST", "/v1/me/two-step/disable", { password, code });
      setPassword("");
      setCode("");
      onDone();
    } catch (e) {
      setError(asError(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <Dialog
      open={open}
      onClose={() => {
        setError(null);
        onClose();
      }}
      title="Turn off two-step sign-in?"
      description="Your password alone will sign you in again, and your recovery codes stop working."
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="danger" loading={busy} onClick={() => void submit()}>
            Turn off
          </Button>
        </>
      }
    >
      {error && <Alert tone="danger" title={errorMessage(error)} className="mb-4" />}
      <div className="grid gap-4">
        <Field label="Password" required>
          <Input
            type="password"
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </Field>
        <Field label="Code from the app, or a recovery code" required>
          <Input value={code} onChange={(e) => setCode(e.target.value)} autoComplete="one-time-code" />
        </Field>
      </div>
    </Dialog>
  );
}
