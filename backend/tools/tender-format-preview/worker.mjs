// Private offline renderer. It receives only signed, application-generated DOCX.
// The existing Go reader remains an independent child until its own migration.
import http from 'node:http';
import {extractPDF} from './pdf-text.mjs';
import {createHmac,timingSafeEqual} from 'node:crypto';
import {mkdtemp,writeFile,readFile,rm} from 'node:fs/promises';
import {join} from 'node:path';
import {tmpdir} from 'node:os';
import {execFile,spawn} from 'node:child_process';
import {promisify} from 'node:util';
const execute=promisify(execFile);process.umask(0o077);
let busy=false,closing=false;
const reader=spawn('/app/document-worker',[],{stdio:'inherit'});
reader.once('exit',()=>{if(!closing)process.exit(1)});
const server=http.createServer(async(req,res)=>{let dir,claimed=false;const controller=new AbortController();res.once('close',()=>{if(!res.writableEnded)controller.abort()});try{
 const native=req.url==='/pdf-native',extraction=native||req.url==='/pdf-text';
 if((req.url!=='/'&&!extraction)||req.method!=='POST'){res.writeHead(404);return res.end()}
 const key=process.env.EPCERP_DOCUMENT_READING_KEY??'',at=req.headers['x-render-time'],signature=req.headers['x-render-signature'];
 if(!/^[a-f\d]{64}$/i.test(key)||typeof at!=='string'||!/^\d{13}$/.test(at)||Math.abs(Date.now()-Number(at))>60000||typeof signature!=='string'||!/^[a-f\d]{64}$/.test(signature)){res.writeHead(403);return res.end()}
 if(busy){res.writeHead(503);return res.end()}
 const parts=[];let size=0;for await(const part of req){size+=part.length;if(size>(extraction?32:8)*1048576){res.writeHead(413);res.end();return}parts.push(part)}const data=Buffer.concat(parts),expected=createHmac('sha256',Buffer.from(key,'hex')).update((native?'epcerp-pdf-native-v1':extraction?'epcerp-pdf-text-v1':'epcerp-format-preview-v1')+'\n'+at+'\n').update(data).digest();if(!timingSafeEqual(expected,Buffer.from(signature,'hex'))||(extraction?data.subarray(0,5).toString()!=='%PDF-':data.subarray(0,2).toString()!=='PK')){res.writeHead(403);return res.end()}
 if(busy){res.writeHead(503);return res.end()}busy=true;claimed=true;dir=await mkdtemp(join(tmpdir(),'epcerp-format-'));if(extraction){const deadline=AbortSignal.any([controller.signal,AbortSignal.timeout(10*60*1000)]);const result=await extractPDF(data,dir,deadline,native);res.writeHead(200,{'Content-Type':'application/json','Cache-Control':'no-store'});res.end(JSON.stringify(result));return}await writeFile(join(dir,'format.docx'),data,{mode:0o600});const conversion=execute('/usr/bin/soffice',['-env:UserInstallation=file://'+join(dir,'profile'),'--headless','--nologo','--nodefault','--norestore','--convert-to','pdf:writer_pdf_Export','--outdir',dir,join(dir,'format.docx')],{timeout:45000,maxBuffer:1048576,detached:true,signal:controller.signal,env:{PATH:'/usr/bin:/bin',HOME:dir,XDG_CACHE_HOME:join(dir,'cache'),XDG_CONFIG_HOME:join(dir,'config'),LANG:'C.UTF-8'}});try{await conversion}finally{if(conversion.child?.pid)try{process.kill(-conversion.child.pid,'SIGKILL')}catch{}}const pdf=await readFile(join(dir,'format.pdf'));if(pdf.length>16*1048576||pdf.subarray(0,5).toString()!=='%PDF-')throw Error('Invalid preview');res.writeHead(200,{'Content-Type':'application/pdf','Cache-Control':'no-store'});res.end(pdf);
 }catch(error){console.error(JSON.stringify({event:'format.preview.failed',code:typeof error?.code==='number'?error.code:'render_unavailable'}));if(!res.headersSent)res.writeHead(503);res.end()}finally{if(dir)await rm(dir,{recursive:true,force:true}).catch(()=>{});if(claimed)busy=false;}});
server.requestTimeout=60000;server.headersTimeout=10000;server.maxConnections=8;server.listen(8082,'0.0.0.0');
for(const signal of['SIGTERM','SIGINT'])process.once(signal,()=>{closing=true;server.close();reader.kill(signal);setTimeout(()=>process.exit(0),3000).unref()});
