import { proxyAuthenticatedRequest } from "@/lib/auth/backend-proxy";
type Context = { params: Promise<{ path?: string[] }> };
const legacy = new Set(["GET", "POST"]);
async function route(request: Request, context: Context) {
  const { path = [] } = await context.params;
  const joined = path.join("/");
  const upload = /^receipts\/[0-9a-f-]{36}\/files\/[0-9a-f-]{36}$/.test(joined);
  const named = /^records(?:\/[0-9a-f-]{36}(?:\/manager-assignments)?)?$/.test(joined);
  const receipt = /^receipts(?:\/[0-9a-f-]{36}(?:\/seal)?)?$/.test(joined);
  if (["POST", "PUT", "PATCH", "DELETE"].includes(request.method) && (named || receipt || upload)) {
    const origin = request.headers.get("origin");
    if (origin !== new URL(request.url).origin) return Response.json({ detail: "Same-origin request required" }, { status: 403 });
  }
  const methods = upload ? new Set(["PUT", "HEAD"]) : named ? new Set(["GET", "POST", "PATCH"]) : receipt ? new Set(["GET", "POST"]) : legacy;
  const limit = upload ? 64 * 1024 * 1024 : receipt || named ? 32 * 1024 : joined === "upload" ? 32 * 1024 * 1024 + 64 * 1024 : joined.endsWith("/ownership") ? 8192 : 1024;
  return proxyAuthenticatedRequest(request, context, "tender-fms", methods, limit, upload && request.method === "PUT" ? "HEAD" : false);
}
export const GET = route;
export const POST = route;
export const PATCH = route;
export const PUT = route;
export const HEAD = route;
