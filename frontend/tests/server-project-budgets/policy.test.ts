import test from 'node:test';
import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';
import { money, format, qty, lineBudget, add, maximum } from '../../src/server/projects/budgets/money';
import { parse, command } from '../../src/server/projects/budgets/input';
import { validate, emptyAudit } from '../../src/server/projects/budgets/policy';
import type { Data } from '../../src/server/projects/budgets/types';
import { BudgetError } from '../../src/server/projects/budgets/validation';
test('budget amounts and quantity products preserve paise beyond safe integers', () => {
    assert.equal(format(money('99999999999999.99')), '99999999999999.99');
    assert.equal(lineBudget('0.005', '1.00'), '0.01');
    assert.equal(lineBudget('2.505', '12.34'), '30.91');
    assert.equal(qty('99999999999999.9999'), BigInt('999999999999999999'));
    for (const invalid of ['-1', 'NaN', 'Infinity', '1e3', '1.001', '1\n', '100000000000000'])
        assert.throws(() => money(invalid), BudgetError);
    assert.throws(() => add(maximum, BigInt(1)), BudgetError);
    assert.throws(() => lineBudget('99999999999999', '99999999999999'), BudgetError);
    assert.throws(() => qty('0'), BudgetError);
});
test('JSON translation retains original decimal lexemes and forbids source/audit overrides', () => {
    const raw = parse('{"total":99999999999999.99,"amount":0.10,"quantity":2.505,"version":3}') as Record<string, unknown>;
    assert.equal(raw.total, '99999999999999.99');
    assert.equal(raw.amount, '0.10');
    assert.equal(raw.quantity, '2.505');
    assert.equal(raw.version, 3);
    const id = randomUUID(), data = {
        total: '100', currency: 'INR', owner_id: id, heads: [], allocations: []
    };
    for (const value of [{
            action: 'save', version: 0, reason: 'Invalid', data: { ...data, approved: {} }
        }, {
            action: 'save', version: '0', reason: 'Invalid', data
        }, {
            action: 'approve', version: 1, reason: 'Invalid', data: null
        }, {
            action: 'price_items', version: 0, reason: 'Invalid', prices: null
        }])
        assert.throws(() => command(value), BudgetError);
    assert.throws(() => command(parse('{"action":"save","version":0,"reason":"Exponent","data":{"total":1e2,"currency":"INR","owner_id":"' + id + '","heads":[],"allocations":[]}}')), BudgetError);
});
test('budget hierarchy permits six levels and rejects cycles, excessive depth and mixed categories', () => {
    const owner = randomUUID(), heads = Array.from({ length: 6 }, (_, i) => ({
        ...emptyAudit(), id: randomUUID(), parent_id: null as string | null, name: 'Level ' + i, kind: 'purchase', amount: '100.00', owner_id: owner, is_active: true
    }));
    for (let i = 1; i < heads.length; i++)
        heads[i].parent_id = heads[i - 1].id;
    const d: Data = {
        total: '100.00', currency: 'INR', owner_id: owner, notes: '', heads, allocations: []
    }, owners = [{ id: owner, name: 'Synthetic' }];
    assert.equal(validate(d, 'INR', owners, [], null).heads.length, 6);
    const deep = structuredClone(d);
    deep.heads.push({ ...heads[5], id: randomUUID(), parent_id: heads[5].id });
    assert.throws(() => validate(deep, 'INR', owners, [], null), BudgetError);
    const cycle = structuredClone(d);
    cycle.heads[0].parent_id = heads[5].id;
    assert.throws(() => validate(cycle, 'INR', owners, [], null), BudgetError);
    const mixed = structuredClone(d);
    mixed.heads[1].kind = 'labour';
    assert.throws(() => validate(mixed, 'INR', owners, [], null), BudgetError);
});
