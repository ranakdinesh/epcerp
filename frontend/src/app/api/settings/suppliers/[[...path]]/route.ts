import { proxyAuthenticatedRequest } from "@/lib/auth/backend-proxy";
const methods = new Set(["GET", "HEAD", "POST", "PATCH", "PUT"]);
async function proxy(request: Request, context: { params: Promise<{ path?: string[] }> }) {
 const { path = [] } = await context.params;
 const upload = request.method === "POST" && path.length === 2 && /^[0-9a-f-]{36}$/.test(path[0]) && path[1] === "documents";
 return proxyAuthenticatedRequest(request, context, "suppliers", methods, upload ? 15 * 1024 * 1024 + 64 * 1024 : path.length <= 1 ? 128 * 1024 : 512 * 1024, upload && process.env.EPCERP_VENDOR_DOCUMENTS_OWNER === "nextjs" ? "HEAD" : false);
}
export const GET = proxy;
export const POST = proxy;
export const PATCH = proxy;

export const PUT = proxy;

export const HEAD = proxy;
