import { AppError } from '../../errors';
import { trim } from '../validation';
export class BudgetError extends AppError {
    constructor(status: number, message: string, public field = '') {
        super(status, message);
    }
}
export function invalid(field: string, message: string): never {
    throw new BudgetError(422, message, field);
}
export const stale = () => new BudgetError(409, 'Budget, purchase quantity or BO version changed; reload before saving');
export const forbidden = () => new BudgetError(403, 'Budget permission or project scope denied');
export const missing = () => new BudgetError(404, 'Project budget not found');
export function id(v: unknown) {
    if (typeof v !== 'string' || !/^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}(?![\s\S])/i.test(v))
        invalid('id', 'Use a valid budget record identifier');
    return v.toLowerCase();
}
export function text(v: unknown) {
    if (typeof v !== 'string')
        invalid('', 'Invalid budget input');
    return v;
}
export function reason(v: unknown) {
    const s = trim(text(v));
    if (!s || [...s].length > 2000)
        invalid('reason', 'Record a budget change reason of at most 2000 characters');
    return s;
}
export function version(v: unknown, min = 0) {
    if (typeof v !== 'number' || !Number.isInteger(v) || v < min || v > 2147483647)
        invalid('version', 'Use the current budget version');
    return v;
}
export function object(v: unknown, allowed: string[], required: string[] = [], nullable: string[] = []): Record<string, unknown> {
    if (!v || typeof v !== 'object' || Array.isArray(v))
        invalid('', 'Expected one JSON object');
    const o = v as Record<string, unknown>;
    for (const key of required)
        if (!(key in o))
            invalid(key, 'This field is required');
    for (const key of Object.keys(o)) {
        if (!allowed.includes(key))
            invalid('', 'Invalid budget input');
        if (o[key] === null && !nullable.includes(key))
            invalid(key, 'This field must not be null');
    }
    return o;
}
export function bool(v: unknown) {
    if (typeof v !== 'boolean')
        invalid('', 'Invalid budget input');
    return v;
}
export function decimal(v: unknown) {
    if (typeof v === 'number')
        v = String(v);
    if (typeof v !== 'string' || v.length > 62 || !/^\d+(?:\.\d+)?(?![\s\S])/.test(v))
        invalid('', 'Invalid budget input');
    return v;
}
