import 'server-only';
import { randomUUID } from 'node:crypto';
import { AppError } from '../../errors';
import { transaction } from '../../db';
import { body } from '../../auth/validation';
import { access, capturePermissions } from './access';
import * as register from './service';
import * as ownership from './ownership';
export const ownsRegister = (path: string) => /^\/tender-fms\/(?:records(?:\/|$)|assignment-options$|[^/]+\/ownership$)/.test(path);
const headers = { 'Cache-Control':'private, no-store','X-Content-Type-Options':'nosniff' };
const json = (value: unknown, status = 200, extra = {}) => Response.json(value,{status,headers:{...headers,...extra}});
export async function handleRegister(request: Request): Promise<Response> {
    try {
        const url = new URL(request.url), parts = url.pathname.replace(/^\/api\/v1/,'').split('/').filter(Boolean), token = /^Bearer ([^\s]+)$/i.exec(request.headers.get('authorization') ?? '')?.[1] ?? '', method = request.method, key = request.headers.get('idempotency-key') ?? '';
        const legacy = parts[2] === 'ownership';
        // Authenticate before body/query validation; each command rechecks within its mutation transaction.
        await transaction(db => access(db,token,legacy ? ['tenders.register.assign'] : method === 'POST' && parts.length === 2 || parts[1] === 'assignment-options' ? capturePermissions : undefined));
        if (parts[1] === 'assignment-options' && parts.length === 2 && method === 'GET') return json(await register.options(token,url.searchParams));
        if (legacy && parts.length === 3) {
            if (method === 'GET') return json(await ownership.options(token,parts[1]));
            if (method === 'POST') return json(await ownership.handover(token,parts[1],await body(request,['owner_id','version','reason'],8192)));
        }
        if (parts[1] === 'records') {
            if (parts.length === 2 && method === 'GET') return json(await register.list(token,url.searchParams));
            if (parts.length === 3 && method === 'GET') return json(await register.get(token,parts[2]));
            let result;
            if (parts.length === 2 && method === 'POST') result = await register.capture(token,key,await body(request,['name','receipt_id','expected_receipt_version','manager_id','external_reference','source_url'],32768));
            if (parts.length === 3 && method === 'PATCH') { await register.get(token,parts[2]); result = await register.patch(token,parts[2],key,await body(request,['name','expected_version','external_reference','source_url'],32768)); }
            if (parts.length === 4 && parts[3] === 'manager-assignments' && method === 'POST') { await register.get(token,parts[2]); result = await register.changeManager(token,parts[2],key,await body(request,['expected_version','expected_assignment_version','manager_id','reason'],32768)); }
            if (result) return json(result.value,parts.length === 2 ? 202 : 200,{Location:'/api/v1/tender-fms/records/'+result.value.id,...(result.replay ? {'Idempotency-Replayed':'true'} : {})});
        }
        return json({detail:'Not found'},404);
    } catch (error) {
        const status = error instanceof AppError ? error.status : 503;
        if (!(error instanceof AppError)) console.error('Tender register operation unavailable');
        return json({code:({401:'unauthenticated',403:'forbidden',404:'not_found',409:'stale_version',422:'validation_failed',429:'rate_limited'} as Record<number,string>)[status] ?? 'dependency_unavailable',detail:error instanceof AppError ? error.message : 'Cannot complete Tender register operation; retry with the same Idempotency-Key',request_id:randomUUID(),retryable:status === 503 || status === 429},status,status === 429 ? {'Retry-After':'60'} : {});
    }
}
