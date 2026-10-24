import { proxyAuthenticatedRequest } from "@/lib/auth/backend-proxy";
export function GET(request: Request) {
 return proxyAuthenticatedRequest(request, {params:Promise.resolve({path:[]})}, "my-tasks",new Set(["GET"]),64*1024);
}
