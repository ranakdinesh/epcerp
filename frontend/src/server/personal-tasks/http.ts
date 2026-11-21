import 'server-only';
import { AppError } from '../errors';
import { page } from './service';
export async function handleTasks(request:Request){
    const headers={'Cache-Control':'private, no-store','X-Content-Type-Options':'nosniff'};
    try{const url=new URL(request.url);if(url.pathname.replace(/^\/api\/v1/,'')!=='/my-tasks'||request.method!=='GET')return Response.json({detail:'Not found'},{status:404,headers});
        const token=/^Bearer ([^\s]+)$/i.exec(request.headers.get('authorization')??'')?.[1]??'';return Response.json(await page(token,url.searchParams),{headers});
    }catch(e){if(!(e instanceof AppError))console.error('Personal Tasks unavailable');return Response.json({detail:e instanceof AppError?e.message:'Cannot load personal tasks'},{status:e instanceof AppError?e.status:503,headers});}
}
