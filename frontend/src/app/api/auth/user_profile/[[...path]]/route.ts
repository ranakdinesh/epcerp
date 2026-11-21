import { hasSameOrigin } from '@/lib/auth/request-origin';
import { proxyAuthenticatedRequest } from "@/lib/auth/backend-proxy";
const methods = new Set(["GET", "PATCH", "PUT", "DELETE"]);
type Context = { params: Promise<{ path?: string[] }> };
const forward = (request: Request, context: Context) => proxyAuthenticatedRequest(request, context, "auth/user_profile", methods, request.method === "PUT" ? 512 * 1024 : 8192);
const handle=(r:Request,c:Context)=>r.method!=='GET'&&!hasSameOrigin(r)?Promise.resolve(Response.json({detail:'Same-origin request required'},{status:403})):forward(r,c);
export { handle as GET, handle as PATCH, handle as PUT, handle as DELETE };
