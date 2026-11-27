import { Suspense, type ReactNode } from "react";
import { Workspace } from "@/components/workspace/workspace";
import { serverMe } from "@/lib/api/server";

// The session is read on the server so the first HTML already holds the
// shell and the page (Stage 26, D-168; the contributor side is D-165). If the read fails, the
// workspace loads the session in the browser as before.
async function ServerWorkspace({ children }: { children: ReactNode }) {
  const me = await serverMe();
  return <Workspace me={me}>{children}</Workspace>;
}

const blank = <div className="min-h-dvh bg-surface" aria-busy="true" />;

export default function WorkspaceLayout({ children }: { children: ReactNode }) {
  return (
    <Suspense fallback={blank}>
      <ServerWorkspace>{children}</ServerWorkspace>
    </Suspense>
  );
}
