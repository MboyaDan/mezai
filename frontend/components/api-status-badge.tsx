"use client";

import { useEffect, useState } from "react";
import { checkApi, statusLabel, type ApiStatus } from "@/lib/api-status";

const dot: Record<ApiStatus, string> = {
  checking: "bg-[#A3A99C]",
  connected: "bg-[#C8F031]",
  unreachable: "bg-[#C2522D]",
};

export function ApiStatusBadge() {
  const [status, setStatus] = useState<ApiStatus>("checking");

  useEffect(() => {
    let live = true;
    checkApi().then((s) => live && setStatus(s));
    return () => {
      live = false;
    };
  }, []);

  return (
    <p role="status" data-status={status} className="inline-flex items-center gap-3 font-mono text-sm">
      <span aria-hidden="true" className={`size-2 rounded-full ${dot[status]}`} />
      {statusLabel[status]}
    </p>
  );
}
