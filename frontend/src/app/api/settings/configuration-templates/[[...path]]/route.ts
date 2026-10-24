import { proxyAuthenticatedRequest } from "@/lib/auth/backend-proxy";
const methods = new Set(["GET", "POST"]);
const proxy = (request: Request, context: { params: Promise<{ path?: string[] }> }) =>
  proxyAuthenticatedRequest(request, context, "configuration-templates", methods, 1024 * 1024);
export const GET = proxy;
export const POST = proxy;
