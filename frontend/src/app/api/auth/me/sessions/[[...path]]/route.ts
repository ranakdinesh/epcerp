import { proxyAuthenticatedRequest } from "@/lib/auth/backend-proxy";
function proxy(request:Request,context:{params:Promise<{path?:string[]}>}){return proxyAuthenticatedRequest(request,context,"auth/me/sessions",new Set(["GET","DELETE"]),8192);}
export const GET=proxy;export const DELETE=proxy;
