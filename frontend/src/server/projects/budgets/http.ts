import { AppError } from '../../errors';
import { bytes, json } from '../../purchasing/http-input';
import { BudgetError, invalid } from './validation';
import { parse } from './input';
import * as service from './service';
export const ownsBudget = (path: string) => /^\/projects\/[^/]+\/budget$/.test(path);
export async function handleBudget(request: Request) {
    try {
        const parts = new URL(request.url).pathname.replace(/^\/api\/v1/, '').split('/').filter(Boolean), token = /^Bearer ([^\s]+)$/i.exec(request.headers.get('authorization') ?? '')?.[1] ?? '';
        if (request.method === 'GET')
            return json(await service.workspace(token, parts[1]));
        if (request.method === 'POST') {
            let raw: unknown;
            try {
                raw = parse((await bytes(request, 512 * 1024)).toString('utf8'));
            }
            catch (e) {
                if (e instanceof AppError)
                    throw e;
                invalid('', 'Expected one JSON object');
            }
            return json(await service.execute(token, parts[1], raw));
        }
        return json({ detail: 'Not found' }, 404);
    }
    catch (e) {
        if (e instanceof AppError)
            return json({ detail: e instanceof BudgetError && e.field ? [{ loc: ['body', ...e.field.split('.')], msg: e.message, type: 'value_error' }] : e.message }, e.status);
        console.error('Project budget operation unavailable');
        return json({ detail: 'Internal server error' }, 500);
    }
}
