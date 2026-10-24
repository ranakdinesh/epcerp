import { proxyAuthenticatedRequest } from "@/lib/auth/backend-proxy";
const methods = new Set(["GET", "POST", "PATCH"]);
const proxy = (
  request: Request,
  context: { params: Promise<{ path?: string[] }> },
) => proxyAuthenticatedRequest(request, context, "manufacturers", methods);
export const GET = proxy;
export const POST = proxy;
export const PATCH = proxy;
