import {proxyAuthenticatedRequest} from "@/lib/auth/backend-proxy";
export async function GET(request:Request){return proxyAuthenticatedRequest(request,{params:Promise.resolve({})},"indent-dispatch-notifications",new Set(["GET"]));}
