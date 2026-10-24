"use client";

import { useEffect, useState } from "react";
import { renewSession, safeReturnPath } from "@/lib/auth/renew-session";

export default function RenewSessionPage() {
  const [unavailable, setUnavailable] = useState(false);
  useEffect(() => {
    let active = true;
    const next = safeReturnPath(new URLSearchParams(window.location.search).get("next"));
    renewSession().then((response) => {
      if (!active) return;
      if (response.ok) window.location.replace(next);
      else if (response.status === 401) window.location.replace(`/login?session=expired&next=${encodeURIComponent(next)}`);
      else setUnavailable(true);
    }).catch(() => { if (active) setUnavailable(true); });
    return () => { active = false; };
  }, []);
  return <main className="flex min-h-screen flex-col items-center justify-center gap-4">
    <p role="status">{unavailable ? "Connection unavailable. Please try again." : "Restoring your session…"}</p>
    {unavailable && <button onClick={() => window.location.reload()}>Try again</button>}
  </main>;
}
