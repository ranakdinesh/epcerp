import { proxyAuthenticatedRequest } from "@/lib/auth/backend-proxy";
const methods = new Set(["GET", "PATCH"]);
const proxy = (request: Request, context: { params: Promise<{ path?: string[] }> }) => proxyAuthenticatedRequest(request, context, "purchase-fms", methods, 16 * 1024);
export const GET = proxy;
export const PATCH = proxy;
