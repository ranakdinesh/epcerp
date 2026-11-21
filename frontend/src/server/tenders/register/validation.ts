import { AppError } from '../../errors';
import { id, integer, object } from '../tsc/validation';
export { id, object };
export const invalid = (message: string): never => { throw new AppError(422, message); };
export function text(value: unknown, min: number, max: number, trim = false) {
    if (typeof value !== 'string') return invalid('Text is required');
    const s = trim ? value.trim() : value;
    if ([...s].length < min || [...s].length > max || /[\uD800-\uDFFF]/u.test(s)) return invalid('Text is outside supported bounds');
    return s;
}
export function name(value: unknown) { const s = text(value, 2, 255, true); if (/\p{Cc}/u.test(s)) return invalid('Tender name cannot contain control characters'); return s; }
export function optional(value: unknown, url = false) {
    if (value == null) return null;
    const s = text(value, url ? 0 : 1, url ? 2048 : 255);
    if (/[\0\r\n]/.test(s)) return invalid('Invalid reference or URL');
    if (url) {
        if (Buffer.byteLength(s) > 2048 || !/^https?:\/\//.test(s)) return invalid('Use an HTTP or HTTPS source URL without credentials');
        try { const u = new URL(s); if (!u.hostname || u.username || u.password) return invalid('Use an HTTP or HTTPS source URL without credentials'); } catch { return invalid('Invalid source URL'); }
    }
    return s;
}
export const version = (value: unknown) => integer(value, 1, Number.MAX_SAFE_INTEGER - 1);
export const reason = (value: unknown) => text(value, 10, 1000, true);
export function query(q: URLSearchParams, options = false) {
    const allowed = options ? ['page','page_size'] : ['page','page_size','search','sort','direction'];
    for (const key of q.keys()) if (!allowed.includes(key) || q.getAll(key).length !== 1) invalid('Unsupported or repeated register query field');
    const number = (key: string, fallback: number) => { const v = q.get(key); if (v === null) return fallback; if (!/^[+-]?\d+$/.test(v)) return invalid('Invalid pagination'); return Number(v); };
    const page = integer(number('page', 1), 1, 100000), size = number('page_size', 20);
    if (![20,50,100].includes(size)) invalid('Invalid page size');
    const search = text(q.get('search') ?? '', 0, 200), sort = q.get('sort') || 'created', direction = q.get('direction') || 'desc';
    if (/[\0\r\n]/.test(search) || !['name','created'].includes(sort) || !['asc','desc'].includes(direction)) invalid('Invalid search or sort');
    return { page, size, search, sort, direction };
}
