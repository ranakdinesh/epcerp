import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import ts from 'typescript';
const compile=path=>ts.transpileModule(readFileSync(new URL(path,import.meta.url),'utf8'),{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022}}).outputText;
test('native startup defaults every module to Next.js and rejects retired ownership',()=>{
 const saved={...process.env};try{
  for(const k of Object.keys(process.env))if(k.endsWith('_OWNER'))delete process.env[k];
  const exports={};new Function('require','exports',compile('../src/server/runtime.ts'))(()=>({}),exports);
  exports.assertNativeRuntime();assert.ok(exports.nativeOwners.length>15);
  for(const owner of exports.nativeOwners)assert.equal(process.env[`EPCERP_${owner}_OWNER`],'nextjs');
  process.env.EPCERP_AUTH_OWNER='go';assert.throws(exports.assertNativeRuntime,/retired/);
 }finally{for(const k of Object.keys(process.env))if(!(k in saved))delete process.env[k];Object.assign(process.env,saved);}
});
test('unmapped and foreign routes never make network fallback requests',async()=>{
 const saved={...process.env},fetch=globalThis.fetch;let calls=0;
 try{
  for(const name of ['AUTH','CLIENTS','REFERENCES','CATALOGUE','VENDOR_PROFILES','VENDOR_DOCUMENTS','VENDOR_VERIFICATION','TSC','TENDER_INTAKE','TENDER_REGISTER','INBOX','PERSONAL_TASKS','PROJECTS','PROJECT_TEAM','PURCHASING','PROJECT_BUDGETS','PLATFORM'])process.env[`EPCERP_${name}_OWNER`]='nextjs';
  delete process.env.EPCERP_GO_DISABLED;globalThis.fetch=async()=>{calls++;throw Error('Network fallback forbidden')};
  const exports={},none=()=>false;
  new Function('require','exports',compile('../src/lib/auth/backend-request.ts'))(name=>name.endsWith('backend-config')?{backendApiUrl:'http://127.0.0.1:1/api/v1'}:{ownsHistorical:none,ownsBudget:none,ownsPurchasing:none,ownsTeam:none,ownsProjects:none,ownsRegister:none,ownsIntake:none,ownsAuthPath:none,ownsMasterPath:none,ownsNativeSupplierPath:none},exports);
  for(const url of ['http://127.0.0.1:1/api/v1/unknown','https://outside.invalid/api/v1/unknown'])assert.equal((await exports.backendRequest(url)).status,404);
  assert.equal(calls,0);
 }finally{globalThis.fetch=fetch;for(const k of Object.keys(process.env))if(!(k in saved))delete process.env[k];Object.assign(process.env,saved);}
});
