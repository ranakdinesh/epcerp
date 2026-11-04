import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { readFileSync } from "node:fs";
import test from "node:test";
import { cleanCategory, parseCategorySources } from "../../src/server/master/categories/domain";
import { parseManufacturer, parseManufacturerImport } from "../../src/server/master/manufacturers/domain";
import { evaluateTemplate, parseDefinition, validateDefinitions } from "../../src/server/master/templates/domain";
import { deriveItem, type ItemConfiguration, type ItemMake } from "../../src/server/master/items/domain";
import { hash } from "../../src/server/master/catalogue/serialization";
import { ownsMasterPath } from "../../src/server/master/http";
import { definition } from "./catalogue-fixtures";
const uuid = "00112233-4455-4677-8899-aabbccddeeff";
test("Master route ownership preserves Go Purchasing's nested client policies", () => {
    for (const path of ["/clients", `/clients/${uuid}`, `/clients/${uuid}/activate`, "/catalogue-items", "/categories", "/manufacturers", `/configuration-templates/${uuid}`]) assert.equal(ownsMasterPath(path), true);
    for (const path of [`/clients/${uuid}/purchase-rules`, `/clients/${uuid}/purchase-rules/${uuid}/history`, `/projects/${uuid}/items/catalogue`]) assert.equal(ownsMasterPath(path), false);
});
const config: ItemConfiguration = { id: uuid, code: "QATYPE", name: "QA product", is_active: true, version: 1, data: { kind: "type", category_code: "QACAT", family_code: "QAFAM", uom: "Nos", revision: 1, model_required: true, fields: [{ code: "SIZE", label: "Size", required: true, options: [{ code: "S1", label: "10" }, { code: "__NA__", label: "N/A" }] }] } };
const make: ItemMake = { id: uuid, code: "QA", name: "QA <Make>", is_active: true, data: { eligible: ["QACAT"] } };
type Fixture = {
    area: string;
    kind?: string;
    body?: unknown;
    configuration?: ItemConfiguration;
    manufacturer?: ItemMake;
    values?: Record<string, string>;
    model?: string;
};
test("catalogue domain rules, hashes, Unicode models and legacy fingerprints match Go", { skip: !process.env.EPCERP_CATALOGUE_COMPATIBILITY_BINARY }, () => {
    const fixtures: Fixture[] = [];
    for (const kind of ["unit", "category", "subcategory"]) {
        for (const name of ["  QA  Product  ", "Σς Straße", "\ufeffX", "x\0y", null])
            for (const area of ["category-create", "category-edit"]) {
                fixtures.push({ area, kind, body: { code: " qa01 ", name, reason: "  QA reviewed  ", ...(area === "category-edit" ? { version: 1 } : kind === "category" ? { unit_id: uuid } : kind === "subcategory" ? { category_id: uuid } : {}) } });
            }
    }
    for (const body of [definition(), { ...definition(), model_required: null }, { ...definition(), fields: [] }, { ...definition(), template_name: "🙂".repeat(100) }, { ...definition(), fields: [{ ...definition().fields[0], id: uuid }] }, { ...definition(), fields: [{ ...definition().fields[0], options: [{ code: "__NA__", label: "N/A", sort_order: 1, is_active: true }] }] }])
        fixtures.push({ area: "template", body });
    const template = { model_required: true, id: uuid, subcategory_id: uuid, template_name: "QA template", revision: 1, is_active: true, fields: definition().fields };
    for (const values of [{ SIZE: "S1" }, { SIZE: "__NA__" }, { SIZE: "bad" }, {}, { SIZE: "S1", UNKNOWN: "S1" }] as Record<string, string>[])
        fixtures.push({ area: "preview", body: template, values });
    for (const model of ["  QA MODEL  ", "Straße Σς İ ı ſ K", "ＡＢＣ１２３", "ΟΣ ẞ ß ﬃ", "E\u0301\u001c\u0085 X", "💡 <test> & \u2028Z", "\ufeffA", "", "x".repeat(121)])
        for (const values of [{ SIZE: "s1" }, { SIZE: "__na__" }, { SIZE: "bad" }, {}] as Record<string, string>[])
            fixtures.push({ area: "item", configuration: config, manufacturer: make, model, values });
    fixtures.push({ area: "item", configuration: config, manufacturer: { ...make, is_active: false }, model: "model", values: { SIZE: "S1" } });
    for (const kind of ["create", "profile", "active"]) {
        for (const body of [{ code: "QA", name: "  QA make  ", data: { eligible: [" QACAT "], notes: " QA notes " }, reason: " reason ", version: 1, is_active: false }, null, {}, { is_active: true, version: "1", reason: "QA" }]) {
            if (body && "code" in body) {
                const v: Record<string, unknown> = { ...body };
                for (const key of kind === "create" ? ["version", "is_active"] : kind === "profile" ? ["code", "is_active"] : ["code", "name", "data"])
                    delete v[key];
                fixtures.push({ area: "manufacturer", kind, body: v });
            }
            else
                fixtures.push({ area: "manufacturer", kind, body });
        }
    }
    const imported = { source: { id: uuid, version: 1, created_at: "2020-01-01T00:00:00.123456Z", updated_at: "2020-01-01T00:00:00Z" }, code: "QA", name: "Legacy QA & Σ", data: { eligible: ["QACAT"], notes: "" }, is_active: false };
    fixtures.push({ area: "manufacturer-import", body: imported });
    const sources = [{ id: uuid, code: "QACAT", name: "QA category", data: { kind: "category" }, is_active: true, version: 1, created_at: "2020-01-01T00:00:00.123456Z", updated_at: "2020-01-01T00:00:00Z" }, { id: "10112233-4455-4677-8899-aabbccddeeff", code: "QATYPE", name: "QA type", data: { kind: "type", category_code: "QACAT", uom: "Nos", family_code: "QAFAM", revision: 1, fields: [{ code: "SIZE", label: "Size", required: true, options: [{ code: "S1", label: "10" }] }] }, is_active: true, version: 1, created_at: "2020-01-01T00:00:00Z", updated_at: "2020-01-01T00:00:00Z" }];
    fixtures.push(...sources.map(body => ({ area: "category-source", body })));
    const oracle = spawnSync(process.env.EPCERP_CATALOGUE_COMPATIBILITY_BINARY!, [], { input: JSON.stringify(fixtures), encoding: "utf8" });
    assert.equal(oracle.status, 0, oracle.stderr);
    const expected = JSON.parse(oracle.stdout);
    fixtures.forEach((fixture, i) => {
        let actual: Record<string, unknown> = { valid: true };
        try {
            switch (fixture.area) {
                case "category-create":
                case "category-edit": {
                    const v = cleanCategory(fixture.kind as "unit" | "category" | "subcategory", fixture.body, fixture.area === "category-edit");
                    actual.value = fixture.area === "category-edit" ? { code: v.code, name: v.name, reason: v.reason, version: v.version } : { code: v.code, name: v.name, ...(v.unit_id ? { unit_id: v.unit_id } : {}), ...(v.category_id ? { category_id: v.category_id } : {}), reason: v.reason };
                    break;
                }
                case "template": {
                    const [v] = validateDefinitions([parseDefinition(fixture.body)]);
                    actual.value = v;
                    actual.fingerprint = hash(v);
                    break;
                }
                case "preview":
                    Object.assign(actual, evaluateTemplate(template, { values: fixture.values }));
                    break;
                case "item": {
                    const v = deriveItem(fixture.configuration!, fixture.manufacturer!, fixture.values!, fixture.model!);
                    Object.assign(actual, { engineering_key: v.engineering_key, fingerprint: v.fingerprint, model: v.model, prefix: v.prefix, description: v.description, specifications: v.specifications });
                    break;
                }
                case "manufacturer": {
                    const v = parseManufacturer(fixture.body, fixture.kind as "create" | "active" | "profile");
                    actual.value = { ...v, ...(fixture.kind === "active" ? { data: { eligible: null, notes: "" } } : {}), is_active: fixture.kind === "active" ? v.is_active : false };
                    break;
                }
                case "manufacturer-import":
                    actual.fingerprint = hash(parseManufacturerImport(fixture.body));
                    break;
                case "category-source":
                    actual.fingerprint = hash(parseCategorySources({ records: sources }).find(s => s.code === (fixture.body as {
                        code: string;
                    }).code));
                    break;
            }
        }
        catch {
            actual = { valid: false };
        }
        assert.equal(actual.valid, expected[i].valid, `fixture ${i} ${fixture.area} ${JSON.stringify(fixture)}`);
        if (actual.valid)
            assert.deepEqual(JSON.parse(JSON.stringify(actual)), expected[i], `fixture ${i} ${fixture.area}`);
    });
    const folded = spawnSync(process.env.EPCERP_CATALOGUE_COMPATIBILITY_BINARY!, ["fold"], { encoding: "utf8" });
    assert.equal(folded.status, 0);
    assert.deepEqual(JSON.parse(folded.stdout), JSON.parse(readFileSync(new URL("../../src/server/master/catalogue/full-fold.json", import.meta.url), "utf8")));
});
