import assert from 'node:assert/strict';
import test from 'node:test';
import {readFileSync} from 'node:fs';
import ts from 'typescript';
const compile = path => ts.transpileModule(readFileSync(new URL(path,import.meta.url),'utf8'),{compilerOptions:{module:ts.ModuleKind.CommonJS}}).outputText;
const origin={exports:{}};new Function('exports',compile('../src/lib/auth/request-origin.ts'))(origin.exports);
function route(calls){const loaded={exports:{}};new Function('require','exports',compile('../src/app/api/tender-fms/[[...path]]/route.ts'))(name=>name.includes('request-origin')?origin.exports:{proxyAuthenticatedRequest:async(request,context,resource,methods)=>{if(!methods.has(request.method))return new Response(null,{status:405});calls.push({path:(await context.params).path,resource});return new Response(null,{status:204})}},loaded.exports);return loaded.exports}
const draft='11111111-1111-4111-8111-111111111111',member='22222222-2222-4222-8222-222222222222';
test('document deletion requires same origin and cannot reach arbitrary resources',async()=>{
 const calls=[],r=route(calls),path=['drafts',draft,'documents',member],context={params:Promise.resolve({path})};
 const request=origin=>new Request('http://localhost/api/tender-fms/'+path.join('/'),{method:'DELETE',headers:{Host:'localhost',Origin:origin}});
 assert.equal((await r.DELETE(request('http://foreign.invalid'),context)).status,403);assert.equal(calls.length,0);
 assert.equal((await r.DELETE(request('http://localhost'),context)).status,204);assert.deepEqual(calls,[{path,resource:'tender-fms'}]);
 for(const extra of [['users',member],['drafts',draft,'documents',member,'arbitrary']])assert.equal((await r.DELETE(request('http://localhost'),{params:Promise.resolve({path:extra})})).status,405);
});
