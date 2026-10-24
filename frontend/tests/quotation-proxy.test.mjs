import assert from 'node:assert/strict';
import test from 'node:test';
import {readFileSync} from 'node:fs';
import ts from 'typescript';
const source=readFileSync(new URL('../src/app/api/projects/[[...path]]/route.ts',import.meta.url),'utf8');
const {outputText}=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022}});
function route(calls){const loaded={exports:{}};new Function('require','exports',outputText)(()=>({proxyAuthenticatedRequest:async(request,context,resource,methods,limit)=>{calls.push({method:request.method,path:(await context.params).path,resource,methods:[...methods],limit});return Response.json({ok:true});}}),loaded.exports);return loaded.exports;}
test('Accounts review admits bounded multilingual comments without increasing other draft limits',async()=>{const calls=[],r=route(calls);for(const path of [['p','purchase-bo','b','quotation-approvals','review'],['p','purchase-bo','b','quotation-approvals','submit'],['p','budget']])await r.POST(new Request('http://localhost/api/projects',{method:'POST'}),{params:Promise.resolve({path})});assert.deepEqual(calls.map(c=>c.limit),[1024*1024,512*1024,512*1024]);assert.ok(calls.every(c=>c.resource==='projects'&&c.methods.join(',')==='GET,POST,PATCH'));});
test('technical uploads alone receive bounded evidence limit and authorization preflight',async()=>{
 const calls=[],loaded={exports:{}};
 new Function('require','exports',outputText)(()=>({proxyAuthenticatedRequest:async(request,context,resource,methods,limit,preflight)=>{calls.push({limit,preflight});return Response.json({ok:true});}}),loaded.exports);
 for(const path of [['p','purchase-workflow','pkg','technical','uploads','document','3'],['p','purchase-workflow','pkg','technical']]) await loaded.exports.POST(new Request('http://localhost/api/projects',{method:'POST'}),{params:Promise.resolve({path})});
 assert.deepEqual(calls,[{limit:11*1024*1024,preflight:true},{limit:64*1024,preflight:false}]);
});
