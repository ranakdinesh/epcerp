// Coalesce concurrent requests, and serialize rotation across browser tabs.
let renewal: Promise<Response> | undefined;
export function withSessionLock<T>(action: () => Promise<T>): Promise<T> {
  return Promise.resolve(typeof navigator !== "undefined" && navigator.locks
    ? navigator.locks.request("epcerp-session-renewal", action)
    : action());
}

export function renewSession(fetcher: typeof fetch = fetch): Promise<Response> {
  if (!renewal) {
    const renew = () => fetcher("/api/auth/refresh", { method: "POST" });
    renewal = withSessionLock(renew).finally(() => { renewal = undefined; });
  }
  return renewal!;
}

export function safeReturnPath(value: string | null): string {
  return value && (value === "/dashboard" || value.startsWith("/dashboard/") || value.startsWith("/dashboard?") || value === "/tenders" || value.startsWith("/tenders?"))
    && !value.includes("\\") ? value : "/dashboard";
}
