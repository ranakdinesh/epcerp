import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { effectivePermissions, strongPassword, type User } from "../../src/server/auth/domain";
import { hashPassword, issueAccess, validPHC, verifyAccess, verifyPassword } from "../../src/server/auth/security";
import { openMail, sealMail } from "../../src/server/auth/mail";
import { body } from "../../src/server/auth/validation";
import registry from "../../src/server/auth/permissions.json";

process.env.VTPL_JWT_SECRET_KEY = "epcerp-synthetic-auth-compatibility-secret-only";
process.env.EPCERP_IMAGE_ENCRYPTION_KEY = "11".repeat(32);
const backend = fileURLToPath(new URL("../../../backend",import.meta.url));
const fixturePath = process.env.EPCERP_AUTH_COMPATIBILITY_FILE;

test("Go Argon2id/i passwords and JWTs survive migration in both directions", { skip: !fixturePath }, async () => {
  const fixture = JSON.parse(readFileSync(fixturePath!,"utf8"));
  for (const hash of fixture.hashes) {
    assert.equal(await verifyPassword(fixture.password,hash),true);
    assert.equal(await verifyPassword("incorrect",hash),false);
  }
  assert.equal(verifyAccess(fixture.token).sub,fixture.id);
  const hash = await hashPassword(fixture.password);
  const token = issueAccess(fixture.id,Math.floor(Date.now()/1000)+3600,0).token;
  assert.equal(openMail(Buffer.from(fixture.mail_payload,"base64"),fixture.mail_hash).toString(),"synthetic queued credential");
  const mail = sealMail(Buffer.from("synthetic queued credential"));
  const result = spawnSync(process.env.EPCERP_AUTH_COMPATIBILITY_BINARY ?? "go", process.env.EPCERP_AUTH_COMPATIBILITY_BINARY ? ["verify"] : ["run","./tools/auth-compatibility","verify"], {
    cwd:backend,input:JSON.stringify({Token:token,Password:fixture.password,Hash:hash,Payload:mail.payload.toString("base64"),PayloadHash:mail.hash}),encoding:"utf8",
  });
  assert.equal(result.status,0,result.stderr);
});
test("password verifier rejects malformed and resource-exhausting PHC parameters", async () => {
  for (const value of ["plaintext","$argon2id$v=19$m=999999999,t=3,p=4$c2FsdA$aGFzaA", "$argon2id$v=19$m=65536,t=0,p=4$c2FsdA$aGFzaA", "$argon2d$v=19$m=65536,t=3,p=4$c2FsdA$aGFzaA"]) {
    assert.equal(validPHC(value),false);
    assert.equal(await verifyPassword("password",value),false);
  }
});
test("access verification rejects tampering, expiry, foreign issuer and wrong key", () => {
  const token = issueAccess("10000000-0000-4000-8000-000000000001",2000,7,1000).token;
  assert.equal(verifyAccess(token,1001).cv,7);
  assert.throws(()=>verifyAccess(token,2000));
  assert.throws(()=>verifyAccess(token.slice(0,-3)+"AAA",1001));
  process.env.VTPL_JWT_ISSUER = "foreign";
  assert.throws(()=>verifyAccess(token,1001)); delete process.env.VTPL_JWT_ISSUER;
  const secret = process.env.VTPL_JWT_SECRET_KEY;
  process.env.VTPL_JWT_SECRET_KEY = "another-synthetic-secret-with-32-characters";
  assert.throws(()=>verifyAccess(token,1001)); process.env.VTPL_JWT_SECRET_KEY=secret;
});
test("permissions exclude inactive grants, forced-password accounts and role-based superadmin", () => {
  const u = { is_active:true,is_superuser:false,must_change_password:false,roles:[{is_active:true,permissions:["projects.master.read","system:admin"]},{is_active:false,permissions:["admin.users.write"]}] } as User;
  assert.ok(effectivePermissions(u).includes("projects.master.read"));
  assert.ok(!effectivePermissions(u).includes("system:admin"));
  assert.ok(!effectivePermissions(u).includes("admin.users.write"));
  assert.deepEqual(effectivePermissions({...u,must_change_password:true}),[]);
  assert.deepEqual(effectivePermissions({...u,is_active:false}),[]);
});
test("credential mail envelopes preserve Go's GCM format and reject tampering", () => {
  const plain = Buffer.from('{"ID":"synthetic","Message":"private test"}'), result = sealMail(plain);
  assert.deepEqual(openMail(result.payload,result.hash),plain);
  assert.equal(result.payload.length,plain.length+28);
  const altered = Buffer.from(result.payload); altered[15]^=1;
  assert.throws(()=>openMail(altered,result.hash));
  assert.throws(()=>openMail(result.payload,"0".repeat(64)));
});
test("password strength keeps Unicode character semantics", () => {
  assert.equal(strongPassword("Éléphant١!"),true);
  for (const s of ["weak","abcdefgh!1","ABCDEFGH!1","Abcd1234"]) assert.equal(strongPassword(s),false);
});
test("request parsing bounds streamed bodies and rejects unknown authority fields", async () => {
  await assert.rejects(body(new Request("http://localhost",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({id:"forged"})}),["username"]));
  await assert.rejects(body(new Request("http://localhost",{method:"POST",headers:{"Content-Type":"application/json"},body:"x".repeat(20000)}),["username"]), {status:413});
});
test("transition permission registries agree with Go", () => {
  const go = readFileSync(`${backend}/internal/modules/auth/application/permissions.go`,"utf8");
  const expected = [...go.matchAll(/\{Code: "([^"]+)", Description: "([^"]+)", Group: "([^"]+)"\}/g)].map(m=>({code:m[1],description:m[2],group:m[3]}));
  assert.deepEqual(registry,expected);
});
