import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import test from "node:test";
import { ownsMasterPath, ownsNativeSupplierPath } from "../../src/server/master/http";
import { dataFingerprint, parseVendorCreate, parseVendorImport, parseVendorUpdate, validateVendorMaster } from "../../src/server/master/vendors/domain";
import { parseCatalogueJSON } from "../../src/server/master/catalogue/json";
test("supplier profiles and optional documents migrate while Finance stays Go-owned", () => {
    const id = "00112233-4455-4677-8899-aabbccddeeff";
    for (const path of ["/suppliers", `/suppliers/${id}`, "/suppliers/legacy-import"])
        assert.equal(ownsMasterPath(path), true);
    for (const path of [`/suppliers/${id}/documents`, `/suppliers/${id}/documents/${id}/preview`, `/suppliers/${id}/documents/${id}/archive`])
        assert.equal(ownsMasterPath(path), process.env.EPCERP_VENDOR_DOCUMENTS_OWNER === "nextjs");
    for (const path of [`/suppliers/${id}/documents/reconcile-approvals`, `/suppliers/${id}/verification`, `/suppliers/${id}/verification/assignment`, `/suppliers/${id}/verification/assignment/options`])
        assert.equal(ownsMasterPath(path), process.env.EPCERP_VENDOR_VERIFICATION_OWNER === "nextjs");
});
test("supplier document and profile owners can roll back independently", () => {
    const previousProfiles = process.env.EPCERP_VENDOR_PROFILES_OWNER, previousDocuments = process.env.EPCERP_VENDOR_DOCUMENTS_OWNER;
    const id = "00112233-4455-4677-8899-aabbccddeeff";
    try {
        for (const profiles of ["go", "nextjs"]) for (const documents of ["go", "nextjs"]) {
            process.env.EPCERP_VENDOR_PROFILES_OWNER = profiles;
            process.env.EPCERP_VENDOR_DOCUMENTS_OWNER = documents;
            assert.equal(ownsNativeSupplierPath("/suppliers"), profiles === "nextjs");
            assert.equal(ownsNativeSupplierPath(`/suppliers/${id}`), profiles === "nextjs");
            assert.equal(ownsNativeSupplierPath(`/suppliers/${id}/documents`), documents === "nextjs");
            assert.equal(ownsNativeSupplierPath(`/suppliers/${id}/documents/${id}/download`), documents === "nextjs");
            assert.equal(ownsNativeSupplierPath(`/suppliers/${id}/documents/reconcile-approvals`), process.env.EPCERP_VENDOR_VERIFICATION_OWNER === "nextjs");
            assert.equal(ownsNativeSupplierPath(`/suppliers/${id}/verification`), process.env.EPCERP_VENDOR_VERIFICATION_OWNER === "nextjs");
        }
    } finally {
        if (previousProfiles === undefined) delete process.env.EPCERP_VENDOR_PROFILES_OWNER; else process.env.EPCERP_VENDOR_PROFILES_OWNER = previousProfiles;
        if (previousDocuments === undefined) delete process.env.EPCERP_VENDOR_DOCUMENTS_OWNER; else process.env.EPCERP_VENDOR_DOCUMENTS_OWNER = previousDocuments;
    }
});
test("supplier validation and exact payment arithmetic match the Go implementation", { skip: !process.env.EPCERP_CATALOGUE_COMPATIBILITY_BINARY }, () => {
    const base = { code: " qa-01_ ", name: "  Straße   Σς  ", data: { qualification: "unverified", contact_person_name: "QA Contact", payment_terms: { advance_pct: 40, delivery_pct: 60, credit_days: 30 } }, reason: "  QA reviewed  " };
    const fixtures: {
        area: string;
        kind: string;
        raw: string;
    }[] = [];
    const variants: unknown[] = [base, null, [], {}, { ...base, code: "ß" }, { ...base, name: "💡".repeat(255) }, { ...base, name: "💡".repeat(256) }, { ...base, reason: "é".repeat(1000) }, { ...base, reason: "é".repeat(1001) }, { ...base, source: { id: "forged" } }, { ...base, data: null }];
    for (const data of [{ qualification: "approved" }, { qualification: "approved", notes: "reviewed" }, { qualification: "suspended" }, { onboarding_verified: true }, { onboarding_verified: 1 }, { category_ids: [] }, { category_ids: ["not-mapped"] }, { item_ids: null }, { unknown: "x" }, { notes: 100 }, { email: null }, { indent_terms: "x".repeat(12000) }, { indent_terms: "x".repeat(12001) }])
        variants.push({ ...base, data });
    const numericTerms = ['{}', '{"advance_pct":0,"delivery_pct":0}', '{"advance_pct":50,"delivery_pct":49.99}', '{"advance_pct":50,"delivery_pct":50.01}', '{"advance_pct":50,"delivery_pct":49.9899999999999999999999}',
        '{"advance_pct":50,"delivery_pct":50.0100000000000000000001}', '{"advance_pct":50,"delivery_pct":49.99,"qc_pct":1e-1000}', '{"advance_pct":50,"delivery_pct":50.01,"qc_pct":1e-1000}',
        '{"advance_pct":100.00000000000000000001}', '{"advance_pct":"100"}', '{"advance_pct":null}', '{"advance_pct":true}', '{"advance_pct":-0}', '{"advance_pct":-0.1}',
        '{"advance_pct":1e2,"credit_days":3e2}', '{"advance_pct":100.00,"credit_days":30.0}', '{"advance_pct":0,"credit_days":365.00000000000000000001}', '{"credit_days":366}', '{"credit_days":"30"}', '{"description":500}', '{"unknown":1}', '{"advance_pct":1e1000001}', '{"advance_pct":1e-1000001}'];
    for (const area of ["vendor-create", "vendor-update"]) {
        for (const body of variants) {
            let value = body;
            if (body && typeof body === "object" && !Array.isArray(body)) {
                value = { ...(body as object) };
                if (area === "vendor-update") {
                    delete (value as Record<string, unknown>).code;
                    Object.assign(value as object, { version: 5 });
                }
            }
            fixtures.push({ area, kind: "QA-01_", raw: JSON.stringify(value) });
        }
        for (const terms of numericTerms) {
            const raw = area === "vendor-create" ? `{"code":"QA-01_","name":"QA","data":{"payment_terms":${terms}},"reason":"QA"}` : `{"name":"QA","data":{"payment_terms":${terms}},"version":5,"reason":"QA","is_active":false}`;
            fixtures.push({ area, kind: "QA-01_", raw });
        }
    }
    const source = { id: "00112233-4455-4677-8899-aabbccddeeff", version: 7, created_at: "2020-01-01T05:30:00.123456+05:30", updated_at: "2020-01-02T00:00:00Z" };
    for (const body of [{ source, code: "QA_IMPORT", name: " Legacy supplier ", data: base.data, is_active: false }, { source: { ...source, id: source.id.toUpperCase() }, code: "QA_IMPORT", name: "QA", data: {}, is_active: true }, { source: { ...source, version: 0 }, code: "QA_IMPORT", name: "QA", data: {}, is_active: true }, { source, code: "qa", name: "QA", data: {}, is_active: true }, { source, code: "QA", name: "QA", data: {} }])
        fixtures.push({ area: "vendor-import", kind: "", raw: JSON.stringify(body) });
    for (const version of ['1.0', '1e0', '"1"', '2147483648'])
        fixtures.push({ area: "vendor-update", kind: "QA-01_", raw: `{"name":"QA","data":{},"reason":"QA","version":${version}}` });
    for (const stamp of ["0001-01-01T00:00:00.000000Z", "2020-02-30T00:00:00Z", "2020-01-01T24:00:00Z"])
        fixtures.push({area:"vendor-import",kind:"",raw:JSON.stringify({source:{...source,created_at:stamp},code:"QA",name:"QA",data:{},is_active:true})});
    const input = '[' + fixtures.map(f => `{"area":${JSON.stringify(f.area)},"kind":${JSON.stringify(f.kind)},"body":${f.raw}}`).join(',') + ']';
    console.info(`Compared ${fixtures.length} supplier fixtures with Go`);
    const result = spawnSync(process.env.EPCERP_CATALOGUE_COMPATIBILITY_BINARY!, [], { input, encoding: "utf8" });
    assert.equal(result.status, 0, result.stderr);
    const expected = JSON.parse(result.stdout);
    fixtures.forEach((f, index) => {
        let actual: Record<string, unknown>;
        try {
            const body = parseCatalogueJSON(f.raw);
            let value: ReturnType<typeof parseVendorCreate> | ReturnType<typeof parseVendorImport> | ReturnType<typeof parseVendorUpdate>;
            if (f.area === "vendor-create")
                value = parseVendorCreate(body);
            else if (f.area === "vendor-import")
                value = parseVendorImport(body);
            else {
                const v = parseVendorUpdate(body);
                value = { ...v, data: validateVendorMaster(f.kind, v.name, v.data) };
            }
            actual = { valid: true, value, fingerprint: dataFingerprint(value.data as Record<string, unknown>) };
        }
        catch {
            actual = { valid: false };
        }
        assert.equal(actual.valid, expected[index].valid, `fixture ${index}: ${f.raw}`);
        if (actual.valid)
            assert.deepEqual(JSON.parse(JSON.stringify(actual)), JSON.parse(JSON.stringify(expected[index])), `fixture ${index}`);
    });
});
