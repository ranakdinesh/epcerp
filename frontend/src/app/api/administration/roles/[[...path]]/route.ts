import { proxyAuthenticatedRequest } from "@/lib/auth/backend-proxy";
import { roleBodyLimit } from "@/lib/auth/role-limits";

const allowedMethods = new Set(["GET", "POST", "PATCH"]);
function proxy(request: Request, context: { params: Promise<{ path?: string[] }> }) {
  return proxyAuthenticatedRequest(request, context, "roles", allowedMethods, roleBodyLimit);
}
export const GET = proxy;
export const POST = proxy;
export const PATCH = proxy;
