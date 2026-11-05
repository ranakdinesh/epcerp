import 'server-only';
import { AppError } from '../errors';
import { PurchaseError, invalid, version } from './core/validation';
import { json, body, query, binary, multipart } from './http-input';
import * as materials from './materials/service';
import * as bo from './bo/service';
import * as groups from './groups/service';
import * as indentDocuments from './indent-documents/service';
import * as indents from './indents/service';
import * as holds from './holds/service';
import * as quotations from './quotations/service';
import * as approvals from './approvals/service';
import * as awards from './awards/service';
import * as workflow from './workflow/service';
import * as sourcing from './sourcing/service';
import * as technical from './technical/service';
import * as orders from './orders/service';
import * as acceptance from './acceptance/service';
import * as advances from './advances/service';
import * as inspections from './inspections/service';
import * as inspectionReports from './inspection-reports/service';
import * as quality from './quality/service';
import * as clientClearance from './client-clearance/service';
import * as dispatch from './dispatch/service';
import * as siteReceipts from './site-receipts/service';
import * as grns from './grns/service';
import * as engineerConfirmations from './engineer-confirmations/service';
import * as accountsAcceptance from './accounts-acceptance/service';
import * as loiDocuments from './loi-documents/service';
import * as clientRules from './client-rules/service';
import * as workspace from './workspace/service';
import * as ownership from './workspace/ownership';
import * as boPDF from './bo/pdf';
import * as approvalPDF from './approvals/pdf';
import * as approvalReads from './approvals/reads';
import * as quotationFiles from './quotations/attachments';
import * as technicalFiles from './technical/evidence';
import * as orderFiles from './orders/evidence';
import * as progress from './progress/service';
export { ownsPurchasing } from './paths';
const stageServices = { sourcing, technical, 'purchase-orders': orders, acceptance, advances, inspection: inspections, 'inspection-report': inspectionReports, quality, 'client-clearance': clientClearance, dispatch, 'site-receipt': siteReceipts, grn: grns, 'engineer-confirmation': engineerConfirmations, 'accounts-acceptance': accountsAcceptance };
const stageFields: Record<string, string[]> = { sourcing: ['version', 'action', 'reason', 'approver_id', 'offer_id', 'supplier_id', 'due_date', 'rfq_terms', 'sent_date', 'channel', 'dispatch_reference', 'quote_reference', 'evidence_reference', 'valid_until', 'pricing_mode', 'unit_price', 'line_total', 'discount', 'tax', 'freight', 'other', 'delivery_days', 'payment_terms', 'warranty', 'compliant', 'compliance_notes', 'approve', 'loi_terms'], technical: ['version', 'action', 'reason', 'document', 'submission', 'approve', 'remarks', 'approval', 'qap_approved', 'conditions_closed'], 'purchase-orders': ['version', 'action', 'reason', 'details', 'submission', 'client_decision'], acceptance: ['version', 'action', 'reason', 'response'], advances: ['version', 'action', 'reason', 'details', 'payment'], inspection: ['version', 'action', 'reason', 'inspection'], 'inspection-report': ['version', 'action', 'reason', 'report'], quality: ['version', 'action', 'reason', 'certificate_code', 'details'], 'client-clearance': ['version', 'action', 'reason', 'clearance'], dispatch: ['version', 'action', 'reason', 'dispatch'], 'site-receipt': ['version', 'action', 'reason', 'receipt'], grn: ['version', 'action', 'reason', 'grn'], 'engineer-confirmation': ['version', 'action', 'reason', 'confirmation'], 'accounts-acceptance': ['version', 'action', 'reason', 'acceptance'] };
export async function handlePurchasing(request: Request) {
    try {
        const url = new URL(request.url), parts = url.pathname.replace(/^\/api\/v1/, '').split('/').filter(Boolean), m = request.method, q = url.searchParams, token = /^Bearer ([^\s]+)$/i.exec(request.headers.get('authorization') ?? '')?.[1] ?? '', n = parts.length;
        if (parts[0] === 'purchase-fms') {
            if (n === 1 && m === 'GET')
                return json(await workspace.dashboard(token, q));
            query(q);
            if (parts[1] === 'projects') {
                const p = parts[2];
                if (n === 3 && m === 'GET')
                    return json(await workspace.context(token, p));
                if (n === 4 && parts[3] === 'assignment-options' && m === 'GET')
                    return json(await ownership.options(token, p));
                if (n === 4 && parts[3] === 'owner' && m === 'PATCH')
                    return json(await ownership.assign(token, p, await body(request, ['purchase_manager_id', 'expected_version', 'reason'], 16384)));
            }
        }
        if (n === 1 && parts[0] === 'quotation-approvals' && m === 'GET') {
            query(q, ['offset']);
            const offset = q.get('offset') ?? '0';
            if (!/^\d+$/.test(offset))
                invalid('offset', 'Offset must be between 0 and 1000000');
            return json(await approvalReads.queue(token, Number(offset)));
        }
        if (n === 1 && parts[0] === 'purchase-advances' && m === 'GET') {
            query(q);
            return json(await advances.queue(token));
        }
        if (n === 1 && parts[0] === 'indent-dispatch-notifications' && m === 'GET')
            return json(await indentDocuments.notifications(token));
        if (parts[0] === 'clients' && parts[2] === 'purchase-rules') {
            const c = parts[1], id = parts[3];
            if (n === 3 && m === 'GET')
                return json(await clientRules.list(token, c));
            if (n === 5 && parts[4] === 'history' && m === 'GET')
                return json(await clientRules.history(token, c, id));
            if (n === 3 && m === 'POST' || n === 4 && m === 'PATCH')
                return json(await clientRules.save(token, c, id, await body(request, ['data', 'version', 'is_active', 'reason'])), m === 'POST' ? 201 : 200);
        }
        if (parts[0] !== 'projects')
            return json({ detail: 'Not found' }, 404);
        const p = parts[1], area = parts[2], id = parts[3], sub = parts[4];
        if (area === 'quotation-status' && n === 3 && m === 'GET')
            return json(await approvalReads.projectStatus(token, p));
        if (area === 'quotation-bo' && n === 3 && m === 'GET')
            return json(await quotations.choices(token, p, q.get('section') || 'receipt'));
        if (area === 'purchase-progress' && n === 3 && m === 'GET') {
            query(q);
            return json(await progress.report(token, p));
        }
        if (area === 'purchase-awards' && n === 3) {
            query(q);
            if (m === 'GET')
                return json(await awards.workspace(token, p));
            if (m === 'POST')
                return json(await awards.execute(token, p, await body(request, ['attempt_id', 'doer_id', 'resolver_id', 'planned_date', 'reason'])));
        }
        if (area === 'vendor-lois') {
            query(q);
            if (n === 4 && m === 'GET')
                return json(await loiDocuments.workspace(token, p, id));
            if (n === 4 && m === 'POST')
                return json(await loiDocuments.generate(token, p, id, await body(request, ['reference', 'date', 'format', 'notes'])));
            if (sub === 'files' && n === 6 && m === 'GET') {
                const f = await loiDocuments.download(token, p, id, parts[5]);
                return binary(f.data, f.name, 'application/pdf', true);
            }
            if (sub === 'files' && n === 7 && parts[6] === 'complete-review' && m === 'POST')
                return json(await loiDocuments.completeReview(token, p, id, parts[5], await body(request, ['details_correct', 'prices_correct', 'format_correct'])));
        }
        if (area === 'purchase-bo') {
            if (n === 3 && m === 'GET')
                return json(await bo.list(token, p));
            if (n === 4 && id === 'pdf' && m === 'GET') {
                const f = await boPDF.print(token, p);
                return binary(f.data, f.code + '-items-to-purchase.pdf');
            }
            if (n === 4 && ['register-complete', 'register-reopen'].includes(id) && m === 'POST')
                return json(await bo.register(token, p, id === 'register-complete' ? 'complete' : 'reopen', await body(request, ['records', 'reason'], 512 * 1024)));
            if (n === 4 && m === 'POST')
                return json(await bo.save(token, p, id, await body(request, ['name', 'version', 'reason', 'lines'], 512 * 1024)));
            if (n === 5 && sub === 'history' && m === 'GET')
                return json(await bo.history(token, p, id));
            if (n === 5 && ['complete', 'reopen'].includes(sub) && m === 'POST')
                return json(await bo.change(token, p, id, sub, await body(request, ['version', 'reason'])));
            if (n === 5 && sub === 'indent-groups') {
                if (m === 'GET')
                    return json(await groups.workspace(token, p, id));
                if (m === 'POST')
                    return json(await groups.execute(token, p, id, await body(request, ['version', 'bo_version', 'action', 'reason', 'groups', 'group_id', 'line_id', 'line_ids', 'selected_line_ids', 'vendor_id', 'vendor_ids', 'sent_date', 'channel', 'dispatch_reference'], 1024 * 1024)));
            }
            if (n === 5 && sub === 'indent-documents') {
                if (m === 'GET')
                    return json(await indentDocuments.workspace(token, p, id));
                if (m === 'POST')
                    return json(await indentDocuments.execute(token, p, id, await body(request, ['action', 'group_id', 'vendor_id', 'document_id', 'version', 'group_version', 'bo_version', 'sent_date', 'channel', 'dispatch_reference'])));
            }
            if (sub === 'group-quotations') {
                if (n === 5 && m === 'GET')
                    return json(await quotations.workspace(token, p, id, q.get('section') || 'receipt'));
                if (n === 5 && m === 'POST')
                    return json(await quotations.execute(token, p, id, await body(request, ['version', 'reason', 'quote'], 512 * 1024)));
                if (n === 7 && parts[6] === 'attachments' && m === 'POST') {
                    await quotations.uploadAuthority(token, p, id, parts[5]);
                    const f = await multipart(request);
                    return json(await quotationFiles.upload(token, p, id, parts[5], f.name, f.mime, f.data), 201);
                }
                if (n === 9 && parts[6] === 'attachments' && parts[8] === 'download' && m === 'GET') {
                    const f = await quotationFiles.download(token, p, id, parts[5], parts[7]);
                    return binary(f.data, f.file.filename, f.file.content_type);
                }
            }
            if (sub === 'quotation-approvals') {
                if (n === 5 && m === 'GET')
                    return json(await approvals.workspace(token, p, id));
                if (n === 6 && parts[5] === 'budget-preview' && m === 'GET') {
                    query(q, ['quote_key']);
                    return json(await approvalReads.preview(token, p, id, q.get('quote_key') ?? ''));
                }
                if (n === 6 && parts[5] === 'print' && ['GET', 'POST'].includes(m)) {
                    query(q, ['group_id', 'attempt_id', 'quote_keys']);
                    if (!q.get('group_id') || q.has('attempt_id') && q.has('quote_keys'))
                        invalid('group_id', 'Choose a group and optional frozen submission');
                    const input = m === 'POST' ? await body(request, ['proposals', 'finance_notes'], 1024 * 1024) : {}, proposals = input.proposals ?? [], notes = input.finance_notes ?? {};
                    if (!Array.isArray(proposals) || !notes || typeof notes !== 'object' || Array.isArray(notes))
                        invalid('proposals', 'Print one complete draft shortlist');
                    if (m === 'POST' && (!proposals.length && !Object.keys(notes).length || proposals.length && q.has('attempt_id') || Object.keys(notes).length && (proposals.length || !q.has('attempt_id'))))
                        invalid('proposals', 'Print one complete draft shortlist');
                    return binary(await approvalPDF.print(token, p, id, q.get('group_id')!, q.get('attempt_id') ?? '', proposals as Parameters<typeof approvalPDF.print>[5], q.get('quote_keys')?.split(',') ?? [], notes as Record<string, string>), 'quotation-comparison.pdf');
                }
                if (n === 6 && parts[5] === 'draft' && m === 'POST')
                    return json(await approvals.saveDraft(token, p, id, await body(request, ['group_id', 'version', 'proposals', 'comment'])));
                if (n === 6 && ['submit', 'review'].includes(parts[5]) && m === 'POST')
                    return json(await approvals.execute(token, p, id, parts[5], await body(request, ['proposals', 'selected_quote_key', 'return_action', 'version', 'reason', 'quote_key', 'attempt_id', 'decisions'], parts[5] === 'review' ? 1024 * 1024 : 65536)));
            }
        }
        if (area === 'items') {
            if (n === 3 && m === 'GET')
                return json(await materials.list(token, p));
            if (n === 4 && id === 'catalogue' && m === 'GET')
                return json(await materials.catalogue(token, p));
            if (n === 5 && sub === 'history' && m === 'GET')
                return json(await materials.history(token, p, id));
            if (n === 3 && m === 'POST' || n === 4 && m === 'PATCH')
                return json(await materials.save(token, p, id, await body(request, ['item_id', 'quantity', 'required_date', 'allocations', 'alternatives', 'is_active', 'version', 'reason'])), m === 'POST' ? 201 : 200);
        }
        if (area === 'purchase-indents') {
            if (n === 3 && m === 'GET')
                return json(await indents.list(token, p));
            if (n === 4 && id === 'workspace' && m === 'GET')
                return json(await indents.workspace(token, p));
            if (n === 5 && sub === 'history' && m === 'GET')
                return json(await indents.history(token, p, id));
            if (n === 3 && m === 'POST' || n === 4 && m === 'PATCH')
                return json(await indents.save(token, p, id, await body(request, ['data', 'version', 'reason'], 512 * 1024)), m === 'POST' ? 201 : 200);
            if (n === 5 && sub === 'actions' && m === 'POST')
                return json(await indents.transition(token, p, id, await body(request, ['version', 'action', 'reason', 'line_ids', 'evidence'], 512 * 1024)));
        }
        if (area === 'purchase-holds') {
            if (n === 3 && m === 'GET')
                return json(await holds.list(token, p));
            if (n === 3 && m === 'POST')
                return json(await holds.create(token, p, await body(request, ['scope', 'category_code', 'demand_ids', 'reason', 'evidence', 'started_at', 'expected_release_at', 'doer_id', 'resolver_id'])), 201);
            if (n === 5 && sub === 'history' && m === 'GET')
                return json(await holds.history(token, p, id));
            if (n === 5 && sub === 'release' && m === 'PATCH')
                return json(await holds.release(token, p, id, await body(request, ['version', 'reason', 'evidence'])));
        }
        if (area === 'purchase-workflow') {
            if (n === 3 && m === 'GET') {
                query(q, ['page', 'page_size', 'search', 'sort', 'direction']);
                const input: Parameters<typeof workflow.listing>[2] = {};
                for (const k of ['page', 'page_size'] as const)
                    if (q.has(k)) {
                        const raw = q.get(k)!;
                        if (!/^[+-]?\d+$/.test(raw))
                            invalid('query', 'Use integer pagination');
                        input[k] = Number(raw);
                    }
                for (const k of ['search', 'sort', 'direction'] as const)
                    if (q.has(k))
                        input[k] = q.get(k)!;
                return json(await workflow.listing(token, p, input));
            }
            query(q);
            if (n === 3 && m === 'POST')
                return json(await workflow.create(token, p, await body(request, ['indent_id', 'demand_id', 'reason'])), 201);
            if (n === 4 && m === 'GET')
                return json(await workflow.workspace(token, p, id));
            if (n === 5 && sub === 'assignments' && m === 'POST')
                return json(await workflow.assign(token, p, id, await body(request, ['version', 'rows', 'reason'])));
            if (n === 6 && sub === 'steps' && m === 'POST') {
                if (!/^\d+$/.test(parts[5]))
                    invalid('step', 'Unknown purchase step');
                return json(await workflow.command(token, p, id, Number(parts[5]), await body(request, ['version', 'action', 'notes', 'reason', 'resolution_due'])));
            }
            const service = stageServices[sub as keyof typeof stageServices];
            if (service && n === 5) {
                if (m === 'GET')
                    return json(await service.workspace(token, p, id));
                if (m === 'POST')
                    return json(await service.execute(token, p, id, await body(request, stageFields[sub])));
            }
            if (sub === 'purchase-orders') {
                if (n === 6 && parts[5] === 'document' && m === 'GET')
                    return json(await orders.document(token, p, id));
                if (n === 7 && parts[5] === 'files' && m === 'GET') {
                    const f = await orderFiles.download(token, p, id, parts[6]);
                    return binary(f.data, f.file.name, f.file.media_type, true);
                }
                if (n === 7 && parts[5] === 'uploads' && ['GET', 'POST'].includes(m)) {
                    if (!/^[1-9]\d*$/.test(parts[6]))
                        invalid('version', 'Use the saved order version');
                    const v = version(Number(parts[6]));
                    await orderFiles.authorizeUpload(token, p, id, v);
                    if (m === 'GET')
                        return new Response(null, { status: 204, headers: { 'Cache-Control': 'no-store' } });
                    const f = await multipart(request);
                    return json(await orderFiles.upload(token, p, id, v, f.name, f.mime, f.data), 201);
                }
            }
            if (sub === 'technical') {
                if (n === 7 && parts[5] === 'files' && m === 'GET') {
                    const f = await technicalFiles.download(token, p, id, parts[6]);
                    return binary(f.data, f.file.name, f.file.media_type, true);
                }
                if (n === 8 && parts[5] === 'uploads' && ['GET', 'POST'].includes(m)) {
                    if (!/^[1-9]\d*$/.test(parts[7]))
                        invalid('version', 'Use the saved technical version');
                    const input = { purpose: parts[6], version: version(Number(parts[7])) };
                    await technicalFiles.authorizeUpload(token, p, id, input);
                    if (m === 'GET')
                        return new Response(null, { status: 204, headers: { 'Cache-Control': 'no-store' } });
                    const f = await multipart(request, true);
                    return json(await technicalFiles.upload(token, p, id, { ...input, reason: f.reason, name: f.name, media_type: f.mime, data: f.data }), 201);
                }
            }
        }
        return json({ detail: 'Not found' }, 404);
    }
    catch (e) {
        if (e instanceof AppError)
            return json({ detail: e instanceof PurchaseError && e.field ? [{ loc: ['body', e.field], msg: e.message, type: 'value_error' }] : e.message }, e.status);
        console.error('Purchasing operation failed');
        return json({ detail: 'Internal server error' }, 500);
    }
}
