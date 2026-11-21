import assert from "node:assert/strict";
import { randomUUID } from "node:crypto";
import { spawnSync } from "node:child_process";
import test from "node:test";
import { initializeAuth } from "../../src/server/auth/bootstrap";
import { login } from "../../src/server/auth/sessions";
import { createRole, updateRole } from "../../src/server/auth/administration";
import { hashPassword } from "../../src/server/auth/security";
import { pool } from "../../src/server/db";
import { handleMaster } from "../../src/server/master/http";
import { prepareReference, referenceKinds } from "../../src/server/master/references/domain";
import { parseReference, parseReferenceQuery } from "../../src/server/master/references/validation";
import { normalizedName } from "../../src/server/master/shared/text";
test("reference parsing and normalization match the actual Go implementation", { skip: !process.env.EPCERP_MASTER_COMPATIBILITY_BINARY }, () => {
    const id = randomUUID();
    const bodies: unknown[] = [{ name: "Public Sector" }, { name: "  Straße   Σς  " }, { name: "\u0085 An\u0085Industry " }, { name: "\ufeffName" },
        { name: "A" }, { name: "AB", code: null }, { name: "AB", code: "ß" }, { name: "AB", code: "ı ſ" }, { name: "AB", state_id: `{${id.toUpperCase()}}` },
        { name: "AB", state_id: null }, { name: "AB", state_id: "" }, { name: "AB", is_active: true }, { name: "A".repeat(31) + "  B" }, [], null];
    const fixtures = referenceKinds.flatMap(kind => bodies.map(body => ({ kind, body })));
    const result = spawnSync(process.env.EPCERP_MASTER_COMPATIBILITY_BINARY!, ["references"], { input: JSON.stringify(fixtures), encoding: "utf8" });
    assert.equal(result.status, 0, result.stderr);
    const expected = JSON.parse(result.stdout);
    fixtures.forEach(({ kind, body }, i) => {
        let actual: Record<string, unknown>;
        try {
            const v = prepareReference(kind, parseReference(body));
            actual = { valid: true, field: "", name: v.name, code: v.code, state_id: v.state_id ?? "", normalized_name: normalizedName(v.name) };
        }
        catch {
            actual = { valid: false };
        }
        assert.equal(actual.valid, expected[i].valid, `fixture ${i}`);
        if (actual.valid)
            assert.deepEqual(actual, expected[i], `fixture ${i}`);
    });
    for (const value of ["TRUE", "yes", "On", "1", "t"])
        assert.equal(parseReferenceQuery(new URLSearchParams({ include_inactive: value })).activeOnly, false);
    for (const value of ["false", "NO", "off", "0", "f"])
        assert.equal(parseReferenceQuery(new URLSearchParams({ include_inactive: value })).activeOnly, true);
    for (const value of [" true ", "", "maybe"])
        assert.throws(() => parseReferenceQuery(new URLSearchParams({ include_inactive: value })));
});
test("reference administration preserves existing PostgreSQL contracts", { skip: !process.env.EPCERP_DATABASE_URL }, async (t) => {
    assert.match(new URL(process.env.EPCERP_DATABASE_URL!).pathname, /\/epcerp_(auth|master)_qa_/u);
    try {
        await initializeAuth();
        const token = (await login("superadmin", process.env.VTPL_BOOTSTRAP_SUPERADMIN_PASSWORD!)).access_token;
        const prefix = `refqa${randomUUID().replace(/-/g, "").slice(0, 10)}`;
        async function call(method: string, path: string, body?: unknown, access = token, status = 200) {
            const r = await handleMaster(new Request(`http://localhost/api/v1/client-references/${path}`, { method, headers: { Authorization: `Bearer ${access}` }, body: body === undefined ? undefined : JSON.stringify(body) }));
            const v = await r.json();
            assert.equal(r.status, status, JSON.stringify(v));
            return v;
        }
        const records: Record<string, {
            id: string;
            name: string;
            code: string;
            state_id: string | null;
            is_active: boolean;
        }> = {};
        const role = await createRole(token, { name: `${prefix}.reader`, description: "QA location reader", permission_codes: ["settings.locations.read"] });
        const reader = randomUUID();
        await pool().query("INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser) VALUES($1,$2,$3,$4,true,false)", [reader, prefix, `${prefix}@fixture.invalid`, await hashPassword("Reference-Reader1!")]);
        await pool().query("INSERT INTO auth.user_roles(user_id,role_id) VALUES($1,$2)", [reader, role.id]);
        const readerToken = (await login(prefix, "Reference-Reader1!")).access_token;
        await t.test("permissions remain granular and anonymous operations fail", async () => {
            await call("GET", "states", undefined, "", 401);
            await call("GET", "states", undefined, readerToken);
            await call("GET", "client-types", undefined, readerToken, 403);
            await call("GET", "industries", undefined, readerToken, 403);
            await call("POST", "states", { name: prefix }, readerToken, 403);
        });
        await t.test("all four directories create, list and edit through the same adapter", async () => {
            for (const kind of ["states", "client-types", "industries", "cities"]) {
                const value = { name: `${prefix} ${kind}`, code: `${prefix}-${kind}`, ...(kind === "cities" ? { state_id: records.states.id } : {}) };
                const v = await call("POST", kind, value, token, 201);
                records[kind] = v;
                assert.deepEqual(Object.keys(v).sort(), ["code", "id", "is_active", "name", "state_id"]);
                assert.equal(v.is_active, true);
                assert.equal(v.state_id, kind === "cities" ? records.states.id : null);
                assert.ok((await call("GET", kind)).some((r: {
                    id: string;
                }) => r.id === v.id));
                const row = (await pool().query(`SELECT created_by_id,updated_by_id FROM ${kind === "states" ? "indian_states" : kind === "cities" ? "indian_cities" : kind.replaceAll("-", "_")} WHERE id=$1`, [v.id])).rows[0];
                assert.ok(row.created_by_id);
                assert.equal(row.created_by_id, row.updated_by_id);
                records[kind] = await call("PATCH", `${kind}/{${v.id.toUpperCase()}}`, { ...value, name: `${value.name} edited` });
                assert.equal(records[kind].id, v.id);
            }
            const cities = await call("GET", `cities?state_id=${records.states.id}`);
            assert.ok(cities.some((v: {
                id: string;
            }) => v.id === records.cities.id));
            assert.deepEqual(await call("GET", `cities?state_id=${randomUUID()}`), []);
        });
        await t.test("duplicate keys conflict while cities share codes and names across parents", async () => {
            await call("POST", "industries", { name: records.industries.name.toUpperCase(), code: `${prefix}other` }, token, 409);
            await call("POST", "industries", { name: `${prefix} different`, code: records.industries.code }, token, 409);
            records.other = await call("POST", "states", { name: `${prefix} other` }, token, 201);
            await call("POST", "cities", { name: records.cities.name, state_id: records.states.id }, token, 409);
            await call("POST", "cities", { name: records.cities.name, state_id: records.other.id, code: records.cities.code }, token, 201);
            await call("POST", "cities", { name: `${prefix} city two`, state_id: records.states.id, code: records.cities.code }, token, 201);
        });
        await t.test("edits retain legacy IDs, created metadata and inactive status", async () => {
            const old = (await pool().query("SELECT created_at,created_by_id FROM industries WHERE id=$1", [records.industries.id])).rows[0];
            await call("POST", `industries/${records.industries.id}/deactivate`);
            const updated = await call("PATCH", `industries/${records.industries.id}`, { name: `${prefix} edited inactive` });
            assert.equal(updated.is_active, false);
            const kept = (await pool().query("SELECT created_at,created_by_id FROM industries WHERE id=$1", [records.industries.id])).rows[0];
            assert.deepEqual(kept, old);
            assert.ok(!(await call("GET", "industries")).some((v: {
                id: string;
            }) => v.id === updated.id));
            assert.ok((await call("GET", "industries?include_inactive=YES")).some((v: {
                id: string;
            }) => v.id === updated.id));
            await call("POST", `industries/${updated.id}/activate`);
        });
        await t.test("state retirement is atomic and reactivation leaves cities retired", async () => {
            await pool().query(`ALTER TABLE indian_states ADD CONSTRAINT epcerp_qa_state_failure CHECK(id <> '${records.states.id}'::uuid OR is_active) NOT VALID`);
            try {
                await call("POST", `states/${records.states.id}/deactivate`, undefined, token, 503);
                assert.equal((await pool().query("SELECT is_active FROM indian_cities WHERE id=$1", [records.cities.id])).rows[0].is_active, true);
            }
            finally {
                await pool().query("ALTER TABLE indian_states DROP CONSTRAINT epcerp_qa_state_failure");
            }
            await call("POST", `states/${records.states.id}/deactivate`);
            assert.equal((await pool().query("SELECT count(*) FROM indian_cities WHERE state_id=$1 AND is_active", [records.states.id])).rows[0].count, "0");
            await call("POST", `cities/${records.cities.id}/activate`, undefined, token, 422);
            await call("POST", "cities", { name: `${prefix} blocked`, state_id: records.states.id }, token, 422);
            await call("PATCH", `cities/${records.cities.id}`, { name: `${prefix} blocked`, state_id: records.states.id }, token, 422);
            await call("POST", `states/${records.states.id}/activate`);
            assert.equal((await pool().query("SELECT is_active FROM indian_cities WHERE id=$1", [records.cities.id])).rows[0].is_active, false);
            await call("POST", `cities/${records.cities.id}/activate`);
            const moved = await call("PATCH", `cities/${records.cities.id}`, { name: records.cities.name + " moved", state_id: records.other.id });
            assert.equal(moved.state_id, records.other.id);
        });
        await t.test("shared lock serializes a city activation against state deactivation", async () => {
            for (let i = 0; i < 3; i++) {
                await call("POST", `states/${records.other.id}/activate`);
                await Promise.all([call("POST", `states/${records.other.id}/deactivate`), (async () => {
                        const r = await handleMaster(new Request(`http://localhost/api/v1/client-references/cities/${records.cities.id}/activate`, { method: "POST", headers: { Authorization: `Bearer ${token}` } }));
                        assert.ok([200, 422].includes(r.status));
                    })()]);
                assert.equal((await pool().query("SELECT count(*) FROM indian_cities WHERE state_id=$1 AND is_active", [records.other.id])).rows[0].count, "0");
            }
        });
        await t.test("revoked grants apply to existing sessions and malformed requests cannot write", async () => {
            await updateRole(token, role.id, { permission_codes: [], expected_version: role.edit_version });
            await call("GET", "states", undefined, readerToken, 403);
            await call("GET", "bogus", undefined, token, 404);
            await call("GET", "states?state_id=bad", undefined, token, 422);
            await call("GET", "states?include_inactive=maybe", undefined, token, 422);
            await call("PATCH", `states/${randomUUID()}`, { name: prefix }, token, 404);
            await call("POST", "states", { name: prefix, created_by_id: randomUUID() }, token, 422);
            await call("POST", "states", { name: prefix, state_id: records.states.id }, token, 422);
            await call("POST", "cities", { name: prefix }, token, 422);
            const body = new ReadableStream({ start(c) { c.enqueue(new TextEncoder().encode("x".repeat(4097))); c.close(); } });
            const r = await handleMaster(new Request("http://localhost/api/v1/client-references/states", { method: "POST", body, duplex: "half" } as RequestInit));
            assert.equal(r.status, 413);
        });
    }
    finally {
        await pool().end();
    }
});
