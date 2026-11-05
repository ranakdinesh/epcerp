import 'server-only';
import {previewImport,commitImport,uploadedDocument} from './import-service';
import { AppError } from '../../errors';
import { create, edit, exportSnapshot, get, history, list, options, originals, requestProposal, resume, responseRaw, isLegacy } from './service';
import { uuid } from './validation';
import { docx } from './export';
export const ownsTSCPath = (path: string) => /^\/tender-fms\/drafts\/[^/]+\/tsc(?:\/|$)/.test(path);
function json(value: unknown, status = 200) { return Response.json(value, { status, headers: { 'Cache-Control': 'private, no-store', 'X-Content-Type-Options': 'nosniff' } }); }
async function body(request: Request, limit=32768) { const reader = request.body?.getReader(), chunks: Uint8Array[] = []; let size = 0; if (reader)
    try {
        while (true) {
            const { value, done } = await reader.read();
            if (done)
                break;
            size += value.length;
            if (size > limit) {
                await reader.cancel();
                throw new AppError(413, 'TSC request exceeds its upload limit');
            }
            chunks.push(value);
        }
    }
    finally {
        reader.releaseLock();
    } try {
    return JSON.parse(Buffer.concat(chunks).toString());
}
catch {
    throw new AppError(422, 'Invalid TSC request');
} }
export async function handleTSC(request: Request): Promise<Response> {
    try {
        const url = new URL(request.url), path = url.pathname.replace(/^\/api\/v1/, ''), parts = path.split('/').filter(Boolean), draft = parts[2], job = parts[4], tail = parts.slice(5), method = request.method, token = /^Bearer ([^\s]+)$/i.exec(request.headers.get('authorization') ?? '')?.[1] ?? '';
        if (!uuid.test(draft ?? ''))
            throw new AppError(422, 'Invalid tender identifier');
        if (job === 'options' && method === 'GET')
            return json(await options(token, draft));
        if (!job) {
            if (method === 'GET')
                return json({ items: await list(token, draft) });
            if (method === 'POST')
                return json(await create(token, draft, request.headers.get('idempotency-key') ?? '', await body(request)), 201);
        }
        if(job==='import'&&method==='POST'&&(tail.length===0||tail.length===1&&tail[0]==='commit'))return json(tail.length?await commitImport(token,draft,request.headers.get('idempotency-key')??'',await body(request,6*1024*1024)):await previewImport(token,draft,await body(request,6*1024*1024)),tail.length?201:200);
        if(uuid.test(job??'')&&tail.length===1&&tail[0]==='uploaded-document'&&method==='GET'){const original=await uploadedDocument(token,draft,job);return new Response(new Uint8Array(original.data),{headers:{'Content-Type':'application/vnd.openxmlformats-officedocument.wordprocessingml.document','Content-Disposition':`attachment; filename="uploaded-checklist.docx"; filename*=UTF-8''${encodeURIComponent(original.name)}`,'Cache-Control':'private, no-store','X-Content-Type-Options':'nosniff'}})}
        if (!uuid.test(job ?? ''))
            return json({ detail: 'Not found' }, 404);
        if(await isLegacy(token,draft,job))return (await import('./legacy')).handleLegacy(request,draft,job,tail,token);
        if (!tail.length && method === 'GET')
            return json(await get(token, draft, job));
        if(tail[0]==='resume'&&method==='POST')return json(await resume(token,draft,job,request.headers.get('idempotency-key')??'',await body(request)),202);
        if (tail[0] === 'history' && method === 'GET')
            return json(await history(token, draft, job));
        if (tail[0] === 'answers' && tail[1]) {
            const question = decodeURIComponent(tail[1]);
            if (tail[2] === 'originals' && method === 'GET')
                return json(await originals(token, draft, job, question));
            if (tail[2] === 'research' && method === 'POST')
                return json(await requestProposal(token, draft, job, question, request.headers.get('idempotency-key') ?? '', await body(request)), 202);
            if (tail.length === 2 && method === 'PATCH')
                return json(await edit(token, draft, job, question, await body(request)));
        }
        if (tail[0] === 'responses' && uuid.test(tail[1] ?? '') && method === 'GET') {
            const raw = await responseRaw(token, draft, job, tail[1]);
            return new Response(raw, { headers: { 'Content-Type': 'application/json', 'Content-Disposition': 'attachment; filename="model-response.json"', 'Cache-Control': 'private, no-store', 'X-Content-Type-Options': 'nosniff' } });
        }
        if (tail[0] === 'document' && method === 'GET') {
            const j = await exportSnapshot(token, draft, job);
            return new Response(new Uint8Array(docx(j.snapshot, j.answers, j.cost.estimated_micro_inr, j.cost.unknown_requests, j.cost.unconfirmed_reserved_paise,j.processing.evidence)), { headers: { 'Content-Type': 'application/vnd.openxmlformats-officedocument.wordprocessingml.document', 'Content-Disposition': 'attachment; filename="TSC-draft.docx"', 'Cache-Control': 'private, no-store', 'X-Content-Type-Options': 'nosniff' } });
        }
        return json({ detail: 'Not found' }, 404);
    }
    catch (e) {
        if (e instanceof AppError)
            return json({ detail: e.message }, e.status);
        if ((e as {
            code?: string;
        }).code === '23505')
            return json({ detail: 'A TSC request is already pending; refresh its saved status' }, 409);
        console.error('TSC operation unavailable');
        return json({ detail: 'TSC service temporarily unavailable' }, 503);
    }
}
