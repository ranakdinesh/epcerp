import {proxyAuthenticatedRequest} from "@/lib/auth/backend-proxy";
import {hasSameOrigin} from "@/lib/auth/request-origin";
const methods=new Set(["GET","POST"]);
const id=/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const proxy=async(request:Request,context:{params:Promise<{path?:string[]}>})=>{
 const params=await context.params,path=params.path ?? [];
 const allowed=request.method==="GET"?(path.length===0||path.length===1&&id.test(path[0])):path.length===2&&id.test(path[0])&&["read","acknowledge"].includes(path[1]);
 if(!allowed)return Response.json({detail:"Not found"},{status:404});
 if(request.method==="POST"&&!hasSameOrigin(request))return Response.json({detail:"Same-origin request required"},{status:403});
 return proxyAuthenticatedRequest(request,{params:Promise.resolve(params)},"inbox",methods,4*1024);
};
export const GET=proxy;
export const POST=proxy;
