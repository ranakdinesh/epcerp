import "server-only";
import { NextResponse } from "next/server";
import { backendApiUrl } from "./backend-config";

export async function recoveryRequest(request: Request, action: "forgot-password" | "reset-password") {
  const headers = { "Cache-Control": "no-store", "Referrer-Policy": "no-referrer" };
  if (request.headers.get("origin") && request.headers.get("origin") !== new URL(request.url).origin) {
    return NextResponse.json({ message: "Invalid request origin." }, { status: 403, headers });
  }
  let payload: Record<string, unknown>;
  try {
    // Bound streamed bodies as well as Content-Length declarations.
    const reader = request.body?.getReader();
    if (!reader) throw new Error("Missing body");
    const chunks: Uint8Array[] = [];
    let length = 0;
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      length += value.byteLength;
      if (length > 4096) { await reader.cancel(); return NextResponse.json({ message: "Request is too large." }, { status: 413, headers }); }
      chunks.push(value);
    }
    payload = JSON.parse(Buffer.concat(chunks).toString("utf8"));
    const allowed = action === "forgot-password" ? ["email"] : ["token", "password"];
    if (!payload || Array.isArray(payload) || typeof payload !== "object" || Object.keys(payload).some(key => !allowed.includes(key)) || allowed.some(key => typeof payload[key] !== "string")) throw new Error("Invalid body");
  } catch {
    return NextResponse.json({ message: "Invalid recovery request." }, { status: 400, headers });
  }
  try {
    const response = await fetch(`${backendApiUrl}/auth/${action}`, {
      method: "POST", headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload), cache: "no-store", signal: AbortSignal.timeout(15000),
    });
    if (response.status === 204) return new NextResponse(null, { status: 204, headers });
    const result = await response.json().catch(() => ({}));
    // Never forward provider bodies, credentials or arbitrary backend errors.
    if (response.status === 429) return NextResponse.json({ message: "Too many attempts. Try again in 15 minutes." }, { status: 429, headers: { ...headers, "Retry-After": response.headers.get("Retry-After") ?? "900" } });
    if (response.status === 503) return NextResponse.json({ message: "Email recovery is not configured yet. Contact your administrator." }, { status: 503, headers });
    if (response.status === 422) return NextResponse.json({ message: typeof result.detail === "string" ? result.detail : "Check the highlighted fields.", detail: result.detail }, { status: 422, headers });
    if (!response.ok) throw new Error("Recovery unavailable");
    return NextResponse.json({ message: result.message }, { status: response.status, headers });
  } catch {
    return NextResponse.json({ message: "Password recovery is temporarily unavailable. Please try again." }, { status: 503, headers });
  }
}
