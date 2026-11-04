import assert from "node:assert/strict";
import { randomUUID } from "node:crypto";
import test from "node:test";
import { initializeAuth } from "../../src/server/auth/bootstrap";
import { login } from "../../src/server/auth/sessions";
import { createRole, updateRole } from "../../src/server/auth/administration";
import { hashPassword } from "../../src/server/auth/security";
import { pool } from "../../src/server/db";
import { handleMaster } from "../../src/server/master/http";

test("Clients migration on the existing owned PostgreSQL schema",{skip:!process.env.EPCERP_DATABASE_URL},async t=>{
  assert.match(new URL(process.env.EPCERP_DATABASE_URL!).pathname,/\/epcerp_(auth|master)_qa_/u,"Refusing to write test fixtures to an application database");
  try {
  await initializeAuth();
  const token=(await login("superadmin",process.env.VTPL_BOOTSTRAP_SUPERADMIN_PASSWORD!)).access_token;
  const prefix=`clientqa${randomUUID().replace(/-/g,"").slice(0,10)}`;
  const ids={type:randomUUID(),industry:randomUUID(),state:"",city:randomUUID(),otherState:"",otherCity:randomUUID()};
  await pool().query("INSERT INTO public.client_types(id,name,normalized_name,code,is_active) VALUES($1,$2,$2,$2,true)",[ids.type,`${prefix}.type`]);
  await pool().query("INSERT INTO public.industries(id,name,normalized_name,code,is_active) VALUES($1,$2,$2,$2,true)",[ids.industry,`${prefix}.industry`]);
  for(const [key,name] of [["state","Maharashtra"],["otherState","Punjab"]] as const){
    const record=await pool().query("INSERT INTO public.indian_states(id,name,normalized_name,code,is_active) VALUES($1,$2::text,lower($2::text),$3,true) ON CONFLICT(normalized_name) DO UPDATE SET is_active=true RETURNING id",[randomUUID(),name,`${prefix}.${key}`]);
    ids[key]=record.rows[0].id;
  }
  for(const [id,state] of [[ids.city,ids.state],[ids.otherCity,ids.otherState]])await pool().query("INSERT INTO public.indian_cities(id,state_id,name,normalized_name,code,is_active) VALUES($1,$2,$3,$3,$3,true)",[id,state,prefix]);
  const readerRole=await createRole(token,{name:`${prefix}.reader`,description:"QA client reader",permission_codes:["settings.clients.read"]});
  const readerID=randomUUID(),password="Client-Reader1!";
  await pool().query("INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser) VALUES($1,$2,$3,$4,true,false)",[readerID,prefix,`${prefix}@fixture.invalid`,await hashPassword(password)]);
  await pool().query("INSERT INTO auth.user_roles(user_id,role_id) VALUES($1,$2)",[readerID,readerRole.id]);
  const readerToken=(await login(prefix,password)).access_token;
  const input=(name=`${prefix} customer`) => ({legal_name:name,client_type_id:ids.type,industry_id:ids.industry,state_id:ids.state,city_id:ids.city,
    pan:"AAACW4202F",gstin:"27AAACW4202F1ZQ",address_line_1:"Synthetic address",postal_code:"400001",website:"example.com",
    contacts:[{full_name:"QA Contact",email:"PERSON@EXAMPLE.COM",phone:"+91 98765 43210",contact_type:"EIC",is_primary:true}]});
  async function request(method:string,path:string,body?:unknown,access=token){
    return handleMaster(new Request(`http://localhost/api/v1${path}`,{method,headers:{"Content-Type":"application/json",...(access?{Authorization:`Bearer ${access}`}:{})},body:body===undefined?undefined:JSON.stringify(body)}));
  }
  async function json(method:string,path:string,body?:unknown,access=token,status=200){
    const r=await request(method,path,body,access),v=await r.json();assert.equal(r.status,status,JSON.stringify(v));return v;
  }
  let client:Awaited<ReturnType<typeof json>>;
    await t.test("server permissions deny anonymous writes and read-only users",async()=>{
      await json("GET","/clients",undefined,"",401);
      await json("POST","/clients",input(),readerToken,403);
      await json("GET","/clients",undefined,readerToken);
      assert.equal((await pool().query("SELECT count(*) FROM clients WHERE legal_name=$1",[input().legal_name])).rows[0].count,"0");
    });
    await t.test("create uses selected references, preserves the response contract and records actors",async()=>{
      client=await json("POST","/clients",{...input(),client_type:"forged",state:"forged"},token,201);
      assert.equal(client.client_type,`${prefix}.type`);assert.equal(client.state,"Maharashtra");assert.equal(client.website,"https://example.com");
      assert.match(client.client_code,/^CLI-[A-F0-9]{12}$/);assert.equal(client.is_active,true);
      assert.deepEqual(client.missing_contact_types,["HOD","SITE_CONTACT"]);assert.equal(client.contacts[0].email,"person@example.com");
      assert.equal(client.legacy_source,undefined);assert.equal(client.normalized_legal_name,undefined);
      const row=(await pool().query("SELECT created_by_id,updated_by_id FROM clients WHERE id=$1",[client.id])).rows[0];assert.equal(row.created_by_id,row.updated_by_id);assert.ok(row.created_by_id);
      await json("GET",`/clients/{${client.id.toUpperCase()}}`,undefined,readerToken);
    });
    await t.test("lists retain filter, pagination, summary and all contact fields",async()=>{
      const result=await json("GET",`/clients?search=${prefix}&page_size=1&sort=client_code_desc`);
      assert.equal(result.total,1);assert.equal(result.page_size,1);assert.equal(result.items[0].id,client.id);
      assert.equal(result.all_count,result.active_count+result.inactive_count);
      assert.equal(result.items[0].contacts[0].contact_type,"EIC");
      await json("GET","/clients?sort=invalid",undefined,token,422);
    });
    await t.test("inactive/mismatched reference selections cannot create clients",async()=>{
      await json("POST","/clients",{...input(`${prefix} foreign city`),city_id:ids.otherCity},token,422);
      await json("POST","/clients",{...input(`${prefix} wrong state`),state_id:ids.otherState,city_id:ids.otherCity},token,422);
      await pool().query("UPDATE client_types SET is_active=false WHERE id=$1",[ids.type]);
      try {await json("POST","/clients",input(`${prefix} inactive type`),token,422);}
      finally {await pool().query("UPDATE client_types SET is_active=true WHERE id=$1",[ids.type]);}
    });
    await t.test("legacy Go names, timestamps, IDs and provenance survive reads and edits",async()=>{
      const legacyID=randomUUID(),name=`${prefix} Straße Σς`;
      const row=await pool().query(`INSERT INTO clients(id,client_code,legal_name,normalized_legal_name,display_name,parent_organization,client_type,industry,gstin,pan,cin,website,address_line_1,address_line_2,city,state,postal_code,country,is_active,legacy_source,legacy_source_id,legacy_source_row,created_at)
        VALUES($1,$2,$3,$4,$3,'','type','industry','27AAACW4202F1ZQ','AAACW4202F','','','Address','','City','Maharashtra','400001','India',true,'go-qa-fixture',$5,42,'2026-10-09T08:00:00.123456Z') RETURNING id`,[legacyID,`CLI-${randomUUID().replace(/-/g,"").slice(0,12).toUpperCase()}`,name,`${prefix} straße σσ`,prefix]);
      assert.equal(row.rows[0].id,legacyID);
      const old=await json("GET",`/clients/${legacyID}`);assert.equal(old.created_at,"2026-10-09T08:00:00.123456Z");
      await json("POST","/clients",input(`${prefix.toUpperCase()} STRAẞE ΣΣ`),token,409);
      await json("PATCH",`/clients/${legacyID}`,{...input(`${prefix} legacy updated`),contacts:[]});
      const kept=(await pool().query("SELECT client_code,legacy_source,legacy_source_id,legacy_source_row FROM clients WHERE id=$1",[legacyID])).rows[0];
      assert.equal(kept.client_code,old.client_code);assert.equal(kept.legacy_source,"go-qa-fixture");assert.equal(kept.legacy_source_id,prefix);assert.equal(kept.legacy_source_row,42);
    });
    await t.test("contacts retain IDs, reject cross-client ownership and retire omitted rows",async()=>{
      const contactID=client.contacts[0].id;
      const update={...input(),contacts:[{...input().contacts[0],id:contactID,full_name:"Updated QA Contact"}]};
      client=await json("PATCH",`/clients/${client.id}`,update);assert.equal(client.contacts[0].id,contactID);
      await json("PATCH",`/clients/${client.id}`,{...input("Must not save"),contacts:[{...input().contacts[0],id:randomUUID()}]},token,422);
      assert.equal((await json("GET",`/clients/${client.id}`)).legal_name,input().legal_name);
      client=await json("PATCH",`/clients/${client.id}`,{...input(),contacts:[]});
      assert.equal(client.contacts[0].id,contactID);assert.equal(client.contacts[0].is_active,false);assert.equal(client.missing_contact_types.length,3);
    });
    await t.test("deactivate retires contacts; activate never resurrects them",async()=>{
      client=await json("POST",`/clients/${client.id}/deactivate`);assert.equal(client.is_active,false);
      const page=await json("GET",`/clients?search=${prefix}&status=inactive`);assert.ok(page.items.some((v:{id:string})=>v.id===client.id));
      client=await json("POST",`/clients/${client.id}/activate`);assert.equal(client.is_active,true);assert.ok(client.contacts.every((c:{is_active:boolean})=>!c.is_active));
    });
    await t.test("contact persistence failure rolls back the client header",async()=>{
      await pool().query("ALTER TABLE client_contacts ADD CONSTRAINT epcerp_qa_contact_failure CHECK(full_name <> 'Reject fixture') NOT VALID");
      try {
        await json("POST","/clients",{...input(`${prefix} rollback`),contacts:[{...input().contacts[0],full_name:"Reject fixture"}]},token,503);
        assert.equal((await pool().query("SELECT count(*) FROM clients WHERE legal_name=$1",[`${prefix} rollback`])).rows[0].count,"0");
      } finally {await pool().query("ALTER TABLE client_contacts DROP CONSTRAINT epcerp_qa_contact_failure");}
    });
    await t.test("revoked role grants affect existing sessions immediately",async()=>{
      await updateRole(token,readerRole.id,{permission_codes:[],expected_version:readerRole.edit_version});
      await json("GET","/clients",undefined,readerToken,403);
      await pool().query("UPDATE auth.users SET is_active=false WHERE id=$1",[readerID]);
      await json("GET","/clients",undefined,readerToken,401);
    });
    await t.test("payloads are bounded before parsing and unknown paths stay unpublished",async()=>{
      const r=await handleMaster(new Request("http://localhost/api/v1/clients",{method:"POST",body:"x".repeat(128*1024+1)}));assert.equal(r.status,413);
      await json("GET","/clients/arbitrary/path",undefined,token,404);
      const result=await json("POST","/clients",{...input(),created_by_id:randomUUID()},token,422);assert.ok(Array.isArray(result.detail));
      assert.equal((await pool().query("SELECT version FROM schema_migrations")).rows[0].version,"98");
    });
  } finally {await pool().end();}
});
