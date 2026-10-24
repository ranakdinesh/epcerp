import { proxyAuthenticatedRequest } from "@/lib/auth/backend-proxy";
export function PATCH(request: Request) { return proxyAuthenticatedRequest(request, { params: Promise.resolve({}) }, "auth/me/email", new Set(["PATCH"]), 8192); }
