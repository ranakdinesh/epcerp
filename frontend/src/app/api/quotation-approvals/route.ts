import { proxyAuthenticatedRequest } from "@/lib/auth/backend-proxy";
export function GET(request: Request) {
 return proxyAuthenticatedRequest(request, {params:Promise.resolve({path:[]})}, "quotation-approvals",new Set(["GET"]),64*1024);
}
