import { hasSameOrigin } from '@/lib/auth/request-origin';
import {proxyAuthenticatedRequest} from "@/lib/auth/backend-proxy";
const methods=new Set(["GET","POST","PATCH"]);
const forward=(request:Request,context:{params:Promise<{path?:string[]}>})=>proxyAuthenticatedRequest(request,context,"work-calendar",methods,16*1024);
const proxy=(r:Request,c:Parameters<typeof forward>[1])=>r.method!=='GET'&&!hasSameOrigin(r)?Promise.resolve(Response.json({detail:'Same-origin request required'},{status:403})):forward(r,c);
export const GET=proxy;export const POST=proxy;export const PATCH=proxy;
