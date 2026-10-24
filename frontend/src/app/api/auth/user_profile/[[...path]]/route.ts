import { proxyAuthenticatedRequest } from "@/lib/auth/backend-proxy";
const methods = new Set(["GET", "PATCH", "PUT", "DELETE"]);
type Context = { params: Promise<{ path?: string[] }> };
const handle = (request: Request, context: Context) => proxyAuthenticatedRequest(request, context, "auth/user_profile", methods, request.method === "PUT" ? 512 * 1024 : 8192);
export { handle as GET, handle as PATCH, handle as PUT, handle as DELETE };
