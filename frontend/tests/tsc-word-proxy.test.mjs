import test from'node:test';import assert from'node:assert/strict';import fs from'node:fs';import ts from'typescript';
const source=fs.readFileSync(new URL('../src/app/api/tender-fms/[[...path]]/route.ts',import.meta.url),'utf8').replace(/^import .*;\n/gm,'').replace('async function route','export async function route');
let received;globalThis.__wordProxy=(...args)=>{received=args;return new Response(null,{status:204})};
const code=ts.transpileModule('const proxyAuthenticatedRequest=globalThis.__wordProxy;const hasSameOrigin=r=>r.headers.get("origin")==="http://localhost";'+source,{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022}}).outputText;
const{route}=await import('data:text/javascript;base64,'+Buffer.from(code).toString('base64'));const draft='11111111-1111-4111-8111-111111111111';
test('Word preview/commit have bounded large bodies and same-origin protection without broadening normal TSC writes',async()=>{
 for(const tail of ['import','import/commit']){const path=['drafts',draft,'tsc',...tail.split('/')];assert.equal((await route(new Request('http://localhost/api/tender-fms/'+path.join('/'),{method:'POST',headers:{origin:'http://elsewhere'}}),{params:Promise.resolve({path})})).status,403);await route(new Request('http://localhost/api/tender-fms/'+path.join('/'),{method:'POST',headers:{origin:'http://localhost'}}),{params:Promise.resolve({path})});assert.equal(received[4],6*1024*1024)}
 await route(new Request('http://localhost/api/tender-fms/drafts/'+draft+'/tsc',{method:'POST',headers:{origin:'http://localhost'}}),{params:Promise.resolve({path:['drafts',draft,'tsc']})});assert.equal(received[4],32*1024);
});
