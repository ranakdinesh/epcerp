// Browser plugin unavailable; regular Playwright exercises only the disposable fixture.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import {pathToFileURL} from 'node:url';
import {createHash} from 'node:crypto';
const {chromium}=await import(pathToFileURL(path.join(process.env.EPCERP_PLAYWRIGHT_MODULE,'index.mjs')).href);
const browser=await chromium.launch({executablePath:process.env.EPCERP_BROWSER_EXECUTABLE,headless:true,args:['--no-proxy-server','--host-resolver-rules=MAP epcerp.test 127.0.0.1']});
const output=process.env.EPCERP_HTTP_PROOF_DIR||'/private/tmp/epcerp-http-proof';fs.mkdirSync(output,{recursive:true});
try {
 for(const width of [1440,390]) {
  const page=await browser.newPage({viewport:{width,height:900}});const errors=[];page.on('pageerror',e=>errors.push(e.message));
  await page.goto('http://epcerp.test:9093/tf102-preview');await page.getByRole('button',{name:'Add tender draft',exact:true}).waitFor();
  assert.deepEqual(await page.evaluate(()=>[isSecureContext,typeof crypto.randomUUID,typeof crypto.subtle]),[false,'undefined','undefined']);
  await page.evaluate(()=>{
   const original=fetch;const draft={id:'abcd0000-0000-4000-8000-000000000001',name:'HTTP tender test',version:1,created_at:new Date().toISOString(),updated_at:new Date().toISOString(),batches:[]};
   window.httpFixture={mode:'timeout',requests:[],hashes:[]};
   const response=body=>new Response(JSON.stringify(body),{headers:{'Content-Type':'application/json'}});
   window.fetch=async(input,init)=>{
    const p=new URL(input,location.href).pathname.replace('/api/tender-fms/','');
    if(p==='drafts'&&init?.method==='POST'){
     httpFixture.requests.push({key:new Headers(init.headers).get('Idempotency-Key'),body:JSON.parse(init.body)});
     if(httpFixture.mode==='timeout')return new Promise((resolve,reject)=>{init.signal.addEventListener('abort',()=>reject(init.signal.reason),{once:true});});
     return response(draft);
    }
    if(p==='drafts')return response({items:httpFixture.mode==='success'?[draft]:[],total:httpFixture.mode==='success'?1:0});
    if(p.endsWith('/batches'))return new Response(null,{status:204});
    if(p.startsWith('drafts/'))return response(draft);
    if(p==='receipts'&&init?.method==='POST')httpFixture.hashes.push(...JSON.parse(init.body).files.map(f=>f.sha256));
    return original(input,init);
   };
  });
  await page.getByRole('button',{name:'Add tender draft',exact:true}).click();await page.getByLabel('Tender name').fill('HTTP tender test');
  // An unavailable RNG must be reported, and must release the busy state.
  await page.evaluate(()=>{window.rng=crypto.getRandomValues.bind(crypto);Object.defineProperty(crypto,'getRandomValues',{configurable:true,value:undefined});});
  await page.getByRole('button',{name:'Create draft',exact:true}).click();await page.getByRole('alert').filter({hasText:'Secure random number generation is unavailable'}).waitFor();
  assert.equal(await page.getByRole('button',{name:'Create draft',exact:true}).isEnabled(),true);
  await page.evaluate(()=>Object.defineProperty(crypto,'getRandomValues',{configurable:true,value:window.rng}));
  await page.getByRole('button',{name:'Create draft',exact:true}).click();
  await page.getByRole('alert').filter({hasText:'within 30 seconds'}).waitFor({timeout:35000});
  await page.screenshot({path:path.join(output,`timeout-${width}.png`)});
  await page.evaluate(()=>{httpFixture.mode='success';});await page.getByRole('button',{name:'Retry same request',exact:true}).click();
  const panel=page.getByRole('dialog',{name:'HTTP tender test',exact:true});await panel.waitFor();
  const requests=await page.evaluate(()=>httpFixture.requests);assert.equal(requests.length,2);assert.equal(requests[0].key,requests[1].key);assert.deepEqual(requests[0].body,requests[1].body);assert.match(requests[0].key,/^[a-f0-9-]{36}$/);
  const content='%PDF-synthetic HTTP upload';await panel.locator('input[type=file]').setInputFiles({name:'synthetic.pdf',mimeType:'application/pdf',buffer:Buffer.from(content)});
  await panel.getByRole('button',{name:'Upload',exact:true}).click();await panel.getByText('Uploaded. Verification and reading will continue in the background.',{exact:true}).waitFor();
  assert.deepEqual(await page.evaluate(()=>httpFixture.hashes),[createHash('sha256').update(content).digest('hex')]);
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true);assert.deepEqual(errors,[]);
  await page.screenshot({path:path.join(output,`success-${width}.png`)});await page.close();console.log(`PASS HTTP ${width}: RNG error, real 30s timeout, same-key retry, created drawer, checksum and upload; no page errors`);
 }
} finally {await browser.close();}
