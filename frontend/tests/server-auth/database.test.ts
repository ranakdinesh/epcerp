import assert from "node:assert/strict";
import { randomUUID } from "node:crypto";
import { readFileSync } from "node:fs";
import test from "node:test";
import { initializeAuth } from "../../src/server/auth/bootstrap";
import { pool, transaction } from "../../src/server/db";
import { handleAuth } from "../../src/server/auth/http";
import { login, refresh, me, logout } from "../../src/server/auth/sessions";
import { sha256 } from "../../src/server/auth/security";
import { accountEvent } from "../../src/server/auth/repository";
import { deliverCredentialMail, openMail } from "../../src/server/auth/mail";

const database = process.env.EPCERP_DATABASE_URL;
test("native auth against owned PostgreSQL schema", { skip: !database }, async t => {
  assert.match(new URL(database!).pathname, /\/epcerp_auth_qa_/ , "Refusing to run mutation tests against an application database");
  const fixture = JSON.parse(readFileSync(process.env.EPCERP_AUTH_COMPATIBILITY_FILE!,"utf8"));
  await initializeAuth();
  const bootstrap = await login("superadmin",process.env.VTPL_BOOTSTRAP_SUPERADMIN_PASSWORD!);
  const adminToken = bootstrap.access_token;
  const prefix = `qa${randomUUID().replace(/-/g,"").slice(0,12)}`;
  const id = randomUUID();
  await transaction(async db => {
    await db.query("DELETE FROM auth.login_attempts WHERE identifier_hash=ANY($1::text[])",[[sha256(`login:${id}`),...['login','/auth/forgot-password','/auth/reset-password','/auth/verify-email'].map(v=>sha256(`http:${v}`))]]);
    await db.query("INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser,first_name,last_name,mobile) VALUES($1,$2,$3,$4,true,false,'Test','Employee','9000000000')",[id,prefix,`${prefix}@fixture.invalid`,fixture.hashes[0]]);
    await db.query("INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser) VALUES($1,$2,$3,$4,true,false) ON CONFLICT(id) DO UPDATE SET username=EXCLUDED.username,email=EXCLUDED.email,password_hash=EXCLUDED.password_hash,is_active=true,must_change_password=false,credential_version=0",[fixture.id,`${prefix}.compat`,`${prefix}.compat@fixture.invalid`,fixture.hashes[0]]);
  });
  async function http(method: string,path: string,body?: unknown,token=adminToken) {
    return handleAuth(new Request(`http://localhost/api/v1${path}`,{method,headers:{"Content-Type":"application/json",...(token ? {Authorization:`Bearer ${token}`} : {})},body:body === undefined ? undefined : JSON.stringify(body)}));
  }
  async function json(method:string,path:string,body?:unknown,token=adminToken,status=200) {
    const response = await http(method,path,body,token);
    const value = response.status===204 ? null : await response.json();
    assert.equal(response.status,status,JSON.stringify(value)); return value;
  }
  const fetchBefore = globalThis.fetch;
  const sent: Array<{ subject:string; textContent:string; to:Array<{email:string}> }> = [];
  // Every outbound request in this process is intercepted; no real emails/providers.
  globalThis.fetch = async (url,init) => {
    assert.equal(String(url),"https://api.brevo.com/v3/smtp/email");
    sent.push(JSON.parse(String(init?.body))); return Response.json({messageId:"synthetic"}, {status:201});
  };
  try {
    await t.test("existing hashes are unchanged by bootstrap and login; Go tokens resolve", async () => {
      assert.equal((await me(fixture.token)).id,fixture.id);
      const before = (await pool().query("SELECT password_hash FROM auth.users WHERE id=$1",[id])).rows[0].password_hash;
      const pair = await login(prefix,fixture.password);
      assert.equal((await me(pair.access_token)).username,prefix);
      await initializeAuth();
      assert.equal((await pool().query("SELECT password_hash FROM auth.users WHERE id=$1",[id])).rows[0].password_hash,before);
    });
    await t.test("refresh rotates once, preserves lifetime, and replay revokes replacement access", async () => {
      const p = await login(prefix,fixture.password), rotated = await refresh(p.refresh_token);
      assert.ok(rotated.refresh_expires_in <= p.refresh_expires_in);
      assert.notEqual(rotated.refresh_token,p.refresh_token);
      await assert.rejects(refresh(p.refresh_token),{status:401});
      await assert.rejects(me(rotated.access_token),{status:401});
      await assert.rejects(refresh(rotated.refresh_token),{status:401});
    });
    await t.test("concurrent refresh cannot create two surviving sessions", async () => {
      const p = await login(prefix,fixture.password);
      const results = await Promise.allSettled([refresh(p.refresh_token),refresh(p.refresh_token)]);
      assert.equal(results.filter(r=>r.status==="fulfilled").length,1);
      for (const r of results) if (r.status === "fulfilled") await assert.rejects(me(r.value.access_token),{status:401});
    });
    await t.test("logout immediately revokes access", async () => {
      const p = await login(prefix,fixture.password); await logout(p.refresh_token);
      await assert.rejects(me(p.access_token),{status:401});
    });
    await t.test("username updates preserve password and sessions, detect stale edits and keep history", async () => {
      const p=await login(prefix,fixture.password), name=`${prefix}.renamed`;
      assert.equal((await json("GET",`/auth/me/username?username=${name}`,undefined,p.access_token)).available,true);
      await json("PATCH","/auth/me/username",{username:name,expected_username:prefix},p.access_token,204);
      assert.equal((await me(p.access_token)).username,name);
      await json("PATCH","/auth/me/username",{username:prefix,expected_username:prefix},p.access_token,409);
      await json("PATCH","/auth/me/username",{username:prefix,expected_username:name},p.access_token,204);
      assert.equal((await me((await login(prefix,fixture.password)).access_token)).id,id);
      assert.ok((await pool().query("SELECT count(*) FROM auth.auth_username_changes WHERE user_id=$1",[id])).rows[0].count>=2);
    });
    await t.test("role changes apply to existing tokens and cannot grant system admin", async () => {
      const r = await json("POST","/roles",{name:`${prefix}.reader`,description:"QA reader",permission_codes:["admin.users.read"]},adminToken,201);
      let u = await json("GET",`/users/${id}`);
      u = await json("PATCH",`/users/${id}`,{role_names:[r.name],expected_version:u.edit_version});
      const p = await login(prefix,fixture.password);
      await json("GET","/users",undefined,p.access_token);
      await json("POST","/roles",{name:`${prefix}.bad`,permission_codes:["system:admin"]},adminToken,422);
      await json("POST","/roles",{name:`${prefix}.forged`,permission_codes:["admin.users.write"]},p.access_token,403);
      const saved = await json("PATCH",`/roles/${r.id}`,{permission_codes:[],expected_version:r.edit_version});
      await json("GET","/users",undefined,p.access_token,403);
      await json("PATCH",`/roles/${r.id}`,{permission_codes:["admin.users.write"],expected_version:r.edit_version},adminToken,409);
      assert.ok(saved.edit_version>r.edit_version);
      await json("PATCH",`/users/${id}`,{first_name:"Wrong",expected_version:u.edit_version-1},adminToken,409);
    });
    await t.test("deactivation invalidates access and refresh permanently, and queues handover", async () => {
      const p = await login(prefix,fixture.password);
      await json("POST",`/users/${id}/status`,{is_active:false,reason:"Synthetic access revocation test"});
      await assert.rejects(me(p.access_token),{status:401}); await assert.rejects(refresh(p.refresh_token),{status:401});
      assert.equal((await pool().query("SELECT count(*) FROM auth.auth_offboarding_cases WHERE user_id=$1",[id])).rows[0].count,"1");
      await json("POST",`/users/${id}/status`,{is_active:true,reason:"Synthetic access restoration test"});
      await assert.rejects(me(p.access_token),{status:401}); await assert.rejects(refresh(p.refresh_token),{status:401});
      const own = (await me(adminToken)).id;
      await json("POST",`/users/${own}/status`,{is_active:false,reason:"Synthetic self deactivation test"},adminToken,422);
    });
    await t.test("session listing marks current session and prevents another user's revocation", async () => {
      const p = await login(prefix,fixture.password), q = await login(prefix,fixture.password);
      const sessions = await json("GET","/auth/me/sessions",undefined,p.access_token);
      const current = sessions.find((s:{current:boolean})=>s.current);
      assert.ok(current);
      await json("DELETE",`/auth/me/sessions/${current.id}`,undefined,adminToken,403);
      await json("DELETE",`/auth/me/sessions/${current.id}`,undefined,p.access_token,204);
      await assert.rejects(me(p.access_token),{status:401}); assert.equal((await me(q.access_token)).id,id);
      await json("DELETE","/auth/me/sessions",undefined,q.access_token,204);
      await assert.rejects(me(q.access_token),{status:401});
    });
    await t.test("new credentials queue encrypted mail, enforce first change and revoke old tokens", async () => {
      const input = {username:`${prefix}.new`,email:`${prefix}.new@fixture.invalid`,first_name:"New",last_name:"Employee",mobile:"9000000001",role_names:[`${prefix}.reader`]};
      await json("POST","/users",input,adminToken,503);
      assert.equal((await pool().query("SELECT count(*) FROM auth.users WHERE username=$1",[input.username])).rows[0].count,"0");
      process.env.EPCERP_EMAIL_DELIVERY_ENABLED="true";process.env.EPCERP_BREVO_API_KEY="synthetic-only";process.env.EPCERP_EMAIL_SENDER_EMAIL="sender@fixture.invalid";
      const u = await json("POST","/users",input,adminToken,201);
      assert.equal(u.must_change_password,true); assert.equal(u.password_hash,undefined);
      const [job] = (await pool().query("SELECT * FROM auth.auth_credential_emails WHERE user_id=$1",[u.id])).rows;
      const mail = JSON.parse(openMail(job.payload,job.payload_hash).toString());
      const password = /Temporary password: (\S+)/.exec(mail.Message)![1];
      const p = await login(input.username,password);
      assert.deepEqual((await me(p.access_token)).permissions,[]);
      await json("GET","/users",undefined,p.access_token,403);
      await json("POST","/auth/me/password",{current_password:password,password:"Changed-Password1!"},p.access_token,204);
      await assert.rejects(me(p.access_token),{status:401});
      assert.equal((await me((await login(input.username,"Changed-Password1!")).access_token)).must_change_password,false);
      assert.equal(await deliverCredentialMail(),false,"stale welcome mail must be cancelled");
      await json("POST",`/users/${u.id}/password`,{reason:"Synthetic admin password reset"},adminToken,204);
      assert.equal(await deliverCredentialMail(),true);
      assert.ok(sent.some(v=>v.subject==="Your epcERP temporary password"));
      assert.equal((await pool().query("SELECT payload FROM auth.auth_credential_emails WHERE user_id=$1 AND state='delivered'",[u.id])).rows[0].payload,null);
    });
    await t.test("email changes require proof, stay unchanged before confirmation and revoke sessions", async () => {
      const p = await login(prefix,fixture.password), before = await me(p.access_token), address=`${prefix}.verified@fixture.invalid`;
      await json("PATCH","/auth/me/email",{current_password:fixture.password,email:address},p.access_token,204);
      assert.equal((await me(p.access_token)).email,before.email);
      const raw = /#token=([A-Za-z0-9_-]+)/.exec(sent.at(-1)!.textContent)![1];
      await json("POST","/auth/verify-email",{token:raw},"",204);
      await assert.rejects(me(p.access_token),{status:401});
      await json("POST","/auth/verify-email",{token:raw},"",422);
      assert.equal((await me((await login(address,fixture.password)).access_token)).email,address);
    });
    await t.test("recovery is single-use, invalidates sessions and does not enumerate users", async () => {
      const p = await login(prefix,fixture.password), email=(await me(p.access_token)).email;
      const known=await json("POST","/auth/forgot-password",{email},"",202), count=sent.length;
      const raw=/#token=([A-Za-z0-9_-]+)/.exec(sent.at(-1)!.textContent)![1];
      const unknown=await json("POST","/auth/forgot-password",{email:"nobody@fixture.invalid"},"",202);
      assert.deepEqual(known,unknown);assert.equal(sent.length,count);
      await json("POST","/auth/reset-password",{token:raw,password:"Recovered-Password1!"},"",204);
      await assert.rejects(me(p.access_token),{status:401});
      await json("POST","/auth/reset-password",{token:raw,password:"Recovered-Password2!"},"",422);
      assert.equal((await me((await login(prefix,"Recovered-Password1!")).access_token)).id,id);
    });
    await t.test("failed logins commit lockout and cannot bypass it using email", async () => {
      const email=(await pool().query("SELECT email FROM auth.users WHERE id=$1",[id])).rows[0].email;
      for(let i=0;i<5;i++) await assert.rejects(login(prefix,"incorrect-password"),{status:401});
      await assert.rejects(login(email,"Recovered-Password1!"),{status:401});
      assert.ok((await pool().query("SELECT blocked_until FROM auth.login_attempts WHERE identifier_hash=$1",[sha256(`login:${id}`)])).rows[0].blocked_until);
    });
    await t.test("audits share transactions, contain no password hashes, and are immutable", async () => {
      const events=await pool().query("SELECT * FROM business_events WHERE entity_id=$1 AND source_module='auth'",[id]);assert.ok(events.rowCount!>0);
      const history=await json("GET",`/auth/administration-history?entity_id=${id}`);assert.ok(history.length>0);
      assert.doesNotMatch(JSON.stringify(history),/password_hash|\$argon2/);
      await assert.rejects(pool().query("DELETE FROM auth.administration_changes WHERE entity_id=$1",[id]));
      const previous=(await pool().query("SELECT account_version FROM auth.users WHERE id=$1",[id])).rows[0].account_version;
      await assert.rejects(transaction(async db=>{await accountEvent(db,id,id,"synthetic_rolled_back");throw new Error("rollback");}));
      assert.equal((await pool().query("SELECT account_version FROM auth.users WHERE id=$1",[id])).rows[0].account_version,previous);
    });
  } finally {
    globalThis.fetch=fetchBefore;process.env.EPCERP_EMAIL_DELIVERY_ENABLED="false";
    await pool().end();
  }
});
