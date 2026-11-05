import { invalid, uuid, version, reason, object, string, requiredText } from '../core/validation';
import { quantity } from '../core/decimal';
import type { Input, Line } from './types';
export function saveInput(raw: Record<string, unknown>) {
    const b = object(raw, ['data', 'version', 'reason']), d = object(b.data, ['currency', 'doer_id', 'resolver_id', 'lines', 'documents']);
    const data: Input = { currency: string(d.currency, 'currency', ''), doer_id: uuid(d.doer_id, 'doer_id', 'Select an active doer'), resolver_id: uuid(d.resolver_id, 'resolver_id', 'Select an active resolver'), lines: [], documents: requiredText(d.documents, 'documents') };
    if (!/^[A-Z]{3}$/.test(data.currency))
        invalid('currency', 'Use a three-letter currency code');
    if (data.doer_id === data.resolver_id)
        invalid('resolver_id', 'Procurement doer and problem resolver must be different people');
    if (!Array.isArray(d.lines) || d.lines.length < 1 || d.lines.length > 200)
        invalid('lines', 'Add 1–200 requirements');
    const seen = new Set<string>();
    for (const row of d.lines) {
        const line = object(row, ['demand_id', 'quantity', 'budget', 'full_kit']), value: Line = { demand_id: uuid(line.demand_id, 'demand_id', 'Select a valid requirement'), quantity: string(line.quantity, 'quantity', ''), budget: string(line.budget, 'budget', ''), full_kit: requiredText(line.full_kit, 'full_kit') };
        if (seen.has(value.demand_id))
            invalid('demand_id', 'Duplicate requirement in indent');
        seen.add(value.demand_id);
        quantity(value.quantity);
        if (!/^[0-9]{1,16}(\.[0-9]{1,2})?$/.test(value.budget))
            invalid('budget', 'Budget must be nonnegative with up to 16 integer digits and 2 decimals');
        data.lines.push(value);
    }
    return { data, version: version(b.version, 'version', 1), reason: reason(b.reason, 'Provide a value within 2000 characters') };
}
export function actionInput(raw: Record<string, unknown>) {
    const b = object(raw, ['version', 'reason', 'evidence', 'action', 'line_ids']), action = string(b.action, 'action', '');
    if (!['submit', 'return', 'review', 'release', 'withdraw'].includes(action))
        invalid('action', 'Unknown indent action');
    if (b.line_ids !== undefined && !Array.isArray(b.line_ids))
        invalid('line_ids', 'Select at most 200 requirements');
    const lines = b.line_ids as unknown[] ?? [];
    if (lines.length > 200)
        invalid('line_ids', 'Select at most 200 requirements');
    const line_ids = lines.map(id => uuid(id));
    if (new Set(line_ids).size !== line_ids.length)
        invalid('line_ids', 'Select distinct lines for release');
    return { action, line_ids, version: version(b.version, 'version', 1), reason: reason(b.reason, 'Provide a value within 2000 characters'), evidence: requiredText(b.evidence, 'evidence') };
}
