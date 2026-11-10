import 'server-only';
import { AppError } from '../../errors';
import { body } from '../../auth/validation';
import { transaction } from '../../db';
import { access } from './access';
import * as drafts from './drafts';
import * as documents from './documents';
import * as receipts from './receipts';
import * as formats from '../formats/service';
import { word, pdf } from '../formats/document';
export const ownsIntake = (path: string) => /^\/settings\/tsc-formats(?:\/|$)/.test(path) || (/^\/tender-fms\/(?:drafts|receipts)(?:\/|$)/.test(path) && !/^\/tender-fms\/drafts\/[^/]+\/tsc(?:\/|$)/.test(path));
const headers = { 'Cache-Control': 'private, no-store', 'X-Content-Type-Options': 'nosniff' };
const json = (value: unknown, status = 200, extra: Record<string, string> = {}) => Response.json(value, { status, headers: { ...headers, ...extra } });
function receipt(out: {
    value: {
        id: unknown;
    };
    replay: boolean;
}, status: number) { return json(out.value, status, { Location: '/api/v1/tender-fms/receipts/' + out.value.id, ...(out.replay ? { 'Idempotency-Replayed': 'true' } : {}) }); }
export async function handleIntake(request: Request): Promise<Response> {
    try {
        const url = new URL(request.url), path = url.pathname.replace(/^\/api\/v1/, ''), parts = path.split('/').filter(Boolean), token = /^Bearer ([^\s]+)$/i.exec(request.headers.get('authorization') ?? '')?.[1] ?? '', method = request.method, key = request.headers.get('idempotency-key') ?? '';
        if (process.env.EPCERP_TENDER_INTAKE_OWNER === 'nextjs' && process.env.EPCERP_TSC_OWNER !== 'nextjs')
            throw new AppError(503, 'Native Tender intake requires native TSC ownership');
        const area = parts[0] === 'settings' ? 'format' : parts[1] === 'drafts' ? 'draft' : 'receipt';
        await transaction(db => access(db, token, area, method !== 'GET' && method !== 'HEAD'));
        if (parts[0] === 'settings' && parts[1] === 'tsc-formats') {
            const format = parts[2], kind = parts[3];
            if (parts.length === 2 && method === 'GET')
                return json(await formats.list(token));
            if (parts.length === 2 && method === 'POST')
                return json(await formats.save(token, undefined, await body(request, ['name', 'purpose', 'description', 'is_active', 'expected_version', 'questions'], 512 << 10)), 201);
            if (parts.length === 3 && method === 'GET')
                return json(await formats.get(token, format));
            if (parts.length === 3 && method === 'PATCH')
                return json(await formats.save(token, format, await body(request, ['name', 'purpose', 'description', 'is_active', 'expected_version', 'questions'], 512 << 10)));
            if (parts.length === 4 && method === 'GET' && ['preview.pdf', 'template.docx'].includes(kind)) {
                const saved = await formats.get(token, format, kind === 'preview.pdf'), bytes = word(saved), data = kind === 'preview.pdf' ? await pdf(bytes, request.signal) : bytes;
                await transaction(db => access(db, token, 'format'));
                return new Response(new Uint8Array(data), { headers: { ...headers, 'Content-Type': kind === 'preview.pdf' ? 'application/pdf' : 'application/vnd.openxmlformats-officedocument.wordprocessingml.document', 'Content-Disposition': `${kind === 'preview.pdf' ? 'inline' : 'attachment'}; filename="TSC-format.${kind === 'preview.pdf' ? 'pdf' : 'docx'}"` } });
            }
        }
        if (parts[0] === 'tender-fms' && parts[1] === 'drafts') {
            const draft = parts[2];
            if (parts.length === 2 && method === 'GET')
                return json(await drafts.list(token, url.searchParams));
            if (parts.length === 2 && method === 'POST')
                return json(await drafts.create(token, key, await body(request, ['name'], 8192)), 201);
            if (parts.length === 3 && method === 'GET')
                return json(await drafts.get(token, draft));
            if (parts.length === 3 && method === 'PATCH')
                return json(await drafts.update(token, draft, await body(request, ['name'], 8192)));
            if (parts.length === 3 && method === 'DELETE')
                return json(await drafts.remove(token, draft));
            if (parts.length === 4 && parts[3] === 'batches' && method === 'POST') {
                await drafts.attach(token, draft, await body(request, ['receipt_id'], 8192));
                return new Response(null, { status: 204, headers });
            }
            if (parts.length === 5 && parts[3] === 'documents' && method === 'DELETE') {
                await documents.remove(token, draft, parts[4], await body(request, ['expected_version'], 4096));
                return new Response(null, {status:204,headers});
            }
            if (parts.length === 6 && parts[3] === 'documents' && parts[5] === 'original' && method === 'GET') {
                try {
                    const original = await documents.download(token, draft, parts[4]);
                    return new Response(new Uint8Array(original.bytes), {headers:{...headers,'Content-Type':'application/octet-stream','Content-Disposition':`attachment; filename*=UTF-8''${encodeURIComponent(original.filename).replace(/['()*]/g,c=>'%'+c.charCodeAt(0).toString(16))}`}});
                } catch (error) {
                    if (!(error instanceof AppError) || error.status !== 404) throw error;
                    // Retained pre-migration reading IDs remain downloadable.
                    const bytes = await drafts.artifact(token, draft, parts[4], 'original');
                    return new Response(new Uint8Array(bytes as Buffer), {headers:{...headers,'Content-Type':'application/octet-stream','Content-Disposition':'attachment'}});
                }
            }
            if (parts.length === 6 && parts[3] === 'documents' && method === 'GET') {
                const kind = parts[5], data = await drafts.artifact(token, draft, parts[4], kind);
                return new Response(typeof data === 'string' ? data : new Uint8Array(data), { headers: { ...headers, 'Content-Type': kind === 'original' ? 'application/octet-stream' : kind === 'result' ? 'application/json' : 'text/plain; charset=utf-8', ...(kind === 'original' ? { 'Content-Disposition': 'attachment' } : {}) } });
            }
            // Local OCR retry is intentionally not part of the cloud/manual intake flow.
            if (parts.length === 6 && parts[3] === 'documents' && parts[5] === 'retry' && method === 'POST')
                throw new AppError(409, 'Local reading is retired for cloud TSC. Upload revised source documents instead.');
        }
        if (parts[0] === 'tender-fms' && parts[1] === 'receipts') {
            const target = parts[2];
            if (parts.length === 2 && method === 'POST')
                return receipt(await receipts.create(token, key, await body(request, ['files'], 32768)), 201);
            if (parts.length === 3 && method === 'GET') {
                if (url.search)
                    throw new AppError(422, 'No receipt query fields are supported');
                return json(await receipts.get(token, target));
            }
            if (parts.length === 4 && parts[3] === 'seal' && method === 'POST')
                return receipt(await receipts.seal(token, target, key, await body(request, ['expected_version'], 32768)), 202);
            if (parts.length === 5 && parts[3] === 'files') {
                if (method === 'HEAD') {
                    await receipts.preflight(token, target, parts[4]);
                    return new Response(null, { status: 204, headers });
                }
                if (method === 'GET') {
                    const result = await receipts.original(token, target, parts[4]);
                    return new Response(new Uint8Array(result.bytes), { headers: { ...headers, 'Content-Type': 'application/octet-stream', 'Content-Disposition': `attachment; filename*=UTF-8''${encodeURIComponent(result.filename).replace(/['()*]/g, c => '%' + c.charCodeAt(0).toString(16))}` } });
                }
                if (method === 'PUT') {
                    await receipts.preflight(token, target, parts[4]);
                    if (request.headers.get('content-type') !== 'application/octet-stream')
                        throw new AppError(415, 'Use application/octet-stream');
                    const version = request.headers.get('x-upload-version') ?? '';
                    if (!/^\d+$/.test(version))
                        throw new AppError(422, 'Current upload version is required');
                    return receipt(await receipts.upload(token, target, parts[4], key, Number(version), request), 200);
                }
            }
        }
        return json({ detail: 'Not found' }, 404);
    }
    catch (e) {
        if (e instanceof AppError)
            return json({ detail: e.message }, e.status, e.status === 429 ? { 'Retry-After': '60' } : {});
        console.error('Tender intake operation unavailable');
        return json({ detail: 'Tender intake service temporarily unavailable' }, 503);
    }
}
