import assert from "node:assert/strict";
import { randomUUID } from "node:crypto";
import test from "node:test";
import { initializeAuth } from "../../src/server/auth/bootstrap";
import { login } from "../../src/server/auth/sessions";
import { createRole, updateRole } from "../../src/server/auth/administration";
import { hashPassword } from "../../src/server/auth/security";
import { pool } from "../../src/server/db";
import { handleMaster } from "../../src/server/master/http";
import { definition } from "./catalogue-fixtures";
test("native catalogue preserves owned records, revisions, identities and atomic audit history", { skip: !process.env.EPCERP_DATABASE_URL }, async (t) => {
    assert.match(new URL(process.env.EPCERP_DATABASE_URL!).pathname, /\/epcerp_(auth|master)_qa_/u, "Refusing to mutate an application database");
    try {
        await initializeAuth();
        const token = (await login("superadmin", process.env.VTPL_BOOTSTRAP_SUPERADMIN_PASSWORD!)).access_token, prefix = `QA${randomUUID().replaceAll("-", "").slice(0, 9).toUpperCase()}`;
        async function call(method: string, path: string, body?: unknown, status = 200, access = token) {
            const r = await handleMaster(new Request("http://localhost/api/v1" + path, { method, headers: { Authorization: `Bearer ${access}` }, body: body === undefined ? undefined : JSON.stringify(body) }));
            const v = await r.json();
            assert.equal(r.status, status, JSON.stringify(v));
            return v;
        }
        const role = await createRole(token, { name: prefix + " reader", description: "Synthetic catalogue reader", permission_codes: ["settings.items.read"] }), readerID = randomUUID();
        await pool().query("INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser) VALUES($1,$2,$3,$4,true,false)", [readerID, prefix, prefix + "@fixture.invalid", await hashPassword("Catalogue-Reader1!")]);
        await pool().query("INSERT INTO auth.user_roles(user_id,role_id) VALUES($1,$2)", [readerID, role.id]);
        const readerToken = (await login(prefix, "Catalogue-Reader1!")).access_token;
        let unit: Awaited<ReturnType<typeof call>>, category: typeof unit, type: typeof unit, make: typeof unit, template: typeof unit, item: typeof unit;
        const def = () => ({ ...definition(), subcategory_code: type.code, template_name: prefix + " configuration" });
        await t.test("permission checks use existing granular grants", async () => {
            await call("GET", "/catalogue-items", undefined, 401, "");
            await call("GET", "/catalogue-items/references", undefined, 200, readerToken);
            for (const path of ["/categories", "/manufacturers", "/configuration-templates/" + randomUUID()])
                await call("GET", path, undefined, 403, readerToken);
            await call("POST", "/catalogue-items", { configuration_id: randomUUID(), manufacturer_id: randomUUID(), specifications: {} }, 403, readerToken);
        });
        await t.test("units, categories and subcategories retain parent relationships and audit history", async () => {
            unit = await call("POST", "/categories/units", { code: prefix, name: prefix + " Unit" }, 201);
            assert.equal((await pool().query("SELECT reason FROM category_catalogue_changes WHERE subject_id=$1 AND action='create'", [unit.id])).rows[0].reason, "Base unit created");
            category = await call("POST", "/categories/categories", { code: prefix + "C", name: prefix + " Category", unit_id: unit.id }, 201);
            assert.equal((await pool().query("SELECT reason FROM category_catalogue_changes WHERE subject_id=$1 AND action='create'", [category.id])).rows[0].reason, "Category created");
            type = await call("POST", "/categories/subcategories", { code: prefix + "T", name: prefix + " Type", category_id: category.id }, 201);
            assert.equal((await pool().query("SELECT reason FROM category_catalogue_changes WHERE subject_id=$1 AND action='create'", [type.id])).rows[0].reason, "Subcategory created");
            await call("PATCH", `/categories/units/${unit.id}`, { code: unit.code, name: unit.name + " corrected", version: unit.version }, 422);
            await call("PATCH", `/categories/subcategories/${type.id}`, { code: type.code, name: type.name + " corrected", version: type.version }, 422);
            assert.equal(type.unit_id, unit.id);
            assert.equal(type.category_id, category.id);
            assert.equal(type.configuration.family_code, type.code);
            const all = await call("GET", "/categories");
            assert.ok(all.categories.some((v: {
                id: string;
            }) => v.id === category.id));
            assert.equal(all.categories.find((v: {
                id: string;
            }) => v.id === category.id).legacy_source, undefined);
            const detail = await call("GET", `/categories/detail/subcategory/${type.id}`);
            assert.equal(detail.unit_code, unit.code);
            const page = await call("GET", `/categories/browse?kind=subcategory&category_id=${category.id}&search=${prefix}&size=20`);
            assert.equal(page.total, 1);
            assert.equal(page.rows[0].id, type.id);
            assert.equal(typeof page.rows[0].child_count, "number");
            const old = category;
            await call("PATCH", `/categories/categories/${category.id}`, { code: category.code, name: category.name + " corrected", version: category.version }, 422);
            category = await call("PATCH", `/categories/categories/${category.id}`, { code: category.code, name: category.name + " corrected", reason: "QA name correction", version: category.version });
            assert.equal(category.version, 2);
            assert.equal(category.created_at, old.created_at);
            assert.equal(category.created_by_id, old.created_by_id);
            await call("PATCH", `/categories/categories/${category.id}`, { code: category.code, name: category.name, reason: "stale", version: 1 }, 409);
            await call("POST", "/categories/subcategories", { code: type.code, name: prefix + " duplicate", category_id: category.id, reason: "QA duplicate" }, 422);
            assert.equal((await pool().query("SELECT count(*) FROM category_catalogue_changes WHERE subject_id=ANY($1::uuid[])", [[unit.id, category.id, type.id]])).rows[0].count, "4");
            assert.equal((await pool().query("SELECT count(*) FROM business_events WHERE source_module='category_catalogue' AND entity_id=ANY($1::uuid[])", [[unit.id, category.id, type.id]])).rows[0].count, "4");
        });
        await t.test("template publication and preview preserve Go fingerprints, controlled options and reasons", async () => {
            await call("GET", `/configuration-templates/${type.id}`, undefined, 404);
            template = await call("POST", `/configuration-templates/${type.id}/publish`, { definition: def(), expected_revision: 0, reason: "QA reviewed configuration" });
            assert.equal(template.revision, 1);
            assert.equal(template.source_fingerprint, undefined);
            const preview = await call("POST", `/configuration-templates/${type.id}/preview`, { values: { SIZE: "S1" } });
            assert.equal(preview.description, "Size 10 mm");
            assert.match(preview.engineering_key, /^[a-f0-9]{64}$/);
            await call("POST", `/configuration-templates/${type.id}/preview`, { values: { SIZE: "invalid" } }, 422);
            await call("POST", `/configuration-templates/${type.id}/preview`, { values: { SIZE: "__NA__" } });
            const newer = { ...def(), fields: [{ ...def().fields[0], options: [...def().fields[0].options, { code: "S3", label: "30", sort_order: 3, is_active: true }] }] };
            template = await call("POST", `/configuration-templates/${type.id}/publish`, { definition: newer, expected_revision: 1, reason: "QA add controlled option" });
            assert.equal(template.revision, 2);
            await call("POST", `/configuration-templates/${type.id}/publish`, { definition: newer, expected_revision: 1, reason: "QA stale editor" }, 409);
            await call("POST", `/configuration-templates/${type.id}/publish`, { definition: def(), expected_revision: 2, reason: "QA remove issued code" }, 422);
            assert.equal((await pool().query("SELECT count(*) FROM configuration_templates WHERE subcategory_id=$1", [type.id])).rows[0].count, "2");
            const count = (await pool().query("SELECT count(*) FROM template_publications WHERE template_id=$1", [template.id])).rows[0].count;
            assert.equal(count, "1");
        });
        await t.test("manufacturers keep eligibility, optimistic versions and history", async () => {
            make = await call("POST", "/manufacturers", { code: prefix, name: prefix + " Make", data: { eligible: [category.code], notes: "QA" } }, 201);
            assert.equal((await pool().query("SELECT reason FROM manufacturers_events WHERE subject_id=$1 AND action='create'", [make.id])).rows[0].reason, "Manufacturer created");
            await call("POST", "/manufacturers", { code: prefix + "X", name: make.name.toUpperCase(), data: { eligible: [category.code] }, reason: "QA duplicate" }, 422);
            await call("POST", "/manufacturers", { code: prefix + "Y", name: prefix + " invalid eligibility", data: { eligible: ["MISSING"] }, reason: "QA missing" }, 422);
            const choices = await call("GET", `/manufacturers/eligibility?codes=${category.code},${type.code}`);
            assert.equal(choices.total, 2);
            assert.ok(choices.rows.every((v: {
                is_active: boolean;
            }) => v.is_active));
            const old = make;
            await call("PATCH", `/manufacturers/${make.id}/profile`, { name: make.name + " corrected", data: make.data, version: make.version }, 422);
            make = await call("PATCH", `/manufacturers/${make.id}/profile`, { name: make.name + " corrected", data: { eligible: [category.code, type.code], notes: "Edited" }, version: make.version, reason: "QA profile correction" });
            assert.equal(make.code, old.code);
            assert.equal(make.created_at, old.created_at);
            await call("PATCH", `/manufacturers/${make.id}/profile`, { name: make.name, data: make.data, version: 1, reason: "QA stale" }, 409);
            assert.equal((await call("GET", `/manufacturers/${make.id}/history`)).length, 2);
            const path = `/manufacturers/${make.id}/profile`;
            const update = { name: make.name, data: { ...make.data, notes: "Status and notes updated together" }, version: make.version };
            await call("PATCH", path, { ...update, is_active: false }, 422);
            make = await call("PATCH", path, { ...update, is_active: false, reason: "QA deactivate in edit" });
            assert.equal(make.is_active, false);
            assert.equal(make.data.notes, update.data.notes);
            const event = (await call("GET", `/manufacturers/${make.id}/history`))[0];
            assert.equal(event.before.is_active, true);
            assert.equal(event.after.is_active, false);
            make = await call("PATCH", path, { ...update, version: make.version, reason: "QA omitted status" });
            assert.equal(make.is_active, false);
            make = await call("PATCH", path, { ...update, version: make.version, is_active: true, reason: "QA reactivate in edit" });
            assert.equal(make.is_active, true);
        });
        await t.test("legacy JSON numbers and timestamp microseconds survive reads and audit snapshots", async () => {
            await pool().query(`UPDATE manufacturers SET data=jsonb_set(data,'{legacy_number}','12345678901234567890.123456'::jsonb),created_at='2020-01-01T00:00:00.123456Z' WHERE id=$1`, [make.id]);
            const response = await handleMaster(new Request("http://localhost/api/v1/manufacturers", { headers: { Authorization: `Bearer ${token}` } }));
            assert.equal(response.status, 200);
            const raw = await response.text();
            assert.ok(raw.includes('12345678901234567890.123456'));
            assert.ok(raw.includes('2020-01-01T00:00:00.123456Z'));
            make = await call("PATCH", `/manufacturers/${make.id}/profile`, { name: make.name, data: { eligible: [category.code, type.code], notes: "QA legacy audit preservation" }, version: make.version, reason: "QA legacy precision" });
            assert.equal(make.created_at, '2020-01-01T00:00:00.123456Z');
            const saved = (await pool().query(`SELECT "before"->'data'->>'legacy_number' AS value FROM manufacturers_events WHERE subject_id=$1 AND reason='QA legacy precision'`, [make.id])).rows[0];
            assert.equal(saved.value, '12345678901234567890.123456');
        });
        const itemInput = (model = "QA ＭＯＤＥＬ Straße", size = "S1") => ({ configuration_id: template.id, specifications: { SIZE: size }, model });
        await t.test("item creation derives immutable snapshots and deduplicates normalized identities concurrently", async () => {
            const copies = await Promise.all(Array.from({ length: 5 }, () => call("POST", "/catalogue-items", itemInput(), 201)));
            item = copies[0];
            assert.ok(copies.every(v => v.id === item.id));
            assert.equal(item.serial, null);
            assert.equal(item.model, "qa model strasse");
            assert.equal(item.sku, `${category.code}-${type.code}-SIZES1-QAMODELSTRASSE`);
            assert.equal(item.category_id, undefined);
            assert.equal(item.manufacturer_id, null);
            assert.equal(item.snapshot.manufacturer, undefined);
            assert.ok(!item.description.includes(make.name));
            assert.equal(item.snapshot.configuration.category_code, category.code);
            assert.equal((await call("GET", `/catalogue-items/${item.id}/history`)).length, 1);
            const second = await call("POST", "/catalogue-items", itemInput("Another model", "S2"), 201);
            assert.equal(second.serial, null);
            assert.notEqual(second.fingerprint, item.fingerprint);
            await call("POST", "/catalogue-items", { ...itemInput(), specifications: { SIZE: "missing" } }, 422);
            const refs = await call("GET", "/catalogue-items/references");
            assert.ok(refs.catalogue.some((v: {
                id: string;
            }) => v.id === template.id));
            assert.equal(refs.manufacturers, undefined);
            await assert.rejects(pool().query("UPDATE catalogue_items SET model='forged' WHERE id=$1", [item.id]), /immutable/);
            await assert.rejects(pool().query("UPDATE catalogue_item_events SET reason='forged' WHERE subject_id=$1", [item.id]), /cannot be deleted or rewritten/);
        });
        await t.test("inactive categories stay visible in history and are unavailable for new selections", async () => {
            const path = `/categories/categories/${category.id}`;
            const edit = { code: category.code, name: category.name, version: category.version, reason: "QA category inactive" };
            await call("PATCH", path, { ...edit, is_active: "false" }, 422);
            await call("PATCH", path, { ...edit, is_active: false, reason: undefined }, 422);
            category = await call("PATCH", path, { ...edit, is_active: false });
            assert.equal(category.is_active, false);
            await call("PATCH", path, { ...edit, is_active: true }, 409);
            const history = (await pool().query("SELECT before_snapshot, after_snapshot FROM category_catalogue_changes WHERE subject_id=$1 AND reason=$2", [category.id, edit.reason])).rows[0];
            assert.equal(history.before_snapshot.is_active, true);
            assert.equal(history.after_snapshot.is_active, false);

            const inactive = await call("GET", `/categories/browse?kind=category&status=Inactive&search=${category.code}`);
            assert.equal(inactive.rows[0].id, category.id);
            const detail = await call("GET", `/categories/detail/subcategory/${type.id}`);
            assert.equal(detail.is_active, true);
            assert.equal(detail.category_id, category.id);
            const eligibility = await call("GET", `/manufacturers/eligibility?search=${category.code}`);
            assert.equal(eligibility.total, 0);
            const refs = await call("GET", "/catalogue-items/references");
            assert.equal(refs.catalogue.find((v: { id: string }) => v.id === category.id).is_active, false);
            assert.equal(refs.catalogue.find((v: { id: string }) => v.id === template.id).is_active, false);
            const issued = (await call("GET", "/catalogue-items")).find((v: { id: string }) => v.id === item.id);
            assert.deepEqual(issued, item);
            await call("POST", "/catalogue-items", itemInput("Inactive category test"), 422);
            await call("POST", "/categories/subcategories", { code: prefix + "IN", name: "Inactive category test", category_id: category.id }, 422);
            await call("POST", "/manufacturers", { code: prefix + "IN", name: "Inactive category test", data: { eligible: [category.code] } }, 422);
            make = await call("PATCH", `/manufacturers/${make.id}/profile`, { name: make.name, data: make.data, version: make.version, reason: "QA retain existing eligibility" });
            assert.ok(make.data.eligible.includes(category.code));

            category = await call("PATCH", path, { ...edit, version: category.version, reason: "QA correction with status omitted" });
            assert.equal(category.is_active, false);
            category = await call("PATCH", path, { ...edit, version: category.version, is_active: true, reason: "QA reactivation" });
            assert.equal(category.is_active, true);
            const restored = await call("GET", `/manufacturers/eligibility?search=${category.code}`);
            assert.ok(restored.rows.some((v: { code: string }) => v.code === category.code));
        });
        await t.test("concurrent creates for the same vendor-free SKU save one item without a serial", async () => {
            const input = itemInput("Shared model");
            const responses = await Promise.all([make.id, randomUUID()].map(manufacturer_id => handleMaster(new Request("http://localhost/api/v1/catalogue-items", {
                method: "POST", headers: { Authorization: `Bearer ${token}` }, body: JSON.stringify({ ...input, manufacturer_id }),
            }))));
            assert.deepEqual(responses.map(r => r.status), [201, 201]);
            const [first, second] = await Promise.all(responses.map(r => r.json()));
            assert.equal(first.id, second.id);
            assert.equal(first.manufacturer_id, null);
            assert.equal(first.sku, `${category.code}-${type.code}-SIZES1-SHAREDMODEL`);
            assert.equal(first.serial, null);

            const sameRequest = await call("POST", "/catalogue-items", input, 201);
            assert.equal(sameRequest.id, first.id);
            // Different model text can produce the same SKU after punctuation is removed.
            await call("POST", "/catalogue-items", itemInput("Shared-model"), 409);

            // Use a distinct item ID, fingerprint and serial to exercise the SKU constraint itself.
            await assert.rejects(pool().query(`INSERT INTO catalogue_items(id,configuration_id,manufacturer_id,category_id,model,specifications,snapshot,fingerprint,engineering_key,description,sku,serial,status,is_active)
   SELECT $1,configuration_id,manufacturer_id,category_id,model,specifications,snapshot,$2,engineering_key,description,sku,5,status,is_active FROM catalogue_items WHERE id=$3`,
            [randomUUID(), randomUUID().replaceAll("-", "").padEnd(64, "0"), first.id]), { code: "23505", constraint: "catalogue_items_sku_key" });
        });
        await t.test("template revisions and parent corrections never rewrite issued items", async () => {
            const original = JSON.stringify(item.snapshot), oldTemplate = template.id;
            const base = await call("GET", `/configuration-templates/${type.id}`), next = { ...def(), fields: base.fields.map((f: {
                    id?: string;
                    options: {
                        id?: string;
                    }[];
                }) => { const { id, ...field } = f; void id; return { ...field, options: field.options.map(o => { const { id, ...option } = o; void id; return option; }) }; }) };
            template = await call("POST", `/configuration-templates/${type.id}/publish`, { definition: { ...next, template_name: prefix + " revision 3" }, expected_revision: 2, reason: "QA new revision" });
            assert.notEqual(template.id, oldTemplate);
            const old = (await call("GET", "/catalogue-items")).find((v: {
                id: string;
            }) => v.id === item.id);
            assert.equal(JSON.stringify(old.snapshot), original);
            assert.equal(old.configuration_id, oldTemplate);
            const duplicate = await call("POST", "/catalogue-items", itemInput(), 201);
            assert.equal(duplicate.id, item.id);
        });
        await t.test("lifecycle guards honor stale versions and require an active configuration", async () => {
            item = await call("PATCH", `/catalogue-items/${item.id}`, { status: "retired", version: item.version, reason: "QA retirement" });
            assert.equal(item.is_active, false);
            await call("PATCH", `/catalogue-items/${item.id}`, { status: "approved", version: 1, reason: "QA stale" }, 409);
            make = await call("PATCH", `/manufacturers/${make.id}`, { is_active: false, version: make.version, reason: "QA inactive make" });
            await call("POST", "/catalogue-items", itemInput("Independent of inactive make"), 201);
            item = await call("PATCH", `/catalogue-items/${item.id}`, { status: "approved", version: item.version, reason: "QA restore item" });
            assert.equal(item.is_active, true);
            assert.equal((await call("GET", `/catalogue-items/${item.id}/history`)).length, 3);
            await pool().query("UPDATE catalogue_units SET is_active=false WHERE id=$1", [unit.id]);
            try {
                await call("POST", "/catalogue-items", itemInput("Blocked unit"), 422);
            }
            finally {
                await pool().query("UPDATE catalogue_units SET is_active=true WHERE id=$1", [unit.id]);
            }
        });
        await t.test("failures in audit/event writes roll back items, profiles and template revisions", async () => {
            await pool().query("ALTER TABLE business_events ADD CONSTRAINT epcerp_qa_catalogue_failure CHECK(source_module <> 'catalogue_items') NOT VALID");
            try {
                await call("POST", "/catalogue-items", itemInput("Must roll back"), 503);
                assert.equal((await pool().query("SELECT count(*) FROM catalogue_items WHERE configuration_id=$1 AND model='must roll back'", [template.id])).rows[0].count, "0");
            }
            finally {
                await pool().query("ALTER TABLE business_events DROP CONSTRAINT epcerp_qa_catalogue_failure");
            }
            await pool().query("ALTER TABLE manufacturers_events ADD CONSTRAINT epcerp_qa_manufacturer_failure CHECK(reason <> 'Reject QA') NOT VALID");
            try {
                await call("PATCH", `/manufacturers/${make.id}/profile`, { name: "Must roll back", data: make.data, version: make.version, reason: "Reject QA" }, 503);
                assert.equal((await pool().query("SELECT version FROM manufacturers WHERE id=$1", [make.id])).rows[0].version, make.version);
            }
            finally {
                await pool().query("ALTER TABLE manufacturers_events DROP CONSTRAINT epcerp_qa_manufacturer_failure");
            }
            await pool().query("ALTER TABLE template_publications ADD CONSTRAINT epcerp_qa_template_failure CHECK(reason <> 'Reject QA') NOT VALID");
            try {
                const current = await call("GET", `/configuration-templates/${type.id}`);
                const next = structuredClone(def());
                next.fields[0].options.push({ code: "S3", label: "30", sort_order: 3, is_active: true });
                await call("POST", `/configuration-templates/${type.id}/publish`, { definition: next, expected_revision: current.revision, reason: "Reject QA" }, 503);
                assert.equal((await call("GET", `/configuration-templates/${type.id}`)).revision, current.revision);
            }
            finally {
                await pool().query("ALTER TABLE template_publications DROP CONSTRAINT epcerp_qa_template_failure");
            }
        });
        await t.test("existing Go-style legacy import fingerprints remain idempotent and preserve provenance", async () => {
            const categoryCode = prefix + "I", typeCode = prefix + "IT", stamp = "2020-01-01T00:00:00.123456Z";
            const records = [{ id: randomUUID(), code: categoryCode, name: prefix + " imported category", data: { kind: "category" }, is_active: true, version: 2, created_at: stamp, updated_at: stamp }, { id: randomUUID(), code: typeCode, name: prefix + " imported type", data: { kind: "type", category_code: categoryCode, uom: "Nos", family_code: typeCode, revision: 1, fields: [{ code: "SIZE", label: "Size", required: true, options: [{ code: "S1", label: "10" }] }] }, is_active: true, version: 3, created_at: stamp, updated_at: stamp }];
            const first = await call("POST", "/categories/import", { records });
            assert.equal(first.categories, 1);
            assert.equal(first.subcategories, 1);
            const again = await call("POST", "/categories/import", { records });
            assert.deepEqual(again, { categories: 0, subcategories: 0, units: 0 });
            const imported = (await call("GET", "/categories")).subcategories.find((v: {
                code: string;
            }) => v.code === typeCode);
            assert.ok(imported);
            assert.notEqual(imported.id, records[1].id);
            const templ = { ...definition(), subcategory_code: typeCode, template_name: prefix + " imported template" };
            assert.deepEqual(await call("POST", "/configuration-templates/import", { templates: [templ] }), { created: 1, unchanged: 0 });
            assert.deepEqual(await call("POST", "/configuration-templates/import", { templates: [templ] }), { created: 0, unchanged: 1 });
            const source = { source: { id: randomUUID(), version: 4, created_at: stamp, updated_at: stamp }, code: prefix + "I", name: prefix + " imported make", data: { eligible: [typeCode], notes: "Legacy snapshot" }, is_active: true };
            const importedMake = await call("POST", "/manufacturers/legacy-import", source);
            assert.equal((await call("POST", "/manufacturers/legacy-import", source)).id, importedMake.id);
            await call("POST", "/manufacturers/legacy-import", { ...source, name: source.name + " conflict" }, 409);
            const proof = (await pool().query("SELECT source_snapshot,source_fingerprint FROM manufacturer_imports WHERE go_id=$1", [importedMake.id])).rows[0];
            assert.equal(proof.source_snapshot.source.version, 4);
            assert.match(proof.source_fingerprint, /^[a-f0-9]{64}$/);
        });
        await t.test("revoked grants, invalid payloads and oversized streamed bodies fail closed", async () => {
            await updateRole(token, role.id, { permission_codes: [], expected_version: role.edit_version });
            await call("GET", "/catalogue-items", undefined, 403, readerToken);
            await call("GET", "/categories/browse?size=999", undefined, 422);
            await call("GET", "/categories/browse?sort=name;drop", undefined, 422);
            await call("GET", "/manufacturers/eligibility?skip=-1", undefined, 422);
            await call("POST", "/catalogue-items", { ...itemInput(), sku: "forged" }, 422);
            await call("POST", "/catalogue-items", { ...itemInput(), specifications: null }, 422);
            await call("POST", "/catalogue-items", { ...itemInput(), specifications: { SIZE: null } }, 422);
            for (const version of ["1.0", "1e0", "2147483648"]) {
                const r = await handleMaster(new Request(`http://localhost/api/v1/catalogue-items/${item.id}`, { method: "PATCH", body: `{"status":"retired","version":${version},"reason":"QA numeric token"}`, headers: { Authorization: `Bearer ${token}` } }));
                assert.equal(r.status, 422);
            }
            for (const [path, limit] of [["/catalogue-items", 65536], ["/categories/units", 16384], ["/configuration-templates/import", 1048576], ["/manufacturers", 1048576]] as const) {
                const body = new ReadableStream({ start(c) { c.enqueue(new TextEncoder().encode("x".repeat(limit + 1))); c.close(); } });
                const r = await handleMaster(new Request("http://localhost/api/v1" + path, { method: "POST", body, duplex: "half" } as RequestInit));
                assert.equal(r.status, 413);
            }
        });
        await t.test("legacy model candidates are reused without rewriting their issued evidence", async () => {
            const legacy = randomUUID(), fingerprint = randomUUID().replaceAll("-", "").padEnd(64, "0");
            const sku = `${category.code}-${type.code}-SIZES1-${make.code}-LEGACYMODEL-050`;
            await pool().query(`INSERT INTO catalogue_items(id,configuration_id,manufacturer_id,category_id,model,specifications,snapshot,fingerprint,engineering_key,description,sku,serial,status,is_active,created_by_id,updated_by_id)
   SELECT $1,configuration_id,$5,category_id,'Legacy ＭＯＤＥＬ',specifications,snapshot || jsonb_build_object('manufacturer',$6::jsonb),$2,engineering_key,'Unchanged legacy description',$3,50,status,is_active,created_by_id,updated_by_id FROM catalogue_items WHERE id=$4`, [legacy, fingerprint, sku, item.id, make.id, JSON.stringify(make)]);
            const before = (await pool().query("SELECT to_jsonb(c)::text AS snapshot FROM catalogue_items c WHERE id=$1", [legacy])).rows[0].snapshot;
            const reused = await call("POST", "/catalogue-items", itemInput("  LEGACY model "), 201);
            assert.equal(reused.id, legacy);
            assert.equal(reused.model, "Legacy ＭＯＤＥＬ");
            assert.equal(reused.description, "Unchanged legacy description");
            assert.equal(reused.manufacturer_id, make.id);
            assert.equal(reused.snapshot.manufacturer.name, make.name);
            await call("POST", "/catalogue-items", itemInput("Legacy-model"), 409);
            assert.equal((await pool().query("SELECT to_jsonb(c)::text AS snapshot FROM catalogue_items c WHERE id=$1", [legacy])).rows[0].snapshot, before);
        });
        await t.test("the last legacy serial does not prevent a new SKU from being issued", async () => {
            await pool().query(`INSERT INTO catalogue_items(id,configuration_id,manufacturer_id,category_id,model,specifications,snapshot,fingerprint,engineering_key,description,sku,serial,status,is_active,created_by_id,updated_by_id)
   SELECT $1,configuration_id,manufacturer_id,category_id,'QA exhaustion fixture',specifications,snapshot,$2,engineering_key,description,$3,999,status,is_active,created_by_id,updated_by_id FROM catalogue_items WHERE id=$4`, [randomUUID(), randomUUID().replaceAll("-", "").padEnd(64, "0"), "QA-EXHAUSTION-" + randomUUID(), item.id]);
            const saved = await call("POST", "/catalogue-items", itemInput("After legacy limit"), 201);
            assert.equal(saved.sku, `${category.code}-${type.code}-SIZES1-AFTERLEGACYLIMIT`);
            assert.equal(saved.serial, null);
        });
    }
    finally {
        await pool().end();
    }
});
