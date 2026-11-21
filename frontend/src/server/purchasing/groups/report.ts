import type { GroupRecord, Line, Vendor, Invitation } from './types';
export function candidates(record: GroupRecord | null, current: Vendor[]) {
    const out = [...current];
    if (!record)
        return out;
    const active = new Set(current.map(v => v.id)), seen = new Set<string>();
    for (const id of record.groups.flatMap(g => g.vendor_ids).sort()) {
        if (active.has(id) || seen.has(id))
            continue;
        seen.add(id);
        out.push({ ...record.source_vendors.find(v => v.id === id) ?? { id, name: 'Unavailable vendor', code: '', data: {}, version: 0 }, is_active: false });
    }
    return out;
}
export function report(record: GroupRecord | null, current: Line[], vendors: Vendor[]) {
    const out: {
        id: string;
        sku: string;
        item: string;
        quantity: string | null;
        uom: string;
        group: string;
        selected_vendors: {
            id: string;
            name: string;
        }[];
        invitations: (Invitation & {
            vendor_name: string;
        })[];
    }[] = [];
    if (!record)
        return out;
    const name = (id: string) => vendors.find(v => v.id === id)?.name ?? 'Unavailable vendor';
    for (const line of record.state === 'created' ? record.source_lines : current) {
        if (!record.selected_line_ids.includes(line.id))
            continue;
        const g = record.groups.find(g => g.line_ids.includes(line.id));
        out.push({ id: line.id, sku: line.item_snapshot.sku, item: line.item_snapshot.description, quantity: line.quantity, uom: line.item_snapshot.uom, group: g?.name ?? 'Ungrouped', selected_vendors: g?.vendor_ids.map(id => ({ id, name: name(id) })) ?? [], invitations: g?.invitations.filter(i => i.line_ids.includes(line.id)).map(i => ({ ...i, vendor_name: name(i.vendor_id) })) ?? [] });
    }
    return out;
}
