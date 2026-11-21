import 'server-only';
import { validateIndentContent } from '@/lib/indent-content';
import { and, eq, sql } from 'drizzle-orm';
import { drizzle } from 'drizzle-orm/node-postgres';
import { integer, jsonb, pgTable, text, timestamp, uuid as uuidColumn } from 'drizzle-orm/pg-core';
import { rows, transaction } from '../../db';
import { purchaseVendors } from '../../master/vendors/purchase-reader';
import { actor, project, scope } from '../core/access';
import { invalid, object, PurchaseError, uuid, version } from '../core/validation';
import { workspace } from '../workspace/repository';

import { currentDocuments, prepareDispatch, type ItemSet } from './dispatch';
import { recordDispatch } from '../indent-documents/service';
import { input as documentInput } from '../indent-documents/validation';
import * as documents from '../indent-documents/repository';
import { source as progressSource } from '../progress/source';

const drafts = pgTable('purchase_type_indent_drafts', {
    project: uuidColumn('project_id').notNull(),
    category: text('category_code').notNull(),
    version: integer('version').notNull(),
    terms: text('terms').notNull(),
    itemSets: jsonb('vendor_item_sets').$type<ItemSet[]>(),
    vendorIds: uuidColumn('vendor_ids').array().notNull(),
    updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().default(sql`clock_timestamp()`),
    updatedBy: uuidColumn('updated_by_id').notNull(),
});
const projectVendors = pgTable('purchase_project_vendors', {
    project: uuidColumn('project_id').primaryKey(),
    itemSets: jsonb('item_sets').$type<ItemSet[]>(),
    vendorIds: uuidColumn('vendor_ids').array().notNull(),
    version: integer('version').notNull(),
    updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().default(sql`clock_timestamp()`),
    updatedBy: uuidColumn('updated_by_id').notNull(),
});
const permission = 'projects.purchase_indent_documents.read';
const dispatchPermission = 'projects.purchase_indent_documents.dispatch';
const writePermission = 'projects.purchase_indent_documents.generate';
type ItemRow = { id: string; bo_id: string; category: string | null; category_name: string | null; description: string | null; sku: string | null; quantity: string | null; unit: string | null; required_date: string | null };

function finalisedItems(client: Parameters<typeof rows>[0], projectId: string) {
    return rows<ItemRow>(client, `
            SELECT l.id, b.id AS bo_id, COALESCE(NULLIF(l.item_snapshot->>'category_code',''), c.code) AS category,
                c.name AS category_name, l.item_snapshot->>'description' AS description,
                l.item_snapshot->>'sku' AS sku, l.quantity::text, l.item_snapshot->>'uom' AS unit, l.required_date::text
            FROM purchase_bo_lists b JOIN purchase_bo_lines l ON l.bo_id=b.id AND l.is_active
            LEFT JOIN catalogue_items i ON i.id=l.item_id LEFT JOIN categories c ON c.id=i.category_id
            WHERE b.project_id=$1 AND b.is_active AND b.state='completed'
            ORDER BY b.created_at, l.sort_order, l.id
        `, [projectId]);
}

function completeSelection(sets: ItemSet[], items: ItemRow[], vendors: { id: string }[]) {
    const remaining = new Map(items.map(item => [item.id, item.category]));
    const eligible = new Set(vendors.map(vendor => vendor.id));
    const vendorsByCategory = new Map<string, Set<string>>();
    for (const set of sets) {
        if (!set.category || !set.itemIds.length || set.vendorIds.length < 3 || set.vendorIds.length > 50) return false;
        const used = vendorsByCategory.get(set.category) ?? new Set<string>();
        for (const id of set.itemIds) {
            if (remaining.get(id) !== set.category) return false;
            remaining.delete(id);
        }
        for (const id of set.vendorIds) {
            if (!eligible.has(id) || used.has(id)) return false;
            used.add(id);
        }
        vendorsByCategory.set(set.category, used);
    }
    return items.length > 0 && remaining.size === 0;
}

export function chooseProjectVendors(token: string, target: string, input?: Record<string, unknown>) {
    return transaction(async client => {
        const user = await actor(client, token, ['projects.purchase_bo.read', permission]);
        const p = await project(client, target, Boolean(input));
        const w = await workspace(client, p.id);
        const access = await scope(client, user, p, w, input ? writePermission : permission, Boolean(input));
        const db = drizzle({ client });
        const key = eq(projectVendors.project, p.id);
        const vendors = await purchaseVendors(client);
        const items = await finalisedItems(client, p.id);
        if (!input) {
            const [selection] = await db.select().from(projectVendors).where(key);
            return {
                selection, items,
                vendors: vendors.map(({ id, code, name }) => ({ id, code, name })),
                can_edit: user.allows(writePermission) && access.can_write && p.purchase_enabled,
            };
        }
        if (!Array.isArray(input.itemSets)) invalid('itemSets', 'Assign vendors to every item.');
        const itemSets = input.itemSets.map(value => {
            const set = object(value, ['category', 'itemIds', 'vendorIds']);
            if (typeof set.category !== 'string' || !Array.isArray(set.itemIds) || !Array.isArray(set.vendorIds))
                invalid('itemSets', 'Select valid items and vendors.');
            return { category: set.category, itemIds: set.itemIds.map(id => uuid(id, 'itemSets')), vendorIds: set.vendorIds.map(id => uuid(id, 'itemSets')) };
        });
        if (!completeSelection(itemSets, items, vendors))
            invalid('itemSets', 'Assign every finalised item to one set with at least three eligible vendors. Use each vendor only once per category.');
        const issued = (await currentDocuments(client, p.id, items)).some(doc => doc.data.type_indent);
        if (issued) {
            const [selection] = await db.select().from(projectVendors).where(key);
            const normalize = (sets: ItemSet[]) => sets.map(set =>
                JSON.stringify([set.category, [...set.itemIds].sort(), [...set.vendorIds].sort()])).sort().join('\n');
            if (!selection?.itemSets || normalize(itemSets) !== normalize(selection.itemSets))
                throw new PurchaseError(409, 'An indent has been finalised. Its vendor assignments are frozen.');
        }
        const vendorIds = [...new Set(itemSets.flatMap(set => set.vendorIds))];
        const expected = version(input.version);
        const [saved] = expected === 0
            ? await db.insert(projectVendors).values({ project: p.id, vendorIds, itemSets, version: 1, updatedBy: user.id }).onConflictDoNothing().returning()
            : await db.update(projectVendors).set({ vendorIds, itemSets, version: expected + 1, updatedBy: user.id, updatedAt: sql`clock_timestamp()` })
                .where(and(key, eq(projectVendors.version, expected))).returning();
        if (!saved) throw new PurchaseError(409, 'The vendor selection changed. Reload before saving.');
        return saved;
    });
}

export function typeIndents(token: string, target: string, input?: Record<string, unknown>, dispatch = false) {
    return transaction(async client => {
        const user = await actor(client, token, ['projects.purchase_bo.read', permission]);
        const p = await project(client, target, Boolean(input));
        const w = await workspace(client, p.id);
        const access = await scope(client, user, p, w, input ? (dispatch ? dispatchPermission : writePermission) : permission, Boolean(input));
        const db = drizzle({ client });
        const items = await finalisedItems(client, p.id);
        const [selection] = await db.select().from(projectVendors).where(eq(projectVendors.project, p.id));
        const current = await currentDocuments(client, p.id, items);
        const issued = current.filter(doc => doc.data.type_indent);
        async function dispatchComplete() {
            if (!issued.length || issued.some(doc => !doc.data.sent_date)) return false;
            return (await progressSource(client, p.id)).register.completed.includes(2);
        }
        if (dispatch && input) {
            const command = documentInput({ ...input, action: 'send' });
            const doc = issued.find(entry => entry.id === command.document_id);
            if (!doc) throw new PurchaseError(404, 'Finalised vendor indent not found.');
            if (doc.version !== command.version) throw documents.stale();
            const before = structuredClone(doc);
            await recordDispatch(client, user, p, doc, command);
            doc.version++;
            const saved = await documents.save(client, doc, user.id);
            await documents.event(client, before, saved, 'send', 'Record category indent dispatch', user.id);
            return { dispatch_complete: await dispatchComplete() };
        }
        const [details] = await rows<{ client: string; site: string }>(client,
            'SELECT c.display_name AS client,p.site_name AS site FROM projects p JOIN clients c ON c.id=p.client_id WHERE p.id=$1', [p.id]);
        if (input) {
            const category = typeof input.category === 'string' ? input.category : '';
            const content = input.content === undefined ? null : validateIndentContent(input.content);
            const terms = content ? 'indent-rich-v1\n' + JSON.stringify(content)
                : typeof input.terms === 'string' ? input.terms.trim() : '';
            if (input.content !== undefined && !content)
                invalid('content', 'Enter specifications and keep the total content within 24,000 characters.');
            if (!content && (!terms || terms.length > 8000))
                invalid('terms', 'Enter specifications within 8,000 characters.');
            const expected = version(input.version);
            if (!items.some(item => item.category === category)) invalid('category', 'Finalise items of this type before saving an indent.');
            if (issued.some(doc => doc.data.type_indent?.category === category))
                throw new PurchaseError(409, 'This indent is already finalised. Open vendor dispatch to continue.');
            const vendors = await purchaseVendors(client);
            if (!selection?.itemSets || !completeSelection(selection.itemSets, items, vendors))
                invalid('itemSets', 'Complete Choose vendors for every item before saving an indent.');
            const itemSets = selection.itemSets.filter(set => set.category === category);
            const vendorIds = itemSets.flatMap(set => set.vendorIds);
            const key = and(eq(drafts.project, p.id), eq(drafts.category, category));
            const [saved] = expected === 0
                ? await db.insert(drafts).values({ project: p.id, category, terms, vendorIds, itemSets, version: 1, updatedBy: user.id }).onConflictDoNothing().returning()
                : await db.update(drafts).set({ terms, vendorIds, itemSets, version: expected + 1, updatedBy: user.id, updatedAt: sql`clock_timestamp()` }).where(and(key, eq(drafts.version, expected))).returning();
            if (!saved) throw new PurchaseError(409, 'The indent draft changed. Reload before saving.');
            await prepareDispatch(client, user, p, items, selection.itemSets, vendors, {
                category, version: saved.version, content: saved.terms, client: details.client, site: details.site,
            }, input.reissue === true);
            return saved;
        }
        const vendorsByCategory = new Map<string, Set<string>>();
        for (const set of selection?.itemSets ?? []) {
            const vendors = vendorsByCategory.get(set.category) ?? new Set<string>();
            for (const id of set.vendorIds) vendors.add(id);
            vendorsByCategory.set(set.category, vendors);
        }
        return {
            items, details, projectName: p.name,
            reissue_required: current.some(doc => !doc.data.type_indent && Boolean(doc.data.sent_date)),
            dispatch_complete: Boolean(dispatch && await dispatchComplete()),
            can_dispatch: user.allows(dispatchPermission) && user.allows('projects.purchase_indent_groups.read') && access.can_write && p.purchase_enabled,
            dispatches: issued.map(doc => ({
                id: doc.id, version: doc.version, category: doc.data.type_indent!.category,
                group: doc.data.group_name, vendor: doc.data.vendor.name,
                number: doc.data.number, itemCount: doc.data.lines.length,
                generatedDate: doc.data.generated_at, sentDate: doc.data.sent_date,
            })),
            vendor_counts: Object.fromEntries([...vendorsByCategory].map(([category, vendors]) => [category, vendors.size])),
            vendors_ready: Boolean(selection?.itemSets && completeSelection(selection.itemSets, items, await purchaseVendors(client))),
            drafts: await db.select().from(drafts).where(eq(drafts.project, p.id)),
            can_edit: user.allows(writePermission) && user.allows('projects.purchase_indent_groups.read') &&
                user.allows('projects.purchase_indent_groups.write') && user.allows('projects.purchase_group_vendors.write') &&
                access.can_write && p.purchase_enabled,
        };
    });
}

export function typeIndentDocument(token: string, target: string, id: string) {
    const documentId = uuid(id);
    return transaction(async client => {
        const user = await actor(client, token, ['projects.purchase_bo.read', permission]);
        const p = await project(client, target);
        await scope(client, user, p, await workspace(client, p.id), permission, false);
        const doc = (await currentDocuments(client, p.id, await finalisedItems(client, p.id)))
            .find(entry => entry.id === documentId && entry.data.type_indent);
        if (!doc?.data.type_indent) throw new PurchaseError(404, 'Finalised vendor indent not found.');
        const snapshot = doc.data.type_indent;
        return {
            category: snapshot.category, project: doc.data.project_name, client: snapshot.client,
            site: snapshot.site, date: new Date(doc.data.generated_at), terms: snapshot.content,
            items: doc.data.lines.map(line => ({
                description: line.item_snapshot.description ?? line.item_snapshot.sku ?? '',
                quantity: line.quantity ?? '', unit: line.item_snapshot.uom ?? '',
            })),
        };
    });
}
