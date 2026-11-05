import 'server-only';
import { AppError } from '../../errors';
import { body } from '../../auth/validation';
import { fields, TeamError } from './validation';
import * as service from './service';
export const ownsTeam = (path: string) => /^\/projects\/[^/]+\/responsibilities(?:\/[^/]+(?:\/history)?)?$/.test(path);
const json = (value: unknown, status = 200) => Response.json(value, { status, headers: { 'Cache-Control': 'private, no-store', 'X-Content-Type-Options': 'nosniff', ...(status === 401 ? { 'WWW-Authenticate': 'Bearer' } : {}) } });
export async function handleTeam(request: Request) {
    try {
        const parts = new URL(request.url).pathname.replace(/^\/api\/v1/, '').split('/').filter(Boolean), method = request.method;
        const token = /^Bearer ([^\s]+)$/i.exec(request.headers.get('authorization') ?? '')?.[1] ?? '';
        if (parts.length === 3 && method === 'GET') return json(await service.workspace(token, parts[1]));
        if (parts.length === 4 && parts[3] === 'options' && method === 'GET') return json(await service.options(token, parts[1]));
        if (parts.length === 5 && parts[4] === 'history' && method === 'GET') return json(await service.history(token, parts[1], parts[3]));
        if (parts.length === 3 && method === 'POST' || parts.length === 4 && method === 'PATCH') {
            const b = await body(request, fields, 32768);
            return json(await service.save(token, parts[1], parts[3], b), method === 'POST' ? 201 : 200);
        }
        return json({ detail: 'Not found' }, 404);
    } catch (error) {
        if (error instanceof AppError) return json({ detail: error instanceof TeamError && error.field ? [{ loc: ['body', error.field], msg: error.message, type: 'value_error' }] : error.message }, error.status);
        console.error('Project responsibility operation unavailable');
        return json({ detail: 'Project responsibility service temporarily unavailable' }, 503);
    }
}
