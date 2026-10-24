import assert from "node:assert/strict";
import test from "node:test";
import { readFileSync } from "node:fs";
import ts from "typescript";
import { NextResponse } from "next/server.js";

function load(path, dependencies, fetcher) {
  const source = readFileSync(new URL(path, import.meta.url), "utf8");
  const { outputText } = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } });
  const loaded = { exports: {} };
  new Function("require", "exports", "fetch", outputText)(name => dependencies[name], loaded.exports, fetcher);
  return loaded.exports;
}
const names = { accessToken: "epcerp_access_token", refreshToken: "epcerp_refresh_token" };
const cookieHelpers = load("../src/lib/auth/cookies.ts", {
  "next/server": { NextResponse },
    "@/lib/auth/backend-config": { backendApiUrl: "http://127.0.0.1:8001/api/v1" },
  "@/lib/auth/session": { AUTH_COOKIE_NAMES: names },
  "@/lib/auth/cookie-policy": { authCookieOptions: () => ({ httpOnly: true, sameSite: "lax", secure: true }) },
});
function refreshRoute(token, fetcher) {
  return load("../src/app/api/auth/refresh/route.ts", {
    "next/server": { NextResponse },
    "@/lib/auth/backend-config": { backendApiUrl: "http://127.0.0.1:8001/api/v1" },
    "next/headers": { cookies: async () => new Map(token ? [[names.refreshToken, { value: token }]] : []) },
    "@/lib/auth/session": { AUTH_COOKIE_NAMES: names },
    "@/lib/auth/cookies": cookieHelpers,
  }, fetcher).POST;
}
test("refresh uses backend remaining lifetime and keeps tokens HttpOnly", async () => {
  const response = await refreshRoute("opaque", async () => Response.json({ access_token: "access", refresh_token: "replacement", expires_in: 900, refresh_expires_in: 1800 }))();
  assert.deepEqual(await response.json(), { ok: true });
  assert.equal(response.cookies.get(names.refreshToken).maxAge, 1800);
  assert.equal(response.cookies.get(names.refreshToken).path, "/api/auth");
  assert.equal(response.cookies.get(names.refreshToken).httpOnly, true);
  assert.equal(response.cookies.get(names.accessToken).maxAge, 900);
});
test("expired or missing sessions clear stale access cookies", async () => {
  for (const token of [undefined, "expired"]) {
    const response = await refreshRoute(token, async () => new Response(null, { status: 401 }))();
    assert.equal(response.status, 401);
    assert.equal(response.cookies.get(names.accessToken).maxAge, 0);
    assert.equal(response.cookies.get(names.refreshToken).maxAge, 0);
  }
});
test("outage and rate limit leave session cookies intact", async () => {
  for (const status of [429, 500, 503]) {
    const response = await refreshRoute("opaque", async () => new Response(null, { status }))();
    assert.equal(response.status, 503);
    assert.equal(response.headers.get("set-cookie"), null);
  }
});
test("missing access token signals renewal without deleting the refresh cookie or forwarding writes", async () => {
  const { proxyAuthenticatedRequest } = load("../src/lib/auth/backend-proxy.ts", {
    "next/server": { NextResponse },
    "@/lib/auth/backend-config": { backendApiUrl: "http://127.0.0.1:8001/api/v1" },
    "next/headers": { cookies: async () => new Map() },
    "@/lib/auth/session": { AUTH_COOKIE_NAMES: names },
  }, () => { throw new Error("Must not forward unauthenticated write"); });
  const response = await proxyAuthenticatedRequest(new Request("https://example.invalid/api/projects", { method: "POST", body: "{}" }), { params: Promise.resolve({}) }, "projects", new Set(["POST"]));
  assert.equal(response.status, 401);
  assert.equal(response.headers.get("X-VTPL-Session-Expired"), "1");
  assert.equal(response.headers.get("set-cookie"), null);
});
