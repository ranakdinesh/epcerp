import 'server-only';
import { AppError } from '../errors';
import { body } from '../auth/validation';
import { InboxError, invalid } from './validation';
import * as inbox from './service';
export const ownsInbox = (path: string) => /^\/inbox(?:\/|$)/.test(path);
const json = (value: unknown, status = 200) => Response.json(value,{status,headers:{'Cache-Control':'private, no-store','X-Content-Type-Options':'nosniff',...(status === 401 ? {'WWW-Authenticate':'Bearer'} : {})}});
export async function handleInbox(request: Request): Promise<Response> {
    try {
        const url = new URL(request.url), parts = url.pathname.replace(/^\/api\/v1/,'').split('/').filter(Boolean), token = /^Bearer ([^\s]+)$/i.exec(request.headers.get('authorization') ?? '')?.[1] ?? '';
        await inbox.authorize(token);
        if (parts.length === 1 && request.method === 'GET') return json(await inbox.list(token,url.searchParams));
        if (url.searchParams.size) invalid('query','Unsupported notice query');
        if (parts.length === 2 && request.method === 'GET') return json(await inbox.read(token,parts[1]));
        if (parts.length === 3 && ['read','acknowledge'].includes(parts[2]) && request.method === 'POST') return json(await inbox.receipt(token,parts[1],await body(request,['version'],4096),parts[2] as 'read' | 'acknowledge'));
        return json({detail:'Not found'},404);
    } catch (error) {
        if (error instanceof AppError) return json({detail:error instanceof InboxError && error.field ? [{loc:['body',error.field],msg:error.message,type:'value_error'}] : error.message},error.status);
        console.error('Inbox operation unavailable');
        return json({detail:'Inbox service temporarily unavailable'},503);
    }
}
