import 'server-only';
import { amount } from '../core/rational';
import { transaction, type DB } from '../../db';
import { actor, project, scope, type Actor } from '../core/access';
import { workspace as purchaseWorkspace } from '../workspace/repository';
import { list as boLists, mergedListAliases } from '../bo/repository';
import { pending } from '../groups/repository';
import { invalid, uuid, version } from '../core/validation';
import { now } from '../core/time';
import { indiaDate } from '../../personal-tasks/validation';
import { receiptState, source } from './source';
import { compare, calculate, quoteExclusions } from './calculation';
import { input, normalizeMoney } from './validation';
import type { QuoteRecord, Quote, Attachment } from './types';
import * as repo from './repository';
import { itemListMode } from '../application-settings';
export async function access(db: DB, token: string, target: string, permissions: string[], write: boolean) { const a = await actor(db, token, permissions), p = await project(db, target, true), w = await purchaseWorkspace(db, p.id), result = await scope(db, a, p, w, permissions[0], write); return { a, p, write: result.can_write }; }
function freezeBlockers(current: Awaited<ReturnType<typeof source>>, record: QuoteRecord | null, today: string) {
    const quotes = new Map((record?.is_active ? record.quotes : []).map(quote => [quote.document_id, quote]));
    return current.documents.flatMap(document => {
        const quote = quotes.get(document.id);
        if (!document.data.is_current || !quote) return [];
        const exclusions = quoteExclusions(quote, document, today, current.currency);
        return exclusions.length ? [{
            document_id: document.id, group_name: document.data.group_name, vendor: document.data.vendor.name, exclusions,
        }] : [];
    });
}
async function workspaceBound(db: DB, a: Actor, project: string, bo: string, write: boolean) {
    const s = await source(db, project, bo), record = await repo.get(db, project, bo), revisions = await repo.revisions(db, project, bo), canRead = a.allows('projects.group_quotations.read'), canCompare = a.allows('projects.group_comparisons.read'), attachments: Attachment[] = [];
    if (canRead)
        for (const d of s.documents)
            attachments.push(...await repo.attachments(db, d.id));
    const receipt = receiptState(s.documents, record, s.expectedReceipts);
    const today = indiaDate(new Date());
    const blockers = canRead ? freezeBlockers(s, record, today) : [];
    return { dispatch_complete: s.dispatchComplete, frozen: canRead && receipt.frozen, ready_to_freeze: canRead && receipt.ready && blockers.length === 0, freeze_blockers: blockers, tax_rate: '18.00', record: canRead ? record : null, documents: s.documents, currency: s.currency, can_write: write && s.available && canRead && a.allows('projects.group_quotations.write'), history: canRead ? await repo.history(db, project, bo) : [], comparisons: canCompare ? compare(record?.quotes ?? [], s.documents, today, s.currency) : [], comparison_revisions: canCompare ? compare(revisions, s.documents, today, s.currency) : [], revisions: canRead ? revisions : [], attachments };
}
function sectionPermissions(section: string) { if (section === 'receipt')
    return ['projects.group_quotations.read', 'projects.group_quotations.view']; if (section === 'comparison')
    return ['projects.group_comparisons.read', 'projects.group_comparisons.view']; invalid('section', 'Choose receipt or comparison; awards require the approved finance workflow'); }
export async function workspace(token: string, target: string, bo: string, section = 'receipt') { return transaction(async (db) => { const { a, p, write } = await access(db, token, target, sectionPermissions(section), false); return workspaceBound(db, a, p.id, uuid(bo), write); }); }
export async function choices(token: string, target: string, section = 'receipt') {
    return transaction(async db => {
        const { p } = await access(db, token, target, sectionPermissions(section), false);
        const lists = (await boLists(db, p.id)).filter(b => b.is_active).map(b => ({id: b.id, name: b.name, state: b.state, item_count: b.lines.length}));
        return {item_list_mode: await itemListMode(db), merged_lists: await mergedListAliases(db, p.id), lists};
    });
}
export async function execute(token: string, target: string, bo: string, raw: Record<string, unknown>) {
    return transaction(async (db) => {
        const { a, p, write } = await access(db, token, target, ['projects.group_quotations.write', 'projects.group_quotations.read'], true);
        bo = uuid(bo);
        const b = input(raw), s = await source(db, p.id, bo);
        if (!s.available)
            invalid('', 'Project purchasing is inactive, held or closed');
        const before = await repo.get(db, p.id, bo), record: QuoteRecord = before ? structuredClone(before) : { project_id: p.id, bo_id: bo, quotes: [], frozen_document_ids: null, is_active: true, version: 0, created_at: '0001-01-01T00:00:00Z', updated_at: '0001-01-01T00:00:00Z', created_by_id: null, updated_by_id: null };
        if (record.version !== b.version)
            throw repo.stale();
        requireOpenNegotiation(s, before);
        record.frozen_document_ids = null;
        const d = s.documents.find(d => d.id === b.quote.document_id);
        if (!d || d.data.sent_date === null)
            invalid('document_id', 'Record dispatch of this vendor indent before entering its quotation');
        if (!d.data.is_current)
            invalid('document_id', 'This indent scope is superseded; record the revised indent quotation');
        if (!d.vendor_available)
            invalid('document_id', 'Vendor is inactive or unavailable');
        if ((await pending(db, p.id, bo)).includes(d.group_id))
            invalid('document_id', 'Waiting for Finance approval; group quotation information is frozen');
        const q = b.quote;
        if (q.currency !== s.currency)
            invalid('currency', 'Quotes must use the project currency');
        let attachment: Attachment | null = null;
        if (q.document_not_provided) {
            if (q.attachment_id !== null)
                invalid('attachment_id', 'Choose either a quotation file or document not provided');
            if (!q.no_document_reason)
                invalid('no_document_reason', 'Record how prices were received without a quotation document');
            q.evidence_reference = '';
        }
        else {
            if (q.attachment_id === null)
                invalid('attachment_id', 'Upload and select a quotation file for this exact vendor indent');
            const file = await repo.attachment(db, d.id, q.attachment_id);
            if (!file?.metadata.is_active)
                invalid('attachment_id', 'Select evidence belonging to this exact vendor indent');
            attachment = file.metadata;
            q.evidence_reference = attachment.filename;
            q.no_document_reason = '';
        }
        const revisions = await repo.revisions(db, p.id, bo);
        const current = record.quotes.find(quote => quote.document_id === q.document_id);
        const correction = Boolean(current && q.attachment_id === current.attachment_id &&
            q.amount_mode === 'total_only' && current.amount_mode === 'total_only' &&
            q.reference === current.reference &&
            q.total_amount !== null && amount(q.total_amount, 20, 2).fixed() === current.calculation.total);
        let revision = 1;
        for (const old of revisions) {
            if (q.attachment_id !== null && q.attachment_id === old.attachment_id && !correction)
                invalid('attachment_id', 'Upload revised evidence before saving a new quotation revision');
            if (old.document_id === q.document_id && old.revision >= revision)
                revision = old.revision + 1;
        }
        const normalized = normalizeMoney(q), at = now(), quote: Quote = { ...normalized, calculation: calculate(normalized, d, indiaDate(new Date())), attachment, created_at: at, created_by_id: a.id, updated_at: at, updated_by_id: a.id, recorded_at: at, recorded_by_id: a.id, revision, reason: b.reason };
        const old = record.quotes.find(q => q.document_id === quote.document_id);
        if (old) {
            quote.created_at = old.created_at;
            quote.created_by_id = old.created_by_id;
        }
        record.quotes = [...record.quotes.filter(q => q.document_id !== quote.document_id), quote];
        record.version++;
        const saved = await repo.save(db, record, a.id);
        await repo.event(db, before, saved, b.reason, a.id);
        return workspaceBound(db, a, p.id, bo, write);
    });
}
export async function setNegotiationFreeze(token: string, target: string, bo: string, action: 'freeze' | 'unfreeze', raw: Record<string, unknown>) {
    return transaction(async db => {
        const { a, p, write } = await access(db, token, target, ['projects.group_quotations.write', 'projects.group_quotations.read'], true);
        bo = uuid(bo);
        const expected = version(raw.version);
        const current = await source(db, p.id, bo);
        if (!current.available)
            invalid('', 'Project purchasing is inactive, held or closed');
        const before = await repo.get(db, p.id, bo);
        if ((before?.version ?? 0) !== expected)
            throw repo.stale();
        const receipt = receiptState(current.documents, before, current.expectedReceipts);
        const freeze = action === 'freeze';
        if (freeze && !current.dispatchComplete)
            invalid('', 'Complete vendor indent dispatch before freezing negotiations');
        if (freeze && !receipt.ready)
            invalid('', 'Record quotations for every current vendor indent before freezing negotiations');
        if (freeze) {
            const [blocker] = freezeBlockers(current, before, indiaDate(new Date()));
            if (blocker)
                invalid('freeze', `${blocker.group_name} · ${blocker.vendor}: ${blocker.exclusions.join('. ')}`);
        }
        if (!before || (freeze ? receipt.frozen : before.frozen_document_ids === null))
            return workspaceBound(db, a, p.id, bo, write);
        const saved = await repo.save(db, {
            ...before, version: before.version + 1, frozen_document_ids: freeze ? receipt.documentIds : null,
        }, a.id);
        await repo.event(db, before, saved, freeze ? 'Vendor negotiations frozen' : 'Vendor negotiations reopened', a.id,
            freeze ? 'negotiation_frozen' : 'negotiation_unfrozen');
        return workspaceBound(db, a, p.id, bo, write);
    });
}
export function requireOpenNegotiation(current: Awaited<ReturnType<typeof source>>, record: QuoteRecord | null) {
    if (!current.dispatchComplete)
        invalid('', 'Complete vendor indent dispatch before recording quotations');
    if (receiptState(current.documents, record, current.expectedReceipts).frozen)
        invalid('', 'Unfreeze negotiations before changing quotations');
}
export async function uploadAuthority(token: string, target: string, bo: string, document: string) { return transaction(async (db) => { const { p } = await access(db, token, target, ['projects.group_quotations.write', 'projects.group_quotations.read'], true); bo = uuid(bo); document = uuid(document); const s = await source(db, p.id, bo); requireOpenNegotiation(s, await repo.get(db, p.id, bo)); if (!s.available || !s.documents.some(d => d.id === document && d.data.sent_date !== null && d.data.is_current && d.vendor_available))
    invalid('document_id', 'Upload only for a current dispatched indent while purchasing is available'); }); }
