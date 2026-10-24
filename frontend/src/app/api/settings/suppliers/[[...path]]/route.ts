import { proxyAuthenticatedRequest } from "@/lib/auth/backend-proxy";
const methods = new Set(["GET", "POST", "PATCH", "PUT"]);
async function proxy(request: Request, context: { params: Promise<{ path?: string[] }> }) {
 const { path = [] } = await context.params;
 const upload = request.method === "POST" && path.length === 2 && /^[0-9a-f-]{36}$/.test(path[0]) && path[1] === "documents";
 return proxyAuthenticatedRequest(request, context, "suppliers", methods, upload ? 15 * 1024 * 1024 + 64 * 1024 : 512 * 1024);
}
export const GET = proxy;
export const POST = proxy;
export const PATCH = proxy;

export const PUT = proxy;
