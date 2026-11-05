import 'server-only';
import {createHmac} from 'node:crypto';
import {pool} from '../../db';
import {encrypt,decrypt} from './crypto';
import type {Original,Question} from './types';
export type PDFText={reader_version:string;pages:{page:number;text:string;method:string}[];warnings:{page:number;code:string}[]};
export async function loadDocumentText(doc:Original,signal?:AbortSignal,nativeOnly=false):Promise<PDFText>{
 const revision=nativeOnly?'pdf-native-v1':'pdf-text-v1',db=pool(),cached=(await db.query("SELECT payload_cipher,payload_hash,source_sha256 FROM tender.tsc_document_text WHERE member_id=$1 AND reader_version=ANY($2::text[]) ORDER BY reader_version='pdf-text-v1' DESC LIMIT 1",[doc.id,nativeOnly?['pdf-native-v1','pdf-text-v1']:[revision]])).rows[0];
 if(cached){if(cached.source_sha256!==doc.sha256)throw Error('source_integrity_failed');return decrypt<PDFText>(cached.payload_cipher,cached.payload_hash)}
 const base=new URL(process.env.EPCERP_TENDER_FORMAT_RENDER_URL??'http://document-processor:8082');
 if(base.protocol!=='http:'||!['document-processor','document-worker','localhost','127.0.0.1'].includes(base.hostname)&&!/^epcerp-[a-z0-9-]+$/.test(base.hostname)||base.username||base.password)throw Error('invalid_local_reader');
 const at=String(Date.now()),key=process.env.EPCERP_DOCUMENT_READING_KEY??'';
 if(!/^[a-f\d]{64}$/i.test(key))throw Error('invalid_local_reader');
 const signature=createHmac('sha256',Buffer.from(key,'hex')).update('epcerp-'+revision+'\n'+at+'\n').update(doc.data).digest('hex');
 const r=await fetch(new URL(nativeOnly?'/pdf-native':'/pdf-text',base),{method:'POST',headers:{'Content-Type':'application/pdf','X-Render-Time':at,'X-Render-Signature':signature},body:new Uint8Array(doc.data),signal:AbortSignal.any([AbortSignal.timeout(10*60*1000),...(signal?[signal]:[])]),redirect:'error'});
 if(!r.ok)throw Error('document_text_extraction_failed');
 const reader=r.body?.getReader();if(!reader)throw Error('document_text_extraction_failed');let size=0;const parts:Uint8Array[]=[];
 try{while(true){const {done,value}=await reader.read();if(done)break;size+=value.length;if(size>10*1048576){await reader.cancel();throw Error('document_text_extraction_failed')}parts.push(value)}}finally{reader.releaseLock()}
 const text=JSON.parse(Buffer.concat(parts).toString()) as PDFText;
 if(text.reader_version!==revision||!Array.isArray(text.pages)||text.pages.length!==doc.pages||text.pages.some((p,i)=>p.page!==i+1||typeof p.text!=='string'||!['native','ocr'].includes(p.method)))throw Error('document_text_extraction_failed');
 const sealed=encrypt(text);await db.query('INSERT INTO tender.tsc_document_text(member_id,reader_version,source_sha256,payload_cipher,payload_hash)VALUES($1,$2,$3,$4,$5)ON CONFLICT DO NOTHING',[doc.id,revision,doc.sha256,sealed.cipher,sealed.hash]);return text;
}
export const loadNativeDocumentText=(doc:Original,signal?:AbortSignal)=>loadDocumentText(doc,signal,true);
const stop=new Set('the is a an of to for in and or what which whether are as by this that does do be with from kindly confirm please required any provide details subject mentioned'.split(' '));
function words(s:string){return [...new Set(s.toLowerCase().match(/[\p{L}\p{N}]{2,}/gu)??[])].filter(w=>!stop.has(w)).map(w=>w.length>4&&w.endsWith('s')?w.slice(0,-1):w)}
function queryWords(s:string){const aliases=[[ /oem/i,'manufacturer dealership authorized authorised warranty certificate testing'],[/emd|bid security|startup/i,'earnest money bid security startup dipp dpiit exemption'],[/joint venture|consortium|\bjv\b/i,'jv joint venture consortium allowed permitted'],[/hard copy|physical|offline/i,'original guarantee delivered person working deadline closing submission'],[/sd.?pbg|bank guarantee/i,'performance pg security deposit validity completion maintenance'],[/experience|similar work/i,'experience completion work order bill tds testimonial certificate'],[/payment|milestone/i,'on account payment supply commissioning retention instalment warranty'],[/form|format/i,'annexure declaration neft certificate self certification'],[/licen[cs]e/i,'license licence statutory voltage validity renewal'],[/surety/i,'insurance surety guarantee'],[/location|site visit/i,'site station locality inspection visit'],[/turnover|turn.over/i,'annual contractual turnover audited balance annexure vib'] ]as const;return words(s+' '+aliases.filter(([re])=>re.test(s)).map(([,terms])=>terms).join(' '))}
export function selectEvidence(docs:{doc:Original;text:PDFText}[],questions:Question[],maxBytes:number){
 const chunks=docs.flatMap(({doc,text})=>text.pages.flatMap(p=>{
  const chunks=[];for(let at=0;at<p.text.length;at+=4000)chunks.push({document_id:doc.id,page:p.page,text:p.text.slice(at,at+4500),method:p.method});return chunks;
 }));
 const totalPages=docs.reduce((n,d)=>n+d.text.pages.length,0);
 if(chunks.reduce((n,c)=>n+Buffer.byteLength(JSON.stringify(c)),0)<=maxBytes)return {pages:chunks,partial:false,total_pages:totalPages,supplied_pages:new Set(chunks.map(p=>p.document_id+':'+p.page)).size};
 const chosen=new Set<number>(),evidence:typeof chunks=[];let bytes=0;
 const tokens=chunks.map(c=>new Set(words(c.text))),frequency=new Map<string,number>();for(const terms of tokens)for(const term of terms)frequency.set(term,(frequency.get(term)??0)+1);const average=tokens.reduce((n,t)=>n+t.size,0)/Math.max(1,tokens.length);
 const rankings=questions.map(q=>{const terms=queryWords(q.question+' '+q.section);return chunks.map((_,i)=>({i,score:terms.reduce((n,t)=>n+(tokens[i].has(t)?Math.log(1+(chunks.length-(frequency.get(t)??0)+.5)/((frequency.get(t)??0)+.5)):0),0)/(1+.25*tokens[i].size/Math.max(1,average))})).sort((a,b)=>b.score-a.score||a.i-b.i)});
 // Cover each question before adding secondary excerpts. Never silently clip a page excerpt.
 for(let depth=0;depth<chunks.length;depth++){
  let added=false;
  for(const ranked of rankings){const hit=ranked[depth];if(!hit||hit.score===0||chosen.has(hit.i))continue;const c=chunks[hit.i],size=Buffer.byteLength(JSON.stringify(c));if(bytes+size>maxBytes)continue;chosen.add(hit.i);evidence.push(c);bytes+=size;added=true}
  if(!added&&bytes>maxBytes-5000)break;
 }
 // Empty keyword overlap still supplies bounded evidence instead of an empty model request.
 if(!evidence.length)for(let i=0;i<chunks.length;i++){const size=Buffer.byteLength(JSON.stringify(chunks[i]));if(bytes+size>maxBytes)break;evidence.push(chunks[i]);chosen.add(i);bytes+=size}
 return {pages:evidence,partial:chosen.size<chunks.length,total_pages:docs.reduce((n,d)=>n+d.text.pages.length,0),supplied_pages:new Set(evidence.map(p=>p.document_id+':'+p.page)).size};
}
export function checkTextCitations(content:unknown,evidence:ReturnType<typeof selectEvidence>){
 const result=content as {answers?:{status?:string;citations?:{document_id:string;page:number;quote:string}[]}[]};
 if(!Array.isArray(result?.answers))return content;
 const normalized=(s:string)=>s.replace(/\s+/g,' ').trim();
 for(const a of result.answers){
  if(a.status==='not_found'&&evidence.partial)a.status='needs_review';
  if(a.citations?.some(c=>typeof c.quote!=='string'||!normalized(c.quote)||!evidence.pages.some(p=>p.document_id===c.document_id&&p.page===c.page&&normalized(p.text).includes(normalized(c.quote)))))a.status='needs_review';
 }
 return content;
}
