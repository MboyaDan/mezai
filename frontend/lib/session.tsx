"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { usePathname, useRouter } from "next/navigation";
import { api, setCsrfToken, type Me } from "@/lib/api/client";
import { homeFor } from "@/lib/onboarding";

type User = Me["user"];
type State = { status: "loading" } | { status: "anonymous" } | { status: "signed-in"; user: User };

type SessionValue = {
  state: State;
  /** Store the session returned by login. */
  signedIn: (me: Me) => void;
  signOut: () => Promise<void>;
  refresh: () => Promise<void>;
};

const SessionContext = createContext<SessionValue | null>(null);

/** Loads /v1/auth/me once and shares the signed-in user. The API checks every call regardless. */
export function SessionProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<State>({ status: "loading" });

  const refresh = useCallback(async () => {
    try {
      const me = await api<Me>("GET", "/v1/auth/me");
      setCsrfToken(me.csrf_token);
      setState({ status: "signed-in", user: me.user });
    } catch {
      // 401 means signed out; a network error is treated the same, and the
      // sign-in page reports it if the API is unreachable.
      setCsrfToken(null);
      setState({ status: "anonymous" });
    }
  }, []);

  useEffect(() => {
    // Loading the session is the effect's purpose; the state update is its result.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void refresh();
  }, [refresh]);

  const signedIn = useCallback((me: Me) => {
    setCsrfToken(me.csrf_token);
    setState({ status: "signed-in", user: me.user });
  }, []);

  const signOut = useCallback(async () => {
    try {
      await api("POST", "/v1/auth/logout");
    } finally {
      setCsrfToken(null);
      setState({ status: "anonymous" });
    }
  }, []);

  const value = useMemo(() => ({ state, signedIn, signOut, refresh }), [state, signedIn, signOut, refresh]);
  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>;
}

export function useSession() {
  const v = useContext(SessionContext);
  if (!v) throw new Error("useSession must be used inside <SessionProvider>");
  return v;
}

/** The signed-in user; only call below <RequireSession>. */
export function useUser(): User {
  const { state } = useSession();
  if (state.status !== "signed-in") throw new Error("useUser needs a signed-in session");
  return state.user;
}

/**
 * Renders children only for a signed-in user of one of `kinds`. Anonymous
 * visitors go to /login with a return path; other kinds go to their own home.
 */
export function RequireSession({
  kinds,
  fallback,
  children,
}: {
  kinds: User["kind"][];
  fallback: ReactNode;
  children: ReactNode;
}) {
  const { state } = useSession();
  const router = useRouter();
  const path = usePathname();
  const allowed = state.status === "signed-in" && kinds.includes(state.user.kind);

  useEffect(() => {
    if (state.status === "anonymous") router.replace(`/login?next=${encodeURIComponent(path)}`);
    else if (state.status === "signed-in" && !kinds.includes(state.user.kind))
      router.replace(homeFor(state.user.kind));
  }, [state, kinds, path, router]);

  return allowed ? children : fallback;
}
