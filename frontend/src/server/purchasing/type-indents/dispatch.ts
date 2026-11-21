import 'server-only';
import { randomUUID } from 'node:crypto';
import type { DB } from '../../db';
import { assignmentIdentity } from '../../auth/access';
import { get as getBO } from '../bo/repository';
import { forbidden, type Actor, type Project } from '../core/access';
import { indiaInstant } from '../core/time';
import { PurchaseError } from '../core/validation';
import * as groups from '../groups/repository';
import type { Group, GroupRecord, Vendor } from '../groups/types';
import { sameSet, validateGroups } from '../groups/validation';
import * as documents from '../indent-documents/repository';
import type { Data, Document } from '../indent-documents/types';
import { readIndentContent } from '@/lib/indent-content';

export type ItemSet = { category: string; itemIds: string[]; vendorIds: string[] };
type Item = { id: string; bo_id: string; category_name: string | null };

export async function currentDocuments(db: DB, project: string, items: Item[]) {
    const current: Document[] = [];
    for (const bo of new Set(items.map(item => item.bo_id))) {
        const record = await groups.get(db, project, bo);
        if (!record?.is_active) continue;
        const revisions = new Map(record.groups.map(group => [group.id, group.scope_revision]));
        current.push(...(await documents.list(db, project, bo)).filter(document =>
            document.is_active && document.data.scope_revision === revisions.get(document.group_id)));
    }
    return current;
}

export async function prepareDispatch(
    db: DB, actor: Actor, project: Project, items: Item[], sets: ItemSet[], vendors: Vendor[],
    indent: NonNullable<Data['type_indent']>, reissue: boolean,
) {
    if (!actor.allows('projects.purchase_indent_groups.read') || !actor.allows('projects.purchase_indent_groups.write') ||
        !actor.allows('projects.purchase_group_vendors.write')) throw forbidden();
    const user = await assignmentIdentity(db, actor.id);
    if (!user?.active) throw new PurchaseError(401, 'Invalid or expired credentials');
    const lookup = new Map(vendors.map(vendor => [vendor.id, vendor]));
    const content = readIndentContent(indent.content);
    const plain = (paragraphs: typeof content.notes) => paragraphs.map(paragraph => paragraph.map(run => run.text).join('')).filter(Boolean);
    const indentTerms = { source: 'type_indent', notes: plain(content.notes), conditions: plain(content.conditions), delivery_basis: '' };
    for (const bo of new Set(items.map(item => item.bo_id))) {
        const list = await getBO(db, project.id, bo);
        if (!list?.is_active || list.state !== 'completed') throw groups.stale();
        const local = new Map(items.filter(item => item.bo_id === bo).map(item => [item.id, item]));
        const planned = sets.flatMap((set, index) => {
            const line_ids = set.itemIds.filter(id => local.has(id));
            return line_ids.length ? [{ category: set.category, group: {
                id: randomUUID(), name: `${(local.get(line_ids[0])?.category_name || set.category).slice(0, 110)} · Set ${index + 1}`,
                line_ids, vendor_ids: set.vendorIds, invitations: [], scope_revision: 1,
            } satisfies Group }] : [];
        });
        if (!planned.some(entry => entry.category === indent.category)) continue;
        const before = await groups.get(db, project.id, bo);
        const existing = await documents.list(db, project.id, bo);
        const match = (group: Group) => before?.groups.find(old =>
            sameSet(old.line_ids, group.line_ids) && sameSet(old.vendor_ids, group.vendor_ids));
        const reusable = before?.is_active && before.state === 'created' && before.bo_version === list.version &&
            before.groups.length === planned.length && planned.every(({ group }) => match(group)) &&
            existing.filter(doc => before.groups.some(group => group.id === doc.group_id && group.scope_revision === doc.data.scope_revision))
                .every(doc => doc.data.type_indent);
        if (!reusable) {
            const legacySent = existing.some(doc => doc.data.sent_date) || before?.groups.some(group => group.invitations.length);
            if (existing.some(doc => doc.data.type_indent))
                throw new PurchaseError(409, 'This purchase list changed after an indent was finalised. Reconcile the issued scope before preparing another indent.');
            if (legacySent && !reissue)
                throw new PurchaseError(409, 'Confirm reissue before replacing the current vendor scope. Earlier dispatch records are retained.');
            const [evidence] = (await db.query<{ exists: boolean }>(`
                SELECT EXISTS(
                    SELECT 1 FROM purchase_group_quotations WHERE bo_id=$1
                    UNION ALL SELECT 1 FROM quotation_approval_sets WHERE bo_id=$1
                    UNION ALL SELECT 1 FROM purchase_quotation_attachments a
                        JOIN purchase_indent_documents d ON d.id=a.document_id WHERE d.bo_id=$1
                )`, [bo])).rows;
            if (evidence.exists)
                throw new PurchaseError(409, 'This purchase list has quotation or approval evidence. Reconcile it before replacing the vendor scope.');
            const savedGroups = planned.map(entry => entry.group);
            const lineIds = list.lines.map(line => line.id);
            validateGroups(savedGroups, lineIds, [...lookup.keys()], true);
            const record: GroupRecord = {
                project_id: project.id, bo_id: bo, version: (before?.version ?? 0) + 1, bo_version: list.version,
                state: 'created', groups: savedGroups, source_lines: list.lines,
                source_vendors: vendors.filter(vendor => savedGroups.some(group => group.vendor_ids.includes(vendor.id))),
                selected_line_ids: lineIds, number: 'GI-' + bo.replaceAll('-', '').slice(0, 12).toUpperCase(),
                is_active: true, created_at: before?.created_at ?? '', updated_at: '',
                created_by_id: before?.created_by_id ?? actor.id, updated_by_id: actor.id,
            };
            const saved = await groups.save(db, record, actor.id);
            await groups.event(db, before, saved, 'create', legacySent
                ? 'Reissue category vendor indents for the current purchase list'
                : 'Freeze category vendor assignments for indent dispatch', actor.id);
        }
        for (const entry of planned.filter(entry => entry.category === indent.category)) {
            const group = reusable ? match(entry.group)! : entry.group;
            for (const vendorId of group.vendor_ids) {
                const id = randomUUID();
                const document: Document = {
                    id, project_id: project.id, bo_id: bo, group_id: group.id, vendor_id: vendorId, version: 1,
                    is_active: true, created_at: '', updated_at: '', created_by_id: null, updated_by_id: null,
                    data: {
                        type_indent: indent, terms: indentTerms, vendor: lookup.get(vendorId)!,
                        lines: list.lines.filter(line => group.line_ids.includes(line.id)),
                        project_name: project.name, generated_by_name: user.name, group_name: group.name,
                        scope_revision: group.scope_revision, number: 'IND-' + id.replaceAll('-', '').toUpperCase(),
                        generated_at: indiaInstant(), generated_by_id: actor.id, print_requests: [],
                        sent_date: null, channel: '', dispatch_reference: '', sent_recorded_at: null, sent_by_id: null, is_current: false,
                    },
                };
                const saved = await documents.save(db, document, actor.id);
                await documents.event(db, null, saved, 'generate', 'Finalise category indent for vendor dispatch', actor.id);
            }
        }
    }
}
