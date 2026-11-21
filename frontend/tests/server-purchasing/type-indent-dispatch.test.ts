import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';
import { writeFile } from 'node:fs/promises';
import test from 'node:test';
import { initializeAuth } from '../../src/server/auth/bootstrap';
import { login } from '../../src/server/auth/sessions';
import { pool } from '../../src/server/db';
import { handleMaster } from '../../src/server/master/http';
import { save, change } from '../../src/server/purchasing/bo/service';
import { calculate } from '../../src/server/purchasing/quotations/calculation';
import { input, normalizeMoney } from '../../src/server/purchasing/quotations/validation';
import { handlePurchasing } from '../../src/server/purchasing/http';
import { indiaDate } from '../../src/server/personal-tasks/validation';
import { project as createProject } from '../server-inbox/fixtures';
import { definition } from '../server-master/catalogue-fixtures';

test('category indent dispatch unlocks quotation pricing only after the last vendor dispatch', {
    skip: !process.env.EPCERP_DATABASE_URL,
}, async () => {
    assert.match(new URL(process.env.EPCERP_DATABASE_URL!).pathname, /^\/epcerp_auth_qa_purchase_/);
    const sql = pool();
    try {
        await initializeAuth();
        const session = await login('superadmin', process.env.VTPL_BOOTSTRAP_SUPERADMIN_PASSWORD!);
        const { id: actor } = (await sql.query("SELECT id FROM auth.users WHERE username='superadmin'")).rows[0];
        const project = await createProject(actor);
        await sql.query("UPDATE projects SET purchase_enabled=true,currency='INR' WHERE id=$1", [project]);
        const headers = { Authorization: 'Bearer ' + session.access_token, 'Content-Type': 'application/json' };
        const root = 'http://fixture.invalid/api/v1';
        const request = (handler: (request: Request) => Promise<Response>, path: string, data?: unknown) =>
            handler(new Request(root + path, { method: data ? 'POST' : 'GET', headers, body: data ? JSON.stringify(data) : undefined }));
        async function json(handler: typeof handlePurchasing, path: string, data?: unknown, expected = 200) {
            const response = await request(handler, path, data);
            const result = await response.json();
            assert.equal(response.status, expected, JSON.stringify(result));
            return result;
        }
        const prefix = 'QA' + randomUUID().slice(0, 8).toUpperCase();
        const unit = await json(handleMaster, '/categories/units', { code: prefix, name: prefix, reason: 'Fixture unit' }, 201);
        const category = await json(handleMaster, '/categories/categories', {
            code: prefix + 'C', name: prefix, unit_id: unit.id, reason: 'Fixture category',
        }, 201);
        const sub = await json(handleMaster, '/categories/subcategories', {
            code: prefix + 'T', name: prefix, category_id: category.id, reason: 'Fixture subcategory',
        }, 201);
        const template = await json(handleMaster, '/configuration-templates/' + sub.id + '/publish', {
            definition: { ...definition(), subcategory_code: sub.code, template_name: prefix }, expected_revision: 0, reason: 'Fixture template',
        });
        const item = await json(handleMaster, '/catalogue-items', {
            configuration_id: template.id, specifications: { SIZE: 'S1' }, model: prefix,
        }, 201);
        const bo = randomUUID(), line = randomUUID(), today = indiaDate(new Date());
        const draft = await save(session.access_token, project, bo, {
            name: 'Fixture BO', version: 0, reason: 'Fixture preparation',
            lines: [{ id: line, item_id: item.id, quantity: '2.500', required_date: today, reference: '', notes: '' }],
        });
        const completed = await change(session.access_token, project, bo, 'complete', { version: draft.version, reason: 'Fixture completion' });
        const vendorIds = [randomUUID(), randomUUID(), randomUUID()];
        for (const [index, id] of vendorIds.entries()) {
            const code = prefix + 'V' + index;
            await sql.query("INSERT INTO suppliers(id,code,name,data,is_active) VALUES($1,$2,$2,'{}',true)", [id, code]);
        }
        const base = '/projects/' + project;
        const indent = base + '/purchase-indents/by-type';
        const dispatch = indent + '/dispatch';
        const quotes = base + '/purchase-bo/' + bo + '/group-quotations';
        await json(handlePurchasing, base + '/purchase-indents/project-vendors', {
            version: 0, itemSets: [{ category: category.code, itemIds: [line], vendorIds }],
        });
        const content = {
            specifications: [[{ text: 'Fixture specifications', bold: true, underline: true }]],
            notes: [[{ text: 'Fixture important note', italic: true }]],
            conditions: [[{ text: 'Fixture conditions' }]],
        };
        const legacyGroup = randomUUID(), boPath = base + '/purchase-bo/' + bo;
        const legacy = await json(handlePurchasing, boPath + '/indent-groups', {
            action: 'create', version: 0, bo_version: completed.version, reason: 'Fixture legacy group',
            groups: [{ id: legacyGroup, name: 'Legacy scope', line_ids: [line], vendor_ids: vendorIds }],
        });
        const legacyDocs = boPath + '/indent-documents';
        const generated = await json(handlePurchasing, legacyDocs, {
            action: 'generate', group_id: legacyGroup, vendor_id: vendorIds[0],
            group_version: legacy.record.version, bo_version: completed.version,
        });
        const older = generated.documents[0];
        await json(handlePurchasing, legacyDocs, {
            action: 'send', document_id: older.id, version: older.version, sent_date: today,
            channel: 'Email', dispatch_reference: 'Earlier fixture dispatch',
        });
        assert.equal((await json(handlePurchasing, indent)).reissue_required, true);
        const finalise = { category: category.code, version: 0, content };
        await json(handlePurchasing, indent, finalise, 409);
        await json(handlePurchasing, indent, { ...finalise, reissue: true });
        const historical = (await json(handlePurchasing, legacyDocs)).documents.find((row: { id: string }) => row.id === older.id);
        assert.equal(historical.data.sent_date, today);
        assert.equal(historical.data.is_current, false);
        await json(handlePurchasing, indent, { category: category.code, version: 1, content }, 409);
        const list = await json(handlePurchasing, dispatch);
        assert.equal(list.dispatches.length, 3);
        assert.equal(list.dispatch_complete, false);
        const [first] = list.dispatches;
        const pdf = await request(handlePurchasing, indent + '/pdf?document=' + first.id);
        assert.equal(pdf.status, 200);
        const bytes = Buffer.from(await pdf.arrayBuffer());
        assert.equal(bytes.subarray(0, 5).toString(), '%PDF-');
        if (process.env.EPCERP_INDENT_TEST_PDF) await writeFile(process.env.EPCERP_INDENT_TEST_PDF, bytes);

        const quotation = { version: 0, reason: 'Fixture prices', quote: {
            document_id: first.id, reference: 'Fixture quotation', document_not_provided: true,
            no_document_reason: 'Fixture prices provided manually', received_date: today, valid_until: today,
            currency: 'INR', freight: '0', other_charges: '0', delivery_days: 14,
            delivery_terms: 'Ex works', payment_terms: 'Fixture terms',
            lines: [{ line_id: line, unit_price: '12', discount_pct: '0', tax_pct: '0', compliant: true, warranty_months: 12 }],
        } };
        await json(handlePurchasing, quotes, quotation, 422);
        for (const [index, row] of list.dispatches.entries()) {
            const result = await json(handlePurchasing, dispatch, {
                document_id: row.id, version: row.version, sent_date: today, channel: 'Email', dispatch_reference: 'Fixture dispatch',
            });
            assert.equal(result.dispatch_complete, index === 2);
        }
        assert.equal((await json(handlePurchasing, quotes)).dispatch_complete, true);
        assert.equal((await json(handlePurchasing, quotes, quotation)).record.quotes[0].lines[0].unit_price, '12.00');
        const lumpSum = { ...quotation.quote, amount_mode: 'total_only', total_amount: '25.00',
            lines: [{ ...quotation.quote.lines[0], unit_price: null }] };
        await json(handlePurchasing, quotes, { ...quotation, version: 1, quote: {
            ...lumpSum, lines: [{ ...lumpSum.lines[0], warranty_months: null }],
        } }, 422);
        const secondLine = randomUUID();
        const synthetic = { ...older, data: { ...older.data, sent_date: today,
            lines: [...older.data.lines, { ...older.data.lines[0], id: secondLine }] } };
        const allocations = (other: string) => normalizeMoney(input({ ...quotation, quote: {
            ...lumpSum, lines: [{ ...lumpSum.lines[0], line_total: '10.00' },
                { ...lumpSum.lines[0], line_id: secondLine, line_total: other, warranty_months: 24 }],
        } }).quote);
        assert.throws(() => calculate(allocations('14.99'), synthetic, today), /add up/);
        const calculated = calculate(allocations('15.00'), synthetic, today);
        assert.deepEqual(calculated.lines.map(item => [item.warranty_months, item.landed]), [[12, '10.00'], [24, '15.00']]);
        let quoteVersion = 1;
        for (const row of list.dispatches) {
            const result = await json(handlePurchasing, quotes, {
                ...quotation, version: quoteVersion++, quote: { ...lumpSum, document_id: row.id },
            });
            assert.equal(result.record.quotes.find((entry: { document_id: string }) => entry.document_id === row.id)
                .calculation.lines[0].warranty_months, 12);
        }
        const last = list.dispatches.at(-1)!;
        const incomplete = await json(handlePurchasing, quotes, {
            ...quotation, version: quoteVersion++, quote: { ...lumpSum, document_id: last.id,
                lines: [{ ...lumpSum.lines[0], compliant: false }] },
        });
        assert.equal(incomplete.ready_to_freeze, false);
        assert.match(incomplete.freeze_blockers[0].exclusions.join(' '), /Technical compliance/);
        await json(handlePurchasing, quotes + '/freeze', { version: quoteVersion }, 422);
        const corrected = await json(handlePurchasing, quotes, {
            ...quotation, version: quoteVersion++, quote: { ...lumpSum, document_id: last.id },
        });
        assert.equal(corrected.ready_to_freeze, true);
        await json(handlePurchasing, quotes + '/freeze', { version: quoteVersion });
        const reviewer = randomUUID();
        await sql.query(`INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser)
            SELECT $1,$2,$3,password_hash,true,true FROM auth.users WHERE id=$4`,
        [reviewer, 'qa' + reviewer.replaceAll('-', ''), reviewer + '@fixture.invalid', actor]);
        const approval = await json(handlePurchasing, boPath + '/quotation-approvals/submit', {
            version: 0, reason: 'Fixture Finance handoff',
            proposals: [{ quote_key: first.id + ':2', best_price: true, preferred: true,
                best_price_comment: 'Lowest eligible amount', preferred_comment: 'Confirmed item warranty' }],
        });
        assert.equal(approval.attempts[0].effective_state, 'pending_accounts');
        assert.equal(approval.attempts[0].lines[0].months, 12);
        assert.equal(approval.attempts[0].lines[0].quoted, '25.00');
        const notices = await sql.query("SELECT title FROM communication_notices WHERE recipient_id=$1 AND project_id=$2 AND notice_kind='quotation'", [reviewer, project]);
        assert.match(notices.rows[0]?.title, /Quotation shortlist submitted/);

    } finally {
        await sql.end();
    }
});
