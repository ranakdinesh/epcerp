import 'server-only';
import { AppError } from '../errors';
import { body } from '../auth/validation';
import { captureFields,ProjectError } from './validation';
import * as projects from './service';
export const ownsProjects=(path:string)=>/^\/projects(?:\/[^/]+(?:\/(?:manager|manager-history|metadata-history|metadata-options|schedule-preview))?)?$/.test(path);
const json=(v:unknown,status=200)=>Response.json(v,{status,headers:{'Cache-Control':'private, no-store','X-Content-Type-Options':'nosniff',...(status===401?{'WWW-Authenticate':'Bearer'}:{})}});
export async function handleProjects(request:Request){try{
    const url=new URL(request.url),parts=url.pathname.replace(/^\/api\/v1/,'').split('/').filter(Boolean),method=request.method,token=/^Bearer ([^\s]+)$/i.exec(request.headers.get('authorization')??'')?.[1]??'';
    if(['POST','PATCH'].includes(method)&&request.headers.get('content-type')?.split(';')[0].trim().toLowerCase()!=='application/json')throw new ProjectError(415,'Use application/json');
    if(parts.length===1&&method==='GET')return json(await projects.list(token,url.searchParams));
    if(parts.length===1&&method==='POST')return json(await projects.create(token,await body(request,['name','client_id','project_manager_id','procurement_executive_id',...captureFields],65536)),201);
    if(parts.length===2&&parts[1]==='assignment-options'&&method==='GET')return json(await projects.options(token));
    if(parts.length===2&&parts[1]==='schedule-preview'&&method==='POST')return json(await projects.preview(token,await body(request,[...captureFields],65536)));
    if(parts.length===2&&method==='GET')return json(await projects.get(token,parts[1]));
    if(parts.length===2&&method==='PATCH')return json(await projects.update(token,parts[1],await body(request,['name','client_id','reason','expected_version',...captureFields],65536)));
    if(parts.length===3){const id=parts[1],kind=parts[2];
        if(kind==='manager'&&method==='PATCH')return json(await projects.transfer(token,id,await body(request,['project_manager_id','reason','expected_version'],65536)));
        if(kind==='manager-history'&&method==='GET')return json(await projects.managerHistory(token,id));
        if(kind==='metadata-history'&&method==='GET')return json(await projects.metadataHistory(token,id));
        if(kind==='metadata-options'&&method==='GET')return json(await projects.metadataOptions(token,id));
        if(kind==='schedule-preview'&&method==='POST')return json(await projects.preview(token,await body(request,[...captureFields],65536),id));
    }
    return json({detail:'Not found'},404);
}catch(e){if(e instanceof AppError)return json({detail:e instanceof ProjectError&&e.field?[{type:'value_error',loc:['body',e.field],msg:e.message}]:e.message},e.status);console.error('Project operation unavailable');return json({detail:'Project service temporarily unavailable'},503);}}
