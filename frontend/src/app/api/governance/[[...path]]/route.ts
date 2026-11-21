import { hasSameOrigin } from '@/lib/auth/request-origin';
import {proxyAuthenticatedRequest} from "@/lib/auth/backend-proxy";
const methods=new Set(["GET","POST"]);
const proxy=async(request:Request,context:{params:Promise<{path?:string[]}>})=>{
 if(request.method!=='GET'&&!hasSameOrigin(request))return Response.json({detail:'Same-origin request required'},{status:403});
 const params=await context.params;
 const path=params.path ?? [];
 if(path.length>1||(path.length===1&&(path[0]!=="options"||request.method!=="GET")))return Response.json({detail:"Not found"},{status:404});
 return proxyAuthenticatedRequest(request,{params:Promise.resolve(params)},"administration/director",methods,16*1024);
};
export const GET=proxy;
export const POST=proxy;
