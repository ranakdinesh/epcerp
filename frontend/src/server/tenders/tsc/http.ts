import 'server-only';
import {previewImport,commitImport,uploadedDocument} from './import-service';
import { AppError } from '../../errors';
import { transaction } from '../../db';
import { authorizeTender } from '../documents';
import { evaluateTenderEligibility, getEligibilityVerdict, verifyEligibilityDecision } from './eligibility';
import { create, edit, exportSnapshot, get, history, list, options, originals, requestProposal, resume, responseRaw, isLegacy } from './service';
import { uuid } from './validation';
import { docx } from './export';
export const ownsTSCPath = (path: string) => /^\/tender-fms\/drafts\/[^/]+\/(?:tsc|flow-status|bidding-approval|prebid-queries|bid-items|vendor-quotations|bid-costing|award-project)(?:\/|$)/.test(path);
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
        if (parts[3] === 'flow-status' && method === 'GET') {
            await transaction(async (db) => authorizeTender(db, token, draft, 'read'));
            const { getTenderFlowStatus } = await import('./bidding-approval');
            return json(await getTenderFlowStatus(draft));
        }
        if (parts[3] === 'bidding-approval') {
            const { getBiddingApproval, saveBiddingApproval } = await import('./bidding-approval');
            if (method === 'GET') {
                await transaction(async (db) => authorizeTender(db, token, draft, 'read'));
                return json(await getBiddingApproval(draft));
            }
            if (method === 'POST') {
                const { actor } = await transaction(async (db) => authorizeTender(db, token, draft, 'write'));
                const b = await body(request);
                return json(await saveBiddingApproval(draft, b, actor.id), 201);
            }
        }
        if (parts[3] === 'prebid-queries') {
            const { listPrebidQueries, createPrebidQuery, updatePrebidQuery, deletePrebidQuery } = await import('./prebid-queries');
            if (method === 'GET') {
                await transaction(async (db) => authorizeTender(db, token, draft, 'read'));
                return json(await listPrebidQueries(draft));
            }
            if (method === 'POST') {
                const { actor } = await transaction(async (db) => authorizeTender(db, token, draft, 'write'));
                return json(await createPrebidQuery(draft, await body(request), actor.id), 201);
            }
            if (method === 'PATCH' && parts[4]) {
                const { actor } = await transaction(async (db) => authorizeTender(db, token, draft, 'write'));
                return json(await updatePrebidQuery(draft, parts[4], await body(request), actor.id));
            }
            if (method === 'DELETE' && parts[4]) {
                await transaction(async (db) => authorizeTender(db, token, draft, 'write'));
                return json(await deletePrebidQuery(draft, parts[4]));
            }
        }
        if (parts[3] === 'bid-items') {
            const { listBidItems, createBidItem, updateBidItem, deleteBidItem } = await import('./bid-costing');
            if (method === 'GET') {
                await transaction(async (db) => authorizeTender(db, token, draft, 'read'));
                return json(await listBidItems(draft));
            }
            if (method === 'POST') {
                await transaction(async (db) => authorizeTender(db, token, draft, 'write'));
                return json(await createBidItem(draft, await body(request)), 201);
            }
            if (method === 'PATCH' && parts[4]) {
                await transaction(async (db) => authorizeTender(db, token, draft, 'write'));
                return json(await updateBidItem(draft, parts[4], await body(request)));
            }
            if (method === 'DELETE' && parts[4]) {
                await transaction(async (db) => authorizeTender(db, token, draft, 'write'));
                return json(await deleteBidItem(draft, parts[4]));
            }
        }
        if (parts[3] === 'vendor-quotations') {
            const { listVendorQuotes, saveVendorQuote, deleteVendorQuote, selectL1VendorQuote } = await import('./bid-costing');
            if (method === 'GET') {
                await transaction(async (db) => authorizeTender(db, token, draft, 'read'));
                return json(await listVendorQuotes(draft, url.searchParams.get('item_id') || undefined));
            }
            if (method === 'POST') {
                await transaction(async (db) => authorizeTender(db, token, draft, 'write'));
                const b = await body(request);
                if (parts[4] === 'select-l1') {
                    return json(await selectL1VendorQuote(draft, b.bid_item_id, b.quote_id));
                }
                return json(await saveVendorQuote(draft, b), 201);
            }
            if (method === 'DELETE' && parts[4]) {
                await transaction(async (db) => authorizeTender(db, token, draft, 'write'));
                return json(await deleteVendorQuote(draft, parts[4]));
            }
        }
        if (parts[3] === 'bid-costing') {
            const { getBidCosting, saveBidCosting } = await import('./bid-costing');
            if (method === 'GET') {
                await transaction(async (db) => authorizeTender(db, token, draft, 'read'));
                return json(await getBidCosting(draft));
            }
            if (method === 'POST') {
                const { actor } = await transaction(async (db) => authorizeTender(db, token, draft, 'write'));
                return json(await saveBidCosting(draft, await body(request), actor.id), 200);
            }
        }
        if (parts[3] === 'award-project' && method === 'POST') {
            const { awardAndCreateProject } = await import('./bid-costing');
            return json(await awardAndCreateProject(token, draft, await body(request)), 201);
        }
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
        if (tail[0] === 'eligibility') {
            if (method === 'GET') {
                await transaction(async (db) => authorizeTender(db, token, draft, 'read'));
                const verdict = await getEligibilityVerdict(draft, job);
                return json(verdict);
            }
            if (method === 'POST' && (!tail[1] || tail[1] === '')) {
                const { actor } = await transaction(async (db) => authorizeTender(db, token, draft, 'read'));
                const verdict = await evaluateTenderEligibility(draft, job, actor.id);
                return json(verdict, 201);
            }
            if (method === 'POST' && tail[1] === 'verify') {
                const { actor } = await transaction(async (db) => authorizeTender(db, token, draft, 'write'));
                const b = await body(request);
                const verdict = await verifyEligibilityDecision(draft, job, b.decision, b.remarks, actor.id);
                return json(verdict);
            }
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
