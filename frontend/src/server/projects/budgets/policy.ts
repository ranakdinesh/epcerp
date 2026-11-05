import type { Data, Item, Price, Owner, Report } from './types';
import { money, format, qty, lineBudget, add } from './money';
import { id, invalid, stale } from './validation';
import { trim } from '../validation';
import { lower } from '../../tenders/intake/receipts';
export const key = (a: {
    bo_id: string;
    line_id: string;
}) => a.bo_id + ':' + a.line_id;
export const emptyAudit = () => ({
    created_at: '0001-01-01T00:00:00Z', updated_at: '0001-01-01T00:00:00Z', created_by_id: null, updated_by_id: null
});
const kinds = new Set(['purchase', 'labour', 'subcontract', 'equipment', 'engineering', 'logistics', 'overheads', 'testing', 'contingency', 'other']);
export function validate(raw: Data, currency: string, owners: Owner[], items: Item[], previous: Data | null): Data {
    const d = structuredClone(raw);
    if (d.pricing_mode && d.pricing_mode !== 'unit_rates')
        invalid('pricing_mode', 'Choose the supported unit-rate budget mode');
    d.owner_id = id(d.owner_id);
    d.total = format(money(d.total));
    if (d.currency !== currency)
        invalid('currency', "Use the project's currency");
    const eligible = new Set(owners.map(o => o.id));
    if (!eligible.has(d.owner_id))
        invalid('owner_id', 'Choose an active project budget owner');
    if ([...d.notes].length > 2000 || d.heads.length > 100 || d.allocations.length > 1000)
        invalid('data', 'Budget notes or record count exceeds permitted limits');
    const heads = new Map<string, Data['heads'][number]>(), allHeads = new Set<string>(), allAllocations = new Set<string>(), names = new Set<string>();
    for (const h of d.heads) {
        h.id = id(h.id);
        if (allHeads.has(h.id))
            invalid('heads', 'Budget record identifiers must be distinct');
        allHeads.add(h.id);
        h.owner_id = id(h.owner_id);
        if (h.parent_id !== null)
            h.parent_id = id(h.parent_id);
        h.name = trim(h.name);
        if (!h.name || [...h.name].length > 128 || !kinds.has(h.kind))
            invalid('heads', 'Choose a valid cost category and a head name of at most 128 characters');
        h.amount = format(money(h.amount));
        if (h.is_active) {
            const k = (h.parent_id ?? '') + ':' + lower(h.name);
            if (names.has(k))
                invalid('heads', 'Use distinct head names within each parent');
            names.add(k);
            if (!eligible.has(h.owner_id))
                invalid('owner_id', 'Choose an active head owner');
            heads.set(h.id, h);
        }
    }
    const sources = new Map(items.map(s => [key(s), s])), assigned = new Set<string>();
    for (const a of d.allocations) {
        a.id = id(a.id);
        a.bo_id = id(a.bo_id);
        a.line_id = id(a.line_id);
        a.head_id = id(a.head_id);
        a.owner_id = id(a.owner_id);
        if (allAllocations.has(a.id))
            invalid('allocations', 'Budget record identifiers must be distinct');
        allAllocations.add(a.id);
        a.amount = format(money(a.amount));
        if (a.unit_price !== null)
            a.unit_price = format(money(a.unit_price));
        if (a.pricing_quantity !== null)
            qty(a.pricing_quantity);
        if (a.is_active) {
            const source = sources.get(key(a));
            if (assigned.has(key(a)) || !source)
                invalid('allocations', "Choose distinct items from this project's BO lists");
            assigned.add(key(a));
            const head = heads.get(a.head_id);
            if (!head || head.kind !== 'purchase' || !eligible.has(a.owner_id))
                invalid('allocations', 'Item allocations need an active purchase head and responsible owner');
            if (a.unit_price !== null) {
                if (a.pricing_quantity === null || qty(a.pricing_quantity) !== qty(source.quantity))
                    invalid('pricing_quantity', 'Purchase quantities changed; reopen Item purchase and save prices');
                if (lineBudget(source.quantity, a.unit_price) !== a.amount)
                    invalid('amount', 'Item budget must equal current quantity multiplied by unit price');
            }
            a.source = structuredClone(source);
        }
    }
    if (previous) {
        const old = new Map(previous.allocations.map(a => [a.id, a]));
        for (const a of d.allocations) {
            const o = old.get(a.id);
            if (o) {
                if (o.bo_id !== a.bo_id || o.line_id !== a.line_id)
                    invalid('allocations', "Retain each allocation's original BO and item identity");
                if (!a.is_active)
                    a.source = structuredClone(o.source);
            }
        }
        for (const h of previous.heads)
            if (!allHeads.has(h.id))
                invalid('heads', 'Deactivate existing budget records instead of deleting them');
        for (const a of previous.allocations)
            if (!allAllocations.has(a.id))
                invalid('allocations', 'Deactivate existing budget records instead of deleting them');
    }
    let roots = BigInt(0);
    for (const h of heads.values()) {
        const path = new Set([h.id]);
        let parent = h.parent_id;
        while (parent !== null) {
            const p = heads.get(parent);
            if (!p)
                invalid('parent_id', 'Subheads require an active parent');
            if (path.has(p.id) || path.size >= 6)
                invalid('parent_id', 'Budget hierarchy must be acyclic and at most six levels');
            path.add(p.id);
            if (p.kind !== h.kind)
                invalid('kind', "Subheads must use their parent's cost category");
            parent = p.parent_id;
        }
        const amount = money(h.amount);
        if (h.parent_id === null)
            roots = add(roots, amount);
        let committed = BigInt(0);
        for (const child of heads.values())
            if (child.parent_id === h.id)
                committed = add(committed, money(child.amount));
        for (const a of d.allocations)
            if (a.is_active && a.head_id === h.id)
                committed = add(committed, money(a.amount));
        if (committed > amount)
            invalid('amount', 'Subheads and item allocations exceed ' + h.name);
    }
    if (roots > money(d.total))
        invalid('total', 'Cost heads exceed the project budget');
    return d;
}
export function report(d: Data): Report {
    const total = money(d.total);
    let roots = BigInt(0), purchase = BigInt(0), items = BigInt(0);
    const heads: Report['heads'] = [];
    for (const a of d.allocations)
        if (a.is_active)
            items = add(items, money(a.amount));
    for (const h of d.heads) {
        if (!h.is_active)
            continue;
        const v = money(h.amount);
        if (h.parent_id === null) {
            roots = add(roots, v);
            if (h.kind === 'purchase')
                purchase = add(purchase, v);
        }
        let assigned = BigInt(0);
        for (const c of d.heads)
            if (c.is_active && c.parent_id === h.id)
                assigned = add(assigned, money(c.amount));
        for (const a of d.allocations)
            if (a.is_active && a.head_id === h.id)
                assigned = add(assigned, money(a.amount));
        if (assigned > v)
            invalid('amount', 'Subheads and item allocations exceed ' + h.name);
        heads.push({ ...h, unallocated: format(v - assigned) });
    }
    if (roots > total)
        invalid('total', 'Cost heads exceed the project budget');
    return {
        total: format(total), allocated_heads: format(roots), purchase_total: format(purchase), unallocated: format(total - roots), item_allocations: format(items), heads
    };
}
export function priceItems(previous: Data | null, prices: Price[], sources: Item[], owner: string, total: string, currency: string, newID: () => string): Data {
    const mapped = new Map(sources.map(s => [key(s), s])), seen = new Set<string>();
    for (const p of prices) {
        const s = mapped.get(key(p));
        if (seen.has(key(p)) || !s)
            invalid('prices', 'Purchase items changed; reload the complete project item list');
        seen.add(key(p));
        if (qty(p.quantity) !== qty(s.quantity) || p.bo_version !== s.bo_version)
            throw stale();
    }
    if (seen.size !== mapped.size)
        invalid('prices', 'Purchase items changed; reload the complete project item list');
    const d = previous ? structuredClone(previous) : {
        owner_id: owner, heads: [], allocations: [], total: '0.00', currency, notes: ''
    } as Data;
    d.total = format(money(total));
    d.currency = currency;
    d.pricing_mode = 'unit_rates';
    const purchase = new Map(d.heads.filter(h => h.is_active && h.kind === 'purchase').map(h => [h.id, h]));
    let root = d.heads.find(h => h.is_active && h.kind === 'purchase' && h.parent_id === null)?.id;
    if (!root) {
        root = newID();
        const h = {
            ...emptyAudit(), id: root, parent_id: null, name: 'Item purchase', kind: 'purchase', amount: '0.00', owner_id: owner, is_active: true
        };
        d.heads.push(h);
        purchase.set(root, h);
    }
    const current = new Map(d.allocations.filter(a => a.is_active).map(a => [key(a), a]));
    for (const a of d.allocations)
        if (!mapped.has(key(a)))
            a.is_active = false;
    for (const p of prices) {
        let a = current.get(key(p));
        const rate = p.changed ? p.unit_price : a?.unit_price ?? null;
        if (rate === null) {
            if (p.changed && a)
                a.is_active = false;
            continue;
        }
        const rateText = format(money(rate)), source = mapped.get(key(p))!;
        if (!a) {
            a = {
                ...emptyAudit(), id: newID(), bo_id: p.bo_id, line_id: p.line_id, head_id: root, owner_id: owner, amount: '0.00', is_active: true, unit_price: null, pricing_quantity: null, source: null
            };
            d.allocations.push(a);
        }
        a.unit_price = rateText;
        a.pricing_quantity = source.quantity;
        a.amount = lineBudget(source.quantity, rateText);
        a.is_active = true;
        if (!purchase.has(a.head_id))
            a.head_id = root;
    }
    const rollup = (id: string, path: Set<string>): bigint => {
        if (path.has(id))
            invalid('heads', 'Purchase budget hierarchy must be acyclic');
        path.add(id);
        let total = BigInt(0);
        for (const a of d.allocations)
            if (a.is_active && a.head_id === id)
                total = add(total, money(a.amount));
        for (const h of purchase.values())
            if (h.parent_id === id)
                total = add(total, rollup(h.id, path));
        path.delete(id);
        purchase.get(id)!.amount = format(total);
        return total;
    };
    for (const h of purchase.values())
        if (h.parent_id === null)
            rollup(h.id, new Set());
    return d;
}
export function attribute(d: Data, previous: Data | null, actor: string, at: string) {
    for (const kind of ['heads', 'allocations'] as const) {
        const old = new Map<string, {
            created_at: string;
            created_by_id: string | null;
        }>(previous?.[kind].map(h => [h.id, h]) ?? []);
        for (const h of d[kind]) {
            const o = old.get(h.id);
            h.created_at = o?.created_at ?? at;
            h.created_by_id = o ? o.created_by_id : actor;
            h.updated_at = at;
            h.updated_by_id = actor;
        }
    }
    return d;
}
