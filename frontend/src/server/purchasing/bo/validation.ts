import { invalid, uuid, reason, version, object, isoDate, string, trim } from '../core/validation';
import { quantity } from '../core/decimal';
import type { Save, InputLine, Line } from './types';
export function input(raw: Record<string, unknown>): Save {
    const b = object(raw, ['name', 'version', 'reason', 'lines']);
    if (b.lines === null)
        invalid('lines', 'This field must not be null');
    if (b.reason === null)
        invalid('reason', 'This field must not be null');
    if (b.version === null)
        invalid('version', 'This field must not be null');
    const result: Save = { name: b.name == null ? null : trim(string(b.name, 'name')), version: version(b.version ?? 0), reason: reason(b.reason, 'Provide a reason within 2000 characters'), lines: [] };
    if (result.name !== null && (!result.name || [...result.name].length > 128))
        invalid('name', 'BO name must contain 1–128 characters');
    if (b.lines !== undefined && !Array.isArray(b.lines))
        invalid('lines', 'Expected item row objects');
    if (Array.isArray(b.lines) && b.lines.length > 100)
        invalid('lines', 'Use at most 100 item rows');
    const seen = new Set<string>();
    for (const row of b.lines as unknown[] ?? []) {
        const r = object(row, ['id', 'item_id', 'quantity', 'required_date', 'reference', 'notes']);
        const line: InputLine = { id: uuid(r.id, 'lines', 'Each item row needs a valid identifier'), item_id: r.item_id == null ? null : uuid(r.item_id, 'item_id', 'Select a valid catalogue item'), quantity: r.quantity == null ? null : string(r.quantity, 'quantity'), required_date: r.required_date == null ? null : string(r.required_date, 'required_date'), reference: string(r.reference, 'reference', ''), notes: string(r.notes, 'notes', '') };
        if (r.reference === null || r.notes === null)
            invalid('lines', 'This field must not be null');
        if (seen.has(line.id))
            invalid('lines', 'Use distinct item row identifiers');
        seen.add(line.id);
        if (line.quantity !== null)
            quantity(line.quantity);
        if (line.required_date !== null && !isoDate(line.required_date))
            invalid('required_date', 'Use a valid required-at-site date');
        if ([...line.reference].length > 500 || [...line.notes].length > 4000)
            invalid('notes', 'Item reference/notes exceed their permitted length');
        result.lines.push(line);
    }
    return result;
}
export function validateLine(line: Line, complete: boolean) {
    if (complete && (!line.item_id || line.quantity === null || line.required_date === null))
        invalid('lines', 'Every item needs a catalogue selection, quantity and required-at-site date');
    if (line.quantity !== null && ['nos', 'each', 'number', 'numbers', 'pcs', 'piece', 'pieces'].includes(line.item_snapshot.uom.toLowerCase()) && quantity(line.quantity) % BigInt(1000) !== BigInt(0))
        invalid('quantity', 'Counted items require whole-number quantities');
}
