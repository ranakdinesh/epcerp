import { proxyAuthenticatedRequest } from "@/lib/auth/backend-proxy";
const methods = new Set(["GET", "PATCH"]);
function handle(request: Request) { return proxyAuthenticatedRequest(request, { params: Promise.resolve({}) }, "auth/me/username", methods, 8192); }
export { handle as GET, handle as PATCH };
