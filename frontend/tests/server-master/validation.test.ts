import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import test from "node:test";
import { parseClient, parseFilter } from "../../src/server/master/clients/validation";
import { normalizedName, lower, upper } from "../../src/server/master/shared/text";
import { canonicalID } from "../../src/server/master/shared/validation";

const id="00112233-4455-4677-8899-aabbccddeeff";
export const input = () => ({legal_name:"Example Client",client_type_id:id,industry_id:id,state_id:id,city_id:id,address_line_1:"Address",postal_code:"201001",pan:"AAACW4202F",gstin:"27AAACW4202F1ZQ",contacts:[{full_name:"Test Contact",email:"CLIENT@EXAMPLE.COM",phone:"+91 98765 43210"}]});
test("client name keys and email lowercase retain Go's Unicode semantics",()=>{
  assert.equal(normalizedName("  Straße   Σς  "),"straße σσ");
  assert.equal(normalizedName("ſ K ẞ"),"s k ß");
  assert.notEqual(normalizedName("Straße"),normalizedName("Strasse"));
  assert.equal(lower("İ ΟΣ"),"i οσ");
  assert.equal(upper("ß ı ſ"),"ß I S");
  assert.equal(normalizedName("\u0085 A\u0085B\u0085"),"a b");
  assert.equal(normalizedName("\ufeffA"),"\ufeffa");
});
test("canonical IDs accept legacy Go forms without changing persisted IDs",()=>{
  for(const value of [id,id.toUpperCase(),`{${id}}`,`urn:uuid:${id}`,id.replace(/-/g,"")])assert.equal(canonicalID(value),id);
  assert.throws(()=>canonicalID("../users"));
});
test("client validation rejects caller-owned identity and provenance fields",()=>{
  for(const key of ["is_active","created_by_id","legacy_source","normalized_legal_name","Legal_name"])
    assert.throws(()=>parseClient({...input(),[key]:"forged"}));
  for(const key of ["gstin","pan","address_line_1","city_id","contacts"]){const value:Record<string,unknown>=input();delete value[key];assert.throws(()=>parseClient(value));}
});
test("client filters bound pagination and whitelist sort identifiers",()=>{
  assert.equal(parseFilter(new URLSearchParams("sort=client_code_desc&page=2&page_size=10")).descending,true);
  for(const query of ["sort=name;drop table clients","page_size=101","page=2147483647&page_size=100","status=deleted","search="])
    assert.throws(()=>parseFilter(new URLSearchParams(query)));
});
test("native validation matches actual Go parsing across compatibility fixtures",{skip:!process.env.EPCERP_MASTER_COMPATIBILITY_BINARY},()=>{
  const fixtures:unknown[]=[input(),{...input(),website:"example.com"},{...input(),website:null},{...input(),contacts:[]},{...input(),legal_name:"  Straße   Σς  ",display_name:""}];
  for(const field of ["pan","gstin","postal_code","state_id","contacts","address_line_1"]){const v:Record<string,unknown>=input();delete v[field];fixtures.push(v);}
  for(const website of ["javascript:alert(1)","https://","https://name:pass@example.com","https://example.com:99999"])fixtures.push({...input(),website});
  for(const contact_type of ["","EIC","HOD","SITE_CONTACT","UNKNOWN"])fixtures.push({...input(),contacts:[{...input().contacts[0],contact_type}]});
  for(const phone of ["+91 98765 43210","09876543210","+12025550123","+91 123","0000000000"])fixtures.push({...input(),contacts:[{...input().contacts[0],phone}]});
  for(const email of ["İ@EXAMPLE.COM","ΟΣ@EXAMPLE.COM","\ufeffX@example.com"])fixtures.push({...input(),contacts:[{...input().contacts[0],email}]});
  const alphabet="0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ",prefix="27SSACW4202F1Z";
  let sum=0;for(let i=0;i<prefix.length;i++){const n=alphabet.indexOf(prefix[i])*(1+i%2);sum+=Math.floor(n/36)+n%36;}
  const gstin=prefix+alphabet[(36-sum%36)%36];
  fixtures.push({...input(),pan:"SSACW4202F",gstin});
  fixtures.push({...input(),pan:"ßACW4202F",gstin:gstin.replace("SS","ß")});
  const result=spawnSync(process.env.EPCERP_MASTER_COMPATIBILITY_BINARY!,["validate"],{input:JSON.stringify(fixtures),encoding:"utf8"});
  assert.equal(result.status,0,result.stderr);
  const expected=JSON.parse(result.stdout);
  fixtures.forEach((value,index)=>{
    let actual:Record<string,unknown>;
    try {const c=parseClient(value);actual={valid:true,field:"",legal_name:c.legal_name,display_name:c.display_name,normalized_name:normalizedName(c.legal_name),website:c.website,gstin:c.gstin,pan:c.pan,country:c.country,...(c.contacts.length?{email:c.contacts[0].email,contact_type:c.contacts[0].contact_type}:{})};}
    catch {actual={valid:false};}
    assert.equal(actual.valid,expected[index].valid,`fixture ${index}`);
    if(actual.valid)assert.deepEqual(actual,expected[index],`fixture ${index}`);
  });
  const table=spawnSync(process.env.EPCERP_MASTER_COMPATIBILITY_BINARY!,[],{encoding:"utf8"});assert.equal(table.status,0);
  const stored=JSON.parse(readFileSync(new URL("../../src/server/master/shared/simple-fold.json",import.meta.url),"utf8"));
  assert.deepEqual(stored,JSON.parse(table.stdout));
});
