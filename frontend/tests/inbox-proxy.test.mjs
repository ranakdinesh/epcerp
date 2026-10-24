import assert from 'node:assert/strict';
import test from 'node:test';
import {readFileSync} from 'node:fs';
import ts from 'typescript';
const source=readFileSync(new URL('../src/app/api/inbox/[[...path]]/route.ts',import.meta.url),'utf8');
const {outputText}=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022}});
function route(calls){const loaded={exports:{}};new Function('require','exports',outputText)(()=>({proxyAuthenticatedRequest:async(request,context,resource,methods,limit)=>{calls.push({method:request.method,path:(await context.params).path,resource,methods:[...methods],limit});return Response.json({ok:true})}}),loaded.exports);return loaded.exports}
const id='11111111-1111-4111-8111-111111111111';
test('Personal inbox proxy exposes only bounded authenticated notice and receipt endpoints',async()=>{const calls=[],r=route(calls);for(const [method,path]of [['GET',[]],['GET',[id]],['POST',[id,'read']],['POST',[id,'acknowledge']]]){const result=await r[method](new Request('http://localhost/api/inbox',{method}),{params:Promise.resolve({path})});assert.equal(result.status,200)}assert.equal(calls.length,4);for(const c of calls){assert.equal(c.resource,'inbox');assert.deepEqual(c.methods,['GET','POST']);assert.equal(c.limit,4096)}});
test('Personal inbox cannot forward arbitrary recipients, delivery commands or source mutation routes',async()=>{const calls=[],r=route(calls);for(const [method,path]of [['POST',[]],['POST',[id]],['GET',[id,'read']],['POST',[id,'delete']],['GET',['users',id]],['POST',[id,'read','extra']],['GET',['..']],['POST',['invalid','read']]]){const result=await r[method](new Request('http://localhost/api/inbox',{method}),{params:Promise.resolve({path})});assert.equal(result.status,404)}assert.equal(calls.length,0)});
