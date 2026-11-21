import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';
import { fork } from 'node:child_process';
import test from 'node:test';
import sharp from 'sharp';
import { handleAuth } from '../../src/server/auth/http';
import { initializeAuth } from '../../src/server/auth/bootstrap';
import { login } from '../../src/server/auth/sessions';
import { pool, transaction } from '../../src/server/db';
import { AppError } from '../../src/server/errors';
import { handleMaster } from '../../src/server/master/http';
import { seal } from '../../src/server/master/documents/files';
import { status, review } from '../../src/server/master/verification/service';
import { execute as saveBudget } from '../../src/server/projects/budgets/service';
import { handlePurchasing } from '../../src/server/purchasing/http';
import { catalogue, permitted } from '../../src/server/purchasing/core/catalogue';
import { workspace as quoteApprovals } from '../../src/server/purchasing/approvals/service';
import { upload as uploadTechnical } from '../../src/server/purchasing/technical/evidence';
import { upload as uploadSignedOrder } from '../../src/server/purchasing/orders/evidence';
import { workspace as orderWorkspace } from '../../src/server/purchasing/orders/service';
import { indiaDate } from '../../src/server/personal-tasks/validation';
import { project as createProject } from '../server-inbox/fixtures';
import { definition } from '../server-master/catalogue-fixtures';

test('purchase workflow enforces vendor verification, advance evidence and client inspection clearance', { skip: !process.env.EPCERP_DATABASE_URL }, async () => {
    assert.match(new URL(process.env.EPCERP_DATABASE_URL!).pathname, /^\/epcerp_auth_qa_purchase_/);
    const sql = pool(), previousStorage = process.env.VTPL_STORAGE_ENDPOINT;
    const storage = fork(new URL('./object-fixture.mjs', import.meta.url), [], { stdio: ['ignore', 'ignore', 'inherit', 'ipc'] });
    try {
        const endpoint = await new Promise<number>((resolve, reject) => {
            storage.once('message', value => resolve((value as { port: number }).port));
            storage.once('error', reject);
        });
        process.env.VTPL_STORAGE_ENDPOINT = '127.0.0.1:' + endpoint;
        await initializeAuth();
        const password = process.env.VTPL_BOOTSTRAP_SUPERADMIN_PASSWORD!;
        const buyer = (await login('superadmin', password)).access_token;
        const buyerID = (await sql.query("SELECT id FROM auth.users WHERE username='superadmin'")).rows[0].id;
        const financeID = randomUUID(), financeName = 'qa' + financeID.replaceAll('-', '').slice(0, 16);
        await sql.query("INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser) SELECT $1,$2,$3,password_hash,true,true FROM auth.users WHERE id=$4", [financeID, financeName, financeName + '@fixture.invalid', buyerID]);
        const finance = (await login(financeName, password)).access_token;
        await sql.query('INSERT INTO organisation_director_designation(id,user_id,version) VALUES($1,$2,1) ON CONFLICT(slot) DO UPDATE SET user_id=EXCLUDED.user_id,is_active=true,version=organisation_director_designation.version+1', [randomUUID(), buyerID]);
        const project = await createProject(buyerID), reason = 'Synthetic vendor release workflow';
        await sql.query("UPDATE projects SET purchase_enabled=true,currency='INR',contract_value=10000 WHERE id=$1", [project]);
        const post = async (handler: typeof handleMaster, path: string, body: object, token = buyer) => {
            const response = await handler(new Request('http://fixture.invalid/api/v1' + path, {
                method: 'POST', headers: { Authorization: 'Bearer ' + token, 'Content-Type': 'application/json' }, body: JSON.stringify(body),
            }));
            const data = await response.json();
            if (!response.ok)
                throw new AppError(response.status, path + ': ' + (typeof data.detail === 'string' ? data.detail : JSON.stringify(data.detail)));
            return data;
        };
        const dashboardResponse = await handlePurchasing(new Request(
            'http://fixture.invalid/api/v1/purchase-fms?search=nonexistent-' + randomUUID() + '&include_scope_total=true',
            { headers: { Authorization: 'Bearer ' + buyer } },
        ));
        assert.equal(dashboardResponse.status, 200);
        const dashboard = await dashboardResponse.json();
        assert.deepEqual(dashboard.items, []);
        assert.equal(dashboard.total, 0);
        assert.ok(dashboard.scope_total >= 1);
        const master = (path: string, body: object) => post(handleMaster, path, { reason, ...body });
        const purchase = (path: string, body: object, token = buyer) => post(handlePurchasing, '/projects/' + project + path, { reason, ...body }, token);
        const prefix = 'QAVG' + randomUUID().replaceAll('-', '').slice(0, 8).toUpperCase();
        const unit = await master('/categories/units', { code: prefix, name: prefix });
        const category = await master('/categories/categories', { code: prefix + 'C', name: prefix, unit_id: unit.id });
        const type = await master('/categories/subcategories', { code: prefix + 'T', name: prefix, category_id: category.id });
        const template = await master('/configuration-templates/' + type.id + '/publish', { definition: { ...definition(), subcategory_code: type.code, template_name: prefix }, expected_revision: 0 });
        const item = await post(handleMaster, '/catalogue-items', { configuration_id: template.id, specifications: { SIZE: 'S1' }, model: prefix });
        assert.equal(item.manufacturer_id, null);
        const checkItem = () => transaction(async db => permitted(db, project, await catalogue(db, item.id)));
        assert.equal((await checkItem()).id, item.id);
        const rule = randomUUID();
        await sql.query(
            'INSERT INTO client_purchase_rules(id,client_id,data,is_active,version) SELECT $1,client_id,$2,true,1 FROM projects WHERE id=$3',
            [rule, { project_id: project, scope_code: category.code, mode: 'approved_only', allowed: [], excluded: [], effective_from: '2000-01-01' }, project],
        );
        try {
            await assert.rejects(checkItem(), /Client manufacturer restriction/);
        } finally {
            await sql.query('UPDATE client_purchase_rules SET is_active=false,version=version+1 WHERE id=$1', [rule]);
        }
        const vendors = await Promise.all([1, 2, 3].map(i => master('/suppliers', { code: prefix + i, name: 'QA vendor ' + prefix + i, data: {} })));
        const vendor = vendors[0];
        assert.equal((await status(finance, vendor.id)).can_generate_purchase_order, false);

        const modeRequest = async (mode?: string) => {
            const response = await handlePurchasing(new Request('http://fixture.invalid/api/v1/purchase-fms/application-settings', {
                method: mode ? 'PATCH' : 'GET',
                headers: {Authorization: 'Bearer ' + buyer, 'Content-Type': 'application/json'},
                body: mode ? JSON.stringify({item_list_mode: mode}) : undefined,
            }));
            assert.equal(response.status, 200);
            return (await response.json()).item_list_mode;
        };
        const legacyProject = await createProject(buyerID);
        await sql.query("UPDATE projects SET purchase_enabled=true,currency='INR' WHERE id=$1", [legacyProject]);
        const alternateItem = await post(handleMaster, '/catalogue-items', { configuration_id: template.id, specifications: { SIZE: 'S2' }, model: prefix });
        const legacyDate = indiaDate(new Date(Date.now() + 30 * 86400000));
        const createPreparation = (id: string, itemID = item.id) => post(handlePurchasing, '/projects/' + legacyProject + '/purchase-bo/' + id, {
            version: 0, name: 'Purchase requirements', reason, lines: [{id: randomUUID(), item_id: itemID, quantity: '1', required_date: legacyDate}],
        });
        assert.equal(await modeRequest(), 'single');
        const firstPreparation = await createPreparation(randomUUID());
        await assert.rejects(createPreparation(randomUUID()), /already has purchase items/);
        assert.equal(await modeRequest('multiple'), 'multiple');
        const secondPreparation = await createPreparation(randomUUID(), alternateItem.id);
        assert.equal(await modeRequest('single'), 'single');
        const savedPreparations = await handlePurchasing(new Request('http://fixture.invalid/api/v1/projects/' + legacyProject + '/purchase-bo', {headers: {Authorization: 'Bearer ' + buyer}}));
        assert.equal(savedPreparations.status, 200);
        const register = await savedPreparations.json();
        assert.equal(register.item_list_mode, 'single');
        assert.deepEqual(register.lists.map((entry: {id: string})=>entry.id), [firstPreparation.id, secondPreparation.id]);
        await assert.rejects(createPreparation(randomUUID()), /already has purchase items/);

        const boID = randomUUID(), lineID = randomUUID(), groupID = randomUUID(), boPath = '/purchase-bo/' + boID;
        const date = indiaDate(new Date()), due = indiaDate(new Date(Date.now() + 30 * 86400000));
        let bo = await purchase(boPath, { name: 'Synthetic BO', version: 0, lines: [{ id: lineID, item_id: item.id, quantity: '1', required_date: due, reference: '', notes: '' }] });
        const managerID = randomUUID(), managerName = 'qa' + managerID.replaceAll('-', '').slice(0, 16);
        const grants = ['projects.purchase_workflow.view', 'projects.purchase_workflow.read', 'projects.purchase_bo.read'];
        const createRole = (name: string, permission_codes: string[]) => post(handleAuth, '/roles', { name, permission_codes });
        const role = await createRole(managerName, grants);
        const get = async (handler: typeof handleMaster, path: string, token = buyer) => {
            const response = await handler(new Request('http://fixture.invalid/api/v1' + path, { headers: { Authorization: 'Bearer ' + token } }));
            assert.equal(response.status, 200);
            return response.json();
        };
        const permissionCatalogue = await get(handleAuth, '/roles/permissions') as {code: string; description: string}[];
        const completion = 'projects.purchase_bo.complete';
        assert.match(permissionCatalogue.find(permission => permission.code === completion)!.description, /Finalise/);
        const broadRole = await createRole(managerName + '-broad', permissionCatalogue.map(permission => permission.code));
        assert.ok(broadRole.permissions.length > 100);
        await sql.query("INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser) SELECT $1,$2,$3,password_hash,true,false FROM auth.users WHERE id=$4", [managerID, managerName, managerName + '@fixture.invalid', buyerID]);
        await sql.query('INSERT INTO auth.user_roles(user_id,role_id) VALUES($1,$2)', [managerID, role.id]);
        await sql.query('INSERT INTO purchase_fms_workspaces(project_id,purchase_manager_id) VALUES($1,$2) ON CONFLICT(project_id) DO UPDATE SET purchase_manager_id=EXCLUDED.purchase_manager_id', [project, managerID]);
        const managerToken = (await login(managerName, password)).access_token;
        const registerPath = '/projects/' + project + '/purchase-bo';
        const managerRegister = () => get(handlePurchasing, registerPath, managerToken);
        const changeRegister = (action: string) => purchase('/purchase-bo/register-' + action, { records: [{id: bo.id, version: bo.version}] }, managerToken);
        assert.equal((await managerRegister()).can_complete, false);
        await assert.rejects(changeRegister('complete'), { status: 403 });
        const roles = await get(handleAuth, '/roles') as {id: string; user_count: number}[];
        assert.equal(roles.find(saved => saved.id === role.id)?.user_count, 1);
        assert.equal(roles.find(saved => saved.id === broadRole.id)?.user_count, 0);
        const updateRole = async (id: string, editVersion: number, permission_codes: string[]) => {
            const response = await handleAuth(new Request('http://fixture.invalid/api/v1/roles/' + id, {
                method: 'PATCH', headers: { Authorization: 'Bearer ' + buyer, 'Content-Type': 'application/json' },
                body: JSON.stringify({ expected_version: editVersion, permission_codes }),
            }));
            const data = await response.json();
            assert.equal(response.status, 200, JSON.stringify(data));
            return data;
        };
        const reduced = broadRole.permissions.filter((code: string) => code !== completion);
        assert.deepEqual((await updateRole(broadRole.id, broadRole.edit_version, reduced)).permissions, reduced);
        assert.ok((await updateRole(role.id, role.edit_version, [...grants, completion])).permissions.includes(completion));
        const permissions = await managerRegister();
        assert.equal(permissions.can_write, false);
        assert.equal(permissions.can_complete, true);
        assert.equal(permissions.can_manage_lists, true);
        bo = (await changeRegister('complete')).lists[0];
        assert.equal(bo.completed_by_id, managerID);
        assert.equal(bo.preparer_id, buyerID);
        const demandID = bo.lines[0].demand_id;
        bo = (await changeRegister('reopen')).lists[0];
        assert.equal(bo.state, 'draft');
        bo = await purchase(boPath + '/complete', { version: bo.version }, managerToken);
        assert.equal(bo.lines[0].demand_id, demandID);
        const groupsPath = boPath + '/indent-groups';
        let grouping = await purchase(groupsPath, { action: 'create_group', version: 0, bo_version: bo.version, groups: [{ id: groupID, name: 'Synthetic group', line_ids: [lineID], vendor_ids: [] }] });
        grouping = await purchase(groupsPath, { action: 'set_vendors', version: grouping.record.version, bo_version: bo.version, group_id: groupID, vendor_ids: vendors.map(v => v.id) });
        const documentsPath = '/projects/' + project + boPath + '/indent-documents';
        let documents = await post(handlePurchasing, documentsPath, { action: 'generate', group_id: groupID, vendor_id: vendor.id, group_version: grouping.record.version, bo_version: bo.version, version: 0 });
        const doc = documents.documents[0];
        assert.deepEqual(doc.data.terms, { source: 'vendor_master', notes: [], conditions: [], delivery_basis: '' });
        assert.equal(doc.data.vendor.code, vendor.code);
        documents = await post(handlePurchasing, documentsPath, { action: 'send', document_id: doc.id, version: doc.version, sent_date: date, channel: 'manual', dispatch_reference: 'Synthetic recorded RFQ dispatch' });
        assert.equal(documents.documents[0].data.sent_date, date);

        const quotePath = boPath + '/group-quotations';
        const changeFreeze = (action: 'freeze' | 'unfreeze', version: number) =>
            post(handlePurchasing, '/projects/' + project + quotePath + '/' + action, {version});
        const readPurchase = async (path: string) => {
            const response = await handlePurchasing(new Request('http://fixture.invalid/api/v1' + path, {headers: {Authorization: 'Bearer ' + buyer}}));
            assert.equal(response.status, 200);
            return response.json();
        };
        const projectProgress = async () => (await readPurchase('/purchase-fms/projects/' + project)).project;
        const firstQuote = {
            document_id: doc.id, reference: 'QA quote', document_not_provided: true, no_document_reason: 'Synthetic manually received quotation',
            received_date: date, valid_until: due, currency: 'INR', freight: '0', other_charges: '0', delivery_days: 14, delivery_terms: 'Ex works', payment_terms: 'Net 30',
            lines: [{line_id: lineID, unit_price: '10', discount_pct: '0', tax_pct: '18', compliant: true, warranty_months: 12}],
        };
        const remaining = [];
        for (const other of vendors.slice(1)) {
            const currentGroups = await readPurchase('/projects/' + project + groupsPath);
            const generated = await post(handlePurchasing, documentsPath, {action: 'generate', group_id: groupID, vendor_id: other.id, group_version: currentGroups.record.version, bo_version: bo.version, version: 0});
            remaining.push(generated.documents.find((candidate: {vendor_id: string}) => candidate.vendor_id === other.id));
        }
        const blocked = await readPurchase('/projects/' + project + quotePath);
        assert.equal(blocked.dispatch_complete, false);
        assert.equal(blocked.documents.filter((entry: {data: {sent_date: string | null}})=>!entry.data.sent_date).length, 2);
        assert.equal((await projectProgress()).completed_steps.includes(2), false);
        await assert.rejects(changeFreeze('freeze', 0), /Complete vendor indent dispatch/);
        await assert.rejects(purchase(quotePath, {version: 0, quote: firstQuote}), /Complete vendor indent dispatch/);
        for (const indent of remaining)
            await post(handlePurchasing, documentsPath, {action: 'send', document_id: indent.id, version: indent.version, sent_date: date, channel: 'manual', dispatch_reference: 'Synthetic recorded RFQ dispatch'});
        assert.equal((await projectProgress()).completed_steps.includes(2), true);
        let receipt = await purchase(quotePath, {version: 0, quote: firstQuote});
        assert.equal(receipt.comparisons[0].eligible, true);
        await assert.rejects(changeFreeze('freeze', receipt.record.version), /every current vendor indent/);
        for (const indent of remaining) {
            receipt = await purchase(quotePath, {version: receipt.record.version, quote: {
                ...firstQuote, document_id: indent.id, reference: 'QA overall quote',
                amount_mode: 'total_only', total_amount: '200', lines: [{line_id: lineID, unit_price: null, line_total: '200', compliant: true, warranty_months: 12}],
            }});
        }
        assert.equal(receipt.ready_to_freeze, true);
        assert.equal((await projectProgress()).completed_steps.includes(3), false);
        const frozen = await changeFreeze('freeze', receipt.record.version);
        assert.equal(frozen.frozen, true);
        assert.equal((await readPurchase('/projects/' + project + quotePath)).frozen, true);
        assert.equal((await projectProgress()).completed_steps.includes(3), true);
        await assert.rejects(changeFreeze('unfreeze', receipt.record.version), /Quotations changed/);
        await assert.rejects(purchase(quotePath, {version: frozen.record.version, quote: {
            document_id: doc.id, reference: 'Frozen revision', document_not_provided: true, no_document_reason: 'Synthetic revision',
            received_date: date, valid_until: due, currency: 'INR', freight: '0', other_charges: '0', delivery_days: 14, delivery_terms: 'Ex works', payment_terms: 'Net 30',
            lines: [{line_id: lineID, unit_price: '10', discount_pct: '0', tax_pct: '18', compliant: true, warranty_months: 12}],
        }}), /Unfreeze negotiations/);
        const reopened = await changeFreeze('unfreeze', frozen.record.version);
        assert.equal(reopened.frozen, false);
        assert.equal((await projectProgress()).completed_steps.includes(3), false);
        assert.equal((await projectProgress()).next_step, 'PUR-03');
        assert.deepEqual(reopened.history.slice(0, 2).map((entry: {action: string}) => entry.action), ['negotiation_unfrozen', 'negotiation_frozen']);

        const approvalPath = boPath + '/quotation-approvals', quoteKey = doc.id + ':1';
        await assert.rejects(purchase(approvalPath + '/submit', {version: 0, quote_key: quoteKey}), /Freeze vendor quotations/);
        await changeFreeze('freeze', reopened.record.version);
        let approval = await purchase(approvalPath + '/submit', { version: 0, quote_key: quoteKey });
        assert.equal(approval.attempts[0].effective_state, 'pending_accounts');
        const head = randomUUID();
        const budget = await saveBudget(finance, project, { action: 'save', version: 0, reason, data: {
            total: '1000', currency: 'INR', owner_id: buyerID, notes: reason,
            heads: [{ id: head, parent_id: null, name: 'Purchase', kind: 'purchase', amount: '1000', owner_id: buyerID, is_active: true }],
            allocations: [{ id: randomUUID(), bo_id: boID, line_id: lineID, head_id: head, amount: '1000', owner_id: buyerID, is_active: true }],
        } });
        await saveBudget(finance, project, { action: 'approve', version: budget.record!.version, reason });
        approval = await purchase(approvalPath + '/review', {
            version: approval.version, attempt_id: approval.attempts[0].id, selected_quote_key: quoteKey,
            decisions: [{ line_id: lineID, decision: 'accept', comment: reason }],
        }, finance);
        assert.equal(approval.ready, true);
        assert.equal((await status(finance, vendor.id)).can_generate_purchase_order, false);

        // LOI and draft PO still use their ordinary supplier contact and address fields.
        const profile = await handleMaster(new Request('http://fixture.invalid/api/v1/suppliers/' + vendor.id, {
            method: 'PATCH', headers: { Authorization: 'Bearer ' + buyer, 'Content-Type': 'application/json' },
            body: JSON.stringify({ version: vendor.version, name: vendor.name, data: { qualification: 'unverified', registration_reference: 'QA registration', contact: 'QA contact', location: 'QA address' }, reason }),
        }));
        assert.equal(profile.status, 200);
        const indentData = { currency: 'INR', doer_id: buyerID, resolver_id: financeID, lines: [{ demand_id: bo.lines[0].demand_id, quantity: '1', budget: '100', full_kit: 'QA full kit' }], documents: 'Synthetic supporting references' };
        let indent = await purchase('/purchase-indents', { data: indentData, version: 1 });
        for (const action of ['submit', 'review', 'release'])
            indent = await purchase('/purchase-indents/' + indent.id + '/actions', { action, version: indent.version, evidence: reason, line_ids: action === 'release' ? [bo.lines[0].demand_id] : [] });
        let workflow = await purchase('/purchase-workflow', { indent_id: indent.id, demand_id: bo.lines[0].demand_id });
        const packageID = workflow.package.id, packagePath = '/purchase-workflow/' + packageID;
        workflow = await purchase(packagePath + '/assignments', { version: workflow.package.version, rows: [3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15].map(number => ({ number, team: 'QA', doer_id: buyerID, resolver_id: financeID, planned_date: due })) });
        const sourcingPath = packagePath + '/sourcing';
        let sourcing = await purchase(sourcingPath, { version: 0, action: 'configure', approver_id: financeID });
        const approved = sourcing.approved_evidence[0], price = approved.price;
        sourcing = await purchase(sourcingPath, { version: 1, action: 'rfq', supplier_id: vendor.id, due_date: due, rfq_terms: 'QA terms' });
        const offer = sourcing.case.offers[0].id;
        await purchase(sourcingPath, { version: 2, action: 'dispatch', offer_id: offer, sent_date: date, channel: 'manual', dispatch_reference: 'QA RFQ' });
        await purchase(sourcingPath, { version: 3, action: 'quote', offer_id: offer, quote_reference: approved.quote_reference, evidence_reference: approved.evidence_reference, valid_until: due, pricing_mode: price.pricing_mode, unit_price: price.unit_price, discount: price.discount, tax: price.tax, freight: price.allocated_charges, other: '0', delivery_days: 14, payment_terms: 'Net 30', warranty: '12 months', compliant: true, compliance_notes: reason });
        await purchase(sourcingPath, { version: 4, action: 'propose', offer_id: offer, loi_terms: 'QA LOI terms' });
        await purchase(sourcingPath, { version: 5, action: 'decide', approve: true }, finance);
        sourcing = await purchase(sourcingPath, { version: 6, action: 'release', sent_date: date, channel: 'manual', dispatch_reference: 'QA LOI' });
        assert.equal(sourcing.case.state, 'released');

        const technicalPath = packagePath + '/technical', pdf = Buffer.from('%PDF-1.7\nSynthetic evidence\n%%EOF');
        let tech = await purchase(technicalPath, { version: 0, action: 'configure' });
        const upload = (purpose: string) => uploadTechnical(buyer, project, packageID, { version: tech.case.version, purpose, reason, name: 'qa.pdf', media_type: 'application/pdf', data: pdf });
        const technicalDoc = await upload('document');
        for (const doc of tech.case.documents.filter((doc: { required: boolean }) => doc.required))
            tech = await purchase(technicalPath, { version: tech.case.version, action: 'document', document: { ...doc, status: 'received', received_date: date, file_id: technicalDoc.id, notes: reason } });
        tech = await purchase(technicalPath, { version: tech.case.version, action: 'team_review', approve: true, remarks: reason }, finance);
        const transmittal = await upload('transmittal');
        tech = await purchase(technicalPath, { version: tech.case.version, action: 'submit', submission: { reference: 'QA transmittal', date, target_date: due, authority: 'QA client', file_id: transmittal.id } });
        const stamped = await upload('client_approval');
        tech = await purchase(technicalPath, { version: tech.case.version, action: 'client_approval', approval: { status: 'approved', reference: 'QA client approval', date, approver_name: 'QA client', approver_designation: 'Engineer', file_id: stamped.id, closure_file_id: '', remarks: reason }, qap_approved: true, conditions_closed: false });
        assert.equal(tech.ready, true);

        const orderPath = packagePath + '/purchase-orders';
        let po = await purchase(orderPath, { version: 0, action: 'prepare', details: { date, issuer_name: 'QA EPC', issuer_address: 'QA office', issuer_tax_reference: '', issuer_registration: '', issuer_email: '', bill_to: 'QA buyer', delivery_address: 'QA site' } });
        assert.equal(po.order.source_snapshot.supplier_verification, '');
        po = await purchase(orderPath, { version: po.order.version, action: 'submit_client', submission: { date, reference: 'QA PO submission' } });
        const signed = await uploadSignedOrder(buyer, project, packageID, po.order.version, 'qa-signed.pdf', 'application/pdf', pdf);
        po = await purchase(orderPath, { version: po.order.version, action: 'client_decision', client_decision: { status: 'approved', reference: 'QA signed PO', date, signatory: 'QA client', designation: 'Engineer', file_id: signed.id } });
        assert.equal(po.actions.includes('release'), false);
        assert.match(po.release_blocker, /Finance must verify.*before PO release/);
        const release = { version: po.order.version, action: 'release' };
        const sequence = () => sql.query('SELECT sum(last_value)::text AS total FROM purchase_order_sequences').then(result => result.rows[0].total);
        const beforeRelease = await sequence();
        await assert.rejects(purchase(orderPath, release), /Finance must verify.*before PO release/);
        assert.equal(await sequence(), beforeRelease);

        const evidence = seal(pdf), decisions = [];
        for (const kind of ['pan', 'cheque']) {
            const document_id = randomUUID();
            await sql.query("INSERT INTO supplier_documents(id,supplier_id,doc_type,title,original_filename,content_type,byte_size,encrypted_content,sha256,is_active,source_id,source_created_at,source_updated_at) VALUES($1,$2,$3::text,$3::text,$3::text||'.pdf','application/pdf',$4,$5,$6,true,$1,now(),now())", [document_id, vendor.id, kind, pdf.length, evidence.encrypted, evidence.sha256]);
            decisions.push({ document_id, decision: 'accept', comment: reason });
        }
        await assert.rejects(purchase(orderPath, release), /Finance must verify.*before PO release/);
        const pending = await status(finance, vendor.id);
        const verified = await review(finance, vendor.id, { version: 0, fingerprint: pending.fingerprint, decisions, reason });
        assert.equal(verified.can_generate_purchase_order, true);
        assert.equal((await quoteApprovals(buyer, project, boID)).ready, true);
        assert.ok((await orderWorkspace(buyer, project, packageID)).actions.includes('release'));
        const draftSource = po.order.source_snapshot, clientApproval = po.order.client_approval;
        po = await purchase(orderPath, release);
        assert.equal(po.order.state, 'released');
        assert.deepEqual(po.order.source_snapshot, draftSource);
        assert.deepEqual(po.order.client_approval, clientApproval);
        assert.equal(po.order.supplier_verification_at_release, verified.fingerprint);

        const acceptancePath = packagePath + '/acceptance';
        const response = { status: 'accepted', accepted_date: date, signatory_name: 'QA vendor', document_reference: 'QA acknowledgement', remarks: '' };
        await assert.rejects(purchase(acceptancePath, { version: 0, action: 'record', response }), /dispatch date/);
        await purchase(acceptancePath, { version: 0, action: 'record', response: {
            ...response, dispatched_date: date, dispatch_reference: 'QA dispatch',
        } });
        const inspectionPath = packagePath + '/inspection';
        const inspection = { status: 'not_scheduled', request_date: date, request_reference: 'QA inspection request', witness_points: [] };
        await assert.rejects(purchase(inspectionPath, { version: 0, action: 'record', inspection }), /source|advance/i);

        const advancePath = packagePath + '/advances';
        let advance = await purchase(advancePath, { version: 0, action: 'prepare', details: {
            kind: 'payment', request_no: 'QA advance', request_date: date, advance_pct: '20',
            proforma_invoice_ref: 'QA invoice', proforma_invoice_date: date, bank_account_ref: 'QA supplier account', notes: '',
        } });
        advance = await purchase(advancePath, { version: advance.record.version, action: 'submit' });
        assert.equal(advance.record.state, 'requested');
        const notice = await sql.query("SELECT 1 FROM communication_notices WHERE recipient_id=$1 AND notice_kind='purchase_advance'", [financeID]);
        assert.equal(notice.rowCount, 1);
        advance = await purchase(advancePath, { version: advance.record.version, action: 'approve' }, finance);
        assert.equal(advance.record.state, 'approved');
        const disbursement = { version: advance.record.version, action: 'disburse', reason,
            payment: { voucher_ref: 'QA voucher', disbursed_date: date, notes: '' } };
        await assert.rejects(purchase(advancePath, disbursement, finance), /payment confirmation screenshot/);

        const stageEvidence = async (path: string, command: object, file: Buffer, name: string, mime: string, token = buyer) => {
            const form = new FormData();
            form.set('reason', JSON.stringify({ reason, ...command }));
            form.set('file', new Blob([new Uint8Array(file)], { type: mime }), name);
            const result = await handlePurchasing(new Request('http://fixture.invalid/api/v1/projects/' + project + path + '/evidence', {
                method: 'POST', headers: { Authorization: 'Bearer ' + token }, body: form,
            }));
            const body = await result.json();
            assert.equal(result.status, 200, JSON.stringify(body));
            return body;
        };
        const png = await sharp({ create: { width: 8, height: 8, channels: 3, background: '#ffffff' } }).png().toBuffer();
        advance = await stageEvidence(advancePath, disbursement, png, 'payment.png', 'image/png', finance);
        assert.equal(advance.record.state, 'disbursed');
        assert.equal(advance.record.payment_evidence.name, 'payment.png');

        const scheduled = { ...inspection, status: 'scheduled', call_date: date, call_notice_ref: 'QA vendor call',
            scheduled_date: date, client_confirmed_date: date, client_confirmation_reference: 'QA client notice',
            inspection_agency: 'client_consultant', factory_location: 'QA factory' };
        const passed = { ...scheduled, status: 'cleared', actual_date: date, inspector_name: 'QA inspector',
            findings: 'QA inspection passed', no_witness_points_reason: 'QA approved QAP has no mandatory witness points' };
        await assert.rejects(purchase(inspectionPath, { version: 0, action: 'record', inspection: passed }), /Save the confirmed inspection schedule/);
        const scheduledRecord = await purchase(inspectionPath, { version: 0, action: 'record', inspection: scheduled });
        const inspectionResult = await purchase(inspectionPath, { version: scheduledRecord.record.version, action: 'record', inspection: passed });
        assert.equal(inspectionResult.record.inspection.status, 'cleared');

        await purchase(packagePath + '/inspection-report', { version: 0, action: 'record', report: {
            status: 'issued', ir_number: 'QA IR', irn_number: 'QA IRN', issue_date: date,
            issuing_authority: 'QA inspector', cleared_quantity: '1', file_name: 'qa-ir.pdf',
        } });
        const qualityPath = packagePath + '/quality';
        let qualityVersion = 0;
        for (const certificate_code of ['TC', 'TR', 'MTC', 'RTC', 'Warranty']) {
            const recorded = await purchase(qualityPath, { version: qualityVersion, action: 'record', certificate_code,
                details: { cert_number: 'QA ' + certificate_code, issued_by: 'QA laboratory', valid_until: due, file_name: 'qa-quality.pdf' } });
            const checked = await purchase(qualityPath, { version: recorded.record.version, action: 'verify', certificate_code }, finance);
            qualityVersion = checked.record.version;
        }
        const clearancePath = packagePath + '/client-clearance';
        const clearance = { status: 'cleared', mdcc_ref: 'QA MDCC', clearance_date: date, cleared_by_authority: 'QA client', remarks: '', file_name: 'qa-mdcc.pdf' };
        await assert.rejects(purchase(clearancePath, { version: 0, action: 'record', clearance }), /clearance certificate/);
        const cleared = await stageEvidence(clearancePath, { version: 0, action: 'record', clearance }, pdf, 'qa-mdcc.pdf', 'application/pdf');
        assert.equal(cleared.record.clearance.status, 'cleared');


    } finally {
        storage.kill();
        if (previousStorage === undefined)
            delete process.env.VTPL_STORAGE_ENDPOINT;
        else
            process.env.VTPL_STORAGE_ENDPOINT = previousStorage;
        await sql.end();
    }
});
