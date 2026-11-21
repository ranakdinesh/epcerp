import { proxyAuthenticatedRequest } from '@/lib/auth/backend-proxy';
import { hasSameOrigin } from '@/lib/auth/request-origin';
const methods=new Set(['GET','POST','DELETE']);
async function proxy(r:Request,c:{params:Promise<{path?:string[]}>}){if(r.method!=='GET'&&!hasSameOrigin(r))return Response.json({detail:'Same-origin request required'},{status:403});const p=(await c.params).path??[];if(p.some(k=>!k||/[\\/]/.test(k)||k==='.'||k==='..'))return Response.json({detail:'Not found'},{status:404});return proxyAuthenticatedRequest(r,c,'documents',methods,50*1048576+65536,r.method==='POST'?'HEAD':false);}
export const GET=proxy;export const POST=proxy;export const DELETE=proxy;
