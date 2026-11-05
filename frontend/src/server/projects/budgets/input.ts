import type { Command, Data, Head, Allocation, Price } from './types';
import { object, text, version, reason, decimal, bool, invalid, id } from './validation';
import { money, qty } from './money';
import { emptyAudit, key } from './policy';
const rows = (v: unknown, field: string, max: number) => {
    if (!Array.isArray(v))
        invalid(field, 'Expected an array');
    if (v.length > max)
        invalid(field, 'Too many records');
    return v;
};
export function command(raw: unknown): Command {
    const c = object(raw, ['action', 'version', 'reason', 'data', 'prices'], ['action', 'version', 'reason'], ['data']);
    const action = text(c.action);
    if (!['save', 'price_items', 'approve', 'revise'].includes(action))
        invalid('action', 'Choose save, price_items, approve or revise');
    if (['approve', 'revise'].includes(action)) {
        if ('data' in c)
            invalid('data', 'Financial commands use saved budget data');
        if ('prices' in c)
            invalid('prices', 'Financial commands use saved item budgets');
    }
    let data: Data | null = null;
    if (c.data != null) {
        const d = object(c.data, ['total', 'currency', 'owner_id', 'notes', 'pricing_mode', 'heads', 'allocations'], ['total', 'currency', 'owner_id', 'heads', 'allocations']);
        const heads: Head[] = rows(d.heads, 'data.heads', 100).map(raw => {
            const h = object(raw, ['id', 'parent_id', 'name', 'kind', 'amount', 'owner_id', 'is_active'], ['id', 'name', 'kind', 'amount', 'owner_id', 'is_active'], ['parent_id']);
            return {
                ...emptyAudit(), id: text(h.id), parent_id: h.parent_id == null ? null : text(h.parent_id), name: text(h.name), kind: text(h.kind), amount: decimal(h.amount), owner_id: text(h.owner_id), is_active: bool(h.is_active)
            };
        });
        const allocations: Allocation[] = rows(d.allocations, 'data.allocations', 1000).map(raw => {
            const a = object(raw, ['id', 'bo_id', 'line_id', 'head_id', 'amount', 'owner_id', 'is_active', 'unit_price', 'pricing_quantity'], ['id', 'bo_id', 'line_id', 'head_id', 'amount', 'owner_id', 'is_active'], ['unit_price', 'pricing_quantity']);
            return {
                ...emptyAudit(), id: text(a.id), bo_id: text(a.bo_id), line_id: text(a.line_id), head_id: text(a.head_id), amount: decimal(a.amount), owner_id: text(a.owner_id), is_active: bool(a.is_active), unit_price: a.unit_price == null ? null : decimal(a.unit_price), pricing_quantity: a.pricing_quantity == null ? null : decimal(a.pricing_quantity), source: null
            };
        });
        data = {
            total: decimal(d.total), currency: text(d.currency), owner_id: text(d.owner_id), notes: d.notes === undefined ? '' : text(d.notes), ...(d.pricing_mode === undefined || d.pricing_mode === '' ? {} : { pricing_mode: text(d.pricing_mode) }), heads, allocations
        };
    }
    const prices: Price[] = ('prices' in c ? rows(c.prices, 'prices', 1000) : []).map(raw => {
        const p = object(raw, ['bo_id', 'line_id', 'bo_version', 'quantity', 'unit_price', 'changed'], ['bo_id', 'line_id', 'bo_version', 'quantity', 'changed'], ['unit_price']);
        const quantity = decimal(p.quantity), rate = p.unit_price == null ? null : decimal(p.unit_price);
        qty(quantity);
        if (rate !== null)
            money(rate);
        return {
            bo_id: id(p.bo_id), line_id: id(p.line_id), bo_version: version(p.bo_version, 1), quantity, unit_price: rate, changed: bool(p.changed)
        };
    });
    if (new Set(prices.map(key)).size !== prices.length)
        invalid('prices', 'Provide each purchase item once');
    if (action === 'save' && !data)
        invalid('data', 'Provide budget data');
    if (action === 'save' && prices.length)
        invalid('prices', 'Use price_items to update unit budgets');
    if (action === 'price_items' && data)
        invalid('data', 'Use save to update budget heads');
    return {
        action, version: version(c.version), reason: reason(c.reason), data, prices
    };
}
// The JSON source context keeps unquoted decimal lexemes exact, including values
// whose paise cannot fit in a JavaScript number. Version fields remain numbers.
export function parse(raw: string) {
    return JSON.parse(raw, (key, value, context?: {
        source: string;
    }) => {
        if (typeof value !== 'number')
            return value;
        if (['total', 'amount', 'quantity', 'unit_price', 'pricing_quantity'].includes(key))
            return context?.source ?? invalid(key, 'Exact decimal parsing unavailable');
        if (['version', 'bo_version'].includes(key) && (!context?.source || !/^-?\d+(?![\s\S])/.test(context.source)))
            invalid(key, 'Use the current budget version');
        return value;
    }) as unknown;
}
