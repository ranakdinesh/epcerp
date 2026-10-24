import { proxyAuthenticatedRequest } from "@/lib/auth/backend-proxy";

const methods = new Set(["GET", "POST", "PATCH"]);
const proxy = (request: Request, context: { params: Promise<{ path?: string[] }> }) =>
  proxyAuthenticatedRequest(request, context, "categories", methods, 1024 * 1024);
export const GET = proxy;
export const POST = proxy;
export const PATCH = proxy;
