import { proxyAuthenticatedRequest } from "@/lib/auth/backend-proxy";
import { hasSameOrigin } from "@/lib/auth/request-origin";

const methods = new Set(["GET", "POST", "PATCH", "DELETE"]);
type Context = { params: Promise<{ path?: string[] }> };

async function route(request: Request, context: Context) {
  if (request.method !== "GET" && request.method !== "HEAD" && !hasSameOrigin(request)) {
    return Response.json({ detail: "Same-origin request required" }, { status: 403 });
  }
  return proxyAuthenticatedRequest(request, context, "settings/company-profile", methods, 1024 * 1024);
}

export const GET = route;
export const POST = route;
export const PATCH = route;
export const DELETE = route;
