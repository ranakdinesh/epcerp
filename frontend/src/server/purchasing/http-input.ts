import { PurchaseError, invalid, object } from './core/validation';
export const json = (value: unknown, status = 200) => Response.json(value, { status, headers: { 'Cache-Control': 'private, no-store', 'X-Content-Type-Options': 'nosniff', ...(status === 401 ? { 'WWW-Authenticate': 'Bearer' } : {}) } });
export function query(q: URLSearchParams, keys: string[] = []) { for (const k of q.keys())
    if (!keys.includes(k) || q.getAll(k).length !== 1)
        invalid('query', keys.length ? 'Use supported single query parameters' : 'No query parameters are supported'); }
export async function bytes(r: Request, limit: number) { const reader = r.body?.getReader(); if (!reader)
    return Buffer.alloc(0); const chunks: Uint8Array[] = []; let size = 0; try {
    for (;;) {
        const { done, value } = await reader.read();
        if (done)
            break;
        size += value.length;
        if (size > limit) {
            await reader.cancel();
            throw new PurchaseError(413, 'Request body too large');
        }
        chunks.push(value);
    }
}
finally {
    reader.releaseLock();
} return Buffer.concat(chunks); }
export async function body(r: Request, fields: string[], limit = 65536) { if (r.headers.get('content-type')?.split(';')[0].trim().toLowerCase() !== 'application/json')
    throw new PurchaseError(415, 'Use application/json'); let raw: unknown; try {
    raw = JSON.parse((await bytes(r, limit)).toString('utf8'));
}
catch (e) {
    if (e instanceof PurchaseError)
        throw e;
    invalid('body', 'Expected one JSON object');
} return object(raw, fields); }
export function binary(data: Buffer, name: string, type = 'application/pdf', encrypted = false) { return new Response(new Uint8Array(data), { headers: { 'Content-Type': type, 'Content-Disposition': 'attachment; filename="' + name.replace(/["\\\r\n]/g, '_').replace(/[^\x20-\x7e]/g, '_') + '"; filename*=UTF-8\'\'' + encodeURIComponent(name), 'Cache-Control': 'private, no-store', 'X-Content-Type-Options': 'nosniff', ...(encrypted ? { 'X-Encrypted-Storage': 'MinIO' } : {}) } }); }
// Bounded, single-file multipart input. Preserve the raw filename so path components
// cannot be hidden by FormData's browser-compatible basename normalization.
export async function multipart(r: Request, reasonAllowed = false) { const type = r.headers.get('content-type') ?? '', boundary = /^multipart\/form-data\s*;\s*boundary=(?:"([^"\r\n]+)"|([^;\s]+))\s*$/i.exec(type), marker = boundary?.[1] ?? boundary?.[2]; if (!marker || marker.length > 70)
    invalid('file', 'Expected one multipart evidence file'); const data = await bytes(r, 11 * 1024 * 1024), sep = Buffer.from('--' + marker), next = Buffer.from('\r\n--' + marker); if (!data.subarray(0, sep.length).equals(sep))
    invalid('file', 'Invalid multipart evidence'); let at = sep.length, file: null | {
    name: string;
    mime: string;
    data: Buffer;
} = null, reason = ''; const names = new Set<string>(); for (let n = 0; n < 3; n++) {
    if (data.subarray(at, at + 2).toString() === '--') {
        if (!file || data.subarray(at + 2).toString().trim())
            invalid('file', 'Expected exactly one evidence file');
        return { ...file, reason };
    }
    if (data.subarray(at, at + 2).toString() !== '\r\n')
        invalid('file', 'Invalid multipart evidence');
    at += 2;
    const end = data.indexOf('\r\n\r\n', at);
    if (end < 0 || end - at > 8192)
        invalid('file', 'Invalid multipart headers');
    const headers = data.subarray(at, end).toString('utf8').split('\r\n'), h = new Map<string, string>();
    for (const line of headers) {
        const i = line.indexOf(':');
        if (i < 1 || h.has(line.slice(0, i).toLowerCase()))
            invalid('file', 'Invalid multipart headers');
        h.set(line.slice(0, i).toLowerCase(), line.slice(i + 1).trim());
    }
    const disp = h.get('content-disposition') ?? '', name = /\bname="([^"\r\n]*)"/i.exec(disp)?.[1], filename = /\bfilename="((?:[^"\\\r\n]|\\.)*)"/i.exec(disp)?.[1]?.replace(/\\(["\\])/g, '$1');
    if (!/^form-data\s*;/i.test(disp) || !name || names.has(name))
        invalid('file', 'Use one evidence file and supported fields');
    names.add(name);
    const stop = data.indexOf(next, end + 4);
    if (stop < 0)
        invalid('file', 'Invalid multipart evidence');
    const payload = data.subarray(end + 4, stop);
    if (name === 'file' && filename !== undefined) {
        if (!filename || payload.length > 10 * 1024 * 1024 || !payload.length)
            invalid('file', 'Upload nonempty evidence of at most 10 MiB');
        file = { name: filename, mime: h.get('content-type') ?? 'application/octet-stream', data: payload };
    }
    else if (name === 'reason' && reasonAllowed && filename === undefined && payload.length <= 8000)
        reason = payload.toString('utf8');
    else
        invalid('file', 'Use one evidence file and supported fields');
    at = stop + 2 + sep.length;
} invalid('file', 'Use one evidence file and supported fields'); }
