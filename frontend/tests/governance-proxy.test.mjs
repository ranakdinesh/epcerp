import assert from "node:assert/strict";
import test from "node:test";
import {readFileSync} from "node:fs";
import ts from "typescript";
const source=readFileSync(new URL('../src/app/api/governance/[[...path]]/route.ts',import.meta.url),'utf8');
const {outputText}=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022}});
function route(calls){const loaded={exports:{}};new Function('require','exports',outputText)(()=>({proxyAuthenticatedRequest:async(request,context,resource,methods,limit)=>{calls.push({method:request.method,path:(await context.params).path,resource,methods:[...methods],limit});return Response.json({ok:true})}}),loaded.exports);return loaded.exports}
test('Director proxy exposes only owned endpoints with a bounded protected payload',async()=>{const calls=[],r=route(calls);for(const [method,path]of [['GET',undefined],['POST',[]],['GET',['options']]]){const result=await r[method](new Request('http://localhost/api/governance',{method}),{params:Promise.resolve({path})});assert.equal(result.status,200)}assert.equal(calls.length,3);for(const c of calls){assert.equal(c.resource,'administration/director');assert.deepEqual(c.methods,['GET','POST']);assert.equal(c.limit,16*1024)}});
test('Director options cannot become an arbitrary administration forwarding route',async()=>{const calls=[],r=route(calls);for(const [method,path]of [['GET',['users']],['GET',['options','users']],['POST',['options']],['GET',['..']]]){const result=await r[method](new Request('http://localhost/api/governance',{method}),{params:Promise.resolve({path})});assert.equal(result.status,404)}assert.equal(calls.length,0)});
