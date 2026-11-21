import { proxyAuthenticatedRequest } from "@/lib/auth/backend-proxy";
export function GET(request:Request){return proxyAuthenticatedRequest(request,{params:Promise.resolve({})},"auth/administration-history",new Set(["GET"]));}
