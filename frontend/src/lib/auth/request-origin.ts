// Next.js may construct request.url from its bind hostname rather than the
// browser-facing host. Host retains the requested authority, including its port.
// Do not trust arbitrary forwarded host/protocol headers here.
export function hasSameOrigin(request: Request): boolean {
  const origin = request.headers.get("origin");
  const host = request.headers.get("host");
  if (!origin || !host) return false;
  try {
    const supplied = new URL(origin);
    const expected = new URL(request.url);
    expected.host = host;
    return origin === supplied.origin && supplied.origin === expected.origin;
  } catch {
    return false;
  }
}
