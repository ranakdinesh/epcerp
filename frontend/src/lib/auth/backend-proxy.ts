import { cookies } from "next/headers";
import { NextResponse } from "next/server";

import { AUTH_COOKIE_NAMES } from "@/lib/auth/session";

import { backendApiUrl } from "@/lib/auth/backend-config";

type ProxyContext = { params: Promise<{ path?: string[] }> };

export async function proxyAuthenticatedRequest(
  request: Request,
  context: ProxyContext,
  resource: string,
  allowedMethods: ReadonlySet<string>,
  maxBodyBytes?: number,
  authorizeBeforeBody: boolean | "HEAD" = false,
) {
  if (!allowedMethods.has(request.method)) {
    return NextResponse.json({ message: "Method not allowed" }, { status: 405 });
  }
  const cookieStore = await cookies();
  const accessToken = cookieStore.get(AUTH_COOKIE_NAMES.accessToken)?.value;
  if (!accessToken) return expiredResponse();
  const { path = [] } = await context.params;
  const backendUrl = new URL(`${backendApiUrl}/${resource}${path.length ? `/${path.join("/")}` : ""}`);
  backendUrl.search = new URL(request.url).search;
  // Sensitive uploads authorize their exact context before this proxy buffers bytes.
  if (authorizeBeforeBody) {
    try {
      const authority = await fetch(backendUrl, {method: authorizeBeforeBody === "HEAD" ? "HEAD" : "GET", headers: {Authorization: `Bearer ${accessToken}`}, cache: "no-store"});
      if (authority.status === 401) return expiredResponse();
      if (!authority.ok) return new NextResponse(await authority.arrayBuffer(), {status: authority.status, headers: {"Content-Type": authority.headers.get("Content-Type") ?? "application/json", "Cache-Control": "private, no-store"}});
    } catch { return NextResponse.json({detail: "Service temporarily unavailable"}, {status: 503}); }
  }
  // Preserve the browser's original payload and content type. This keeps JSON APIs
  // working while allowing multipart contract evidence to pass through untouched.
  const rawBody =
    request.method === "GET" || request.method === "HEAD"
      ? undefined
      : maxBodyBytes === undefined ? await request.arrayBuffer() : await readBoundedBody(request, maxBodyBytes);
  if (rawBody === null) return NextResponse.json({ detail: "Request exceeds the permitted size" }, { status: 413 });
  const body = rawBody && rawBody.byteLength > 0 ? rawBody : undefined;
  const contentType = request.headers.get("content-type");
  const send = (token: string) =>
    fetch(backendUrl, {
      method: request.method,
      headers: {
        Authorization: `Bearer ${token}`,
        ...(contentType ? { "Content-Type": contentType } : {}),
        ...Object.fromEntries(["Idempotency-Key", "X-Upload-Version"].flatMap(name => { const value = request.headers.get(name); return value ? [[name, value]] : []; })),
      },
      body,
      cache: "no-store",
    });
  try {
    const backendResponse = await send(accessToken);
    if (backendResponse.status === 401) return expiredResponse();
    const headers = new Headers();
    const contentTypeHeader = backendResponse.headers.get("Content-Type");
    if (contentTypeHeader) headers.set("Content-Type", contentTypeHeader);
    const contentDisposition = backendResponse.headers.get("Content-Disposition");
    if (contentDisposition) headers.set("Content-Disposition", contentDisposition);
    const encStorage = backendResponse.headers.get("X-Encrypted-Storage");
    if (encStorage) headers.set("X-Encrypted-Storage", encStorage);

    for (const name of ["Idempotency-Replayed", "Location", "Retry-After", "X-Request-ID"]) {
      const value = backendResponse.headers.get(name); if (value) headers.set(name, value);
    }
    headers.set("Cache-Control", "private, no-store");
    headers.set("X-Content-Type-Options", "nosniff");
    const data = backendResponse.status === 204 || request.method === "HEAD" ? null : await backendResponse.arrayBuffer();
    return new NextResponse(data, {
      status: backendResponse.status,
      headers,
    });

  } catch {
    return NextResponse.json({ message: "Service temporarily unavailable" }, { status: 503 });
  }
}

function expiredResponse() {
  const response = NextResponse.json({ message: "Session expired" }, { status: 401 });
  response.headers.set("X-VTPL-Session-Expired", "1");
  return response;
}

// Bound new account/photo routes before buffering or forwarding their payloads.
async function readBoundedBody(request: Request, limit: number): Promise<ArrayBuffer | null> {
  const reader = request.body?.getReader();
  if (!reader) return new ArrayBuffer(0);
  const chunks: Uint8Array[] = []; let size = 0;
  try {
    while (true) {
      const { done, value } = await reader.read(); if (done) break;
      size += value.byteLength;
      if (size > limit) { await reader.cancel(); return null; }
      chunks.push(value);
    }
  } finally { reader.releaseLock(); }
  const body = new Uint8Array(size); let offset = 0;
  for (const chunk of chunks) { body.set(chunk, offset); offset += chunk.byteLength; }
  return body.buffer;
}
