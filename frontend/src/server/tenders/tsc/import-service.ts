import 'server-only';
import{randomUUID}from'node:crypto';
import{transaction,type DB}from'../../db';
import{AppError}from'../../errors';
import{authorizeTender,sourceFingerprint}from'../documents';
import{parseWord}from'./word-import';
import{blank,id,integer,object,str}from'./validation';
import{encrypt,decrypt,digest}from'./crypto';
import{decodeJob,event,getJob}from'./repository';
import{get}from'./service';
import type{Question,Snapshot,JobRow}from'./types';
type Input={version:number;formatID:string;formatVersion:number;name:string;data:Buffer;sha256:string};
function input(value:unknown){const r=object(value,['expected_version','format_id','format_version','filename','document','mapping','reviewed','skip_unmatched_confirmed']);const name=str(r.filename,240);if(!/\.docx$/i.test(name)||/[\x00-\x1f/\\]/.test(name))throw new AppError(422,'Choose a .docx Word file');const encoded=str(r.document,5592408),data=Buffer.from(encoded,'base64');if(data.length>4*1024*1024||data.toString('base64')!==encoded)throw new AppError(422,'Upload a valid Word file up to 4 MiB');return{r,version:integer(r.expected_version,1),formatID:id(r.format_id),formatVersion:integer(r.format_version,1),name,data,sha256:digest(data)}}
async function format(db:DB,x:Input){const row=(await db.query('SELECT id,name,version FROM tender.tsc_formats WHERE id=$1 AND is_active FOR SHARE',[x.formatID])).rows[0];if(!row||Number(row.version)!==x.formatVersion)throw new AppError(409,'Checklist format changed; preview the document again');const questions=(await db.query<Question>('SELECT id,section,printed_number,question,manual_review FROM tender.tsc_format_questions WHERE format_id=$1 ORDER BY position',[x.formatID])).rows;return{row,questions}}
export async function previewImport(token:string,draft:string,value:unknown){
 const x=input(value);const loaded=await transaction(async db=>{const {draft:d}=await authorizeTender(db,token,draft,'write');if(d.version!==x.version)throw new AppError(409,'Tender changed; refresh before importing');return format(db,x)});return{...parseWord(x.data,loaded.questions),filename:x.name,sha256:x.sha256,source_version:x.version,questions:loaded.questions,format_version:x.formatVersion};
}
export async function commitImport(token:string,draft:string,key:string,value:unknown){
 id(key);const x=input(value);if(typeof x.r.reviewed!=='boolean'||typeof x.r.skip_unmatched_confirmed!=='boolean')throw new AppError(422,'Confirm the import review choices');
 if(!Array.isArray(x.r.mapping)||x.r.mapping.length>200)throw new AppError(422,'Choose valid question matches');
 const mapping=x.r.mapping.map(v=>{const r=object(v,['row_id','question_id']);return{row_id:integer(r.row_id,1,240),question_id:str(r.question_id,80)}}).sort((a,b)=>a.row_id-b.row_id);
 if(new Set(mapping.map(m=>m.question_id)).size!==mapping.length)throw new AppError(422,'Each checklist question can be matched only once');
 const hash=digest(JSON.stringify({version:x.version,format:x.formatID,formatVersion:x.formatVersion,name:x.name,sha256:x.sha256,mapping,reviewed:x.r.reviewed,skip:x.r.skip_unmatched_confirmed}));
 const savedID=await transaction(async db=>{
  const {actor,draft:d}=await authorizeTender(db,token,draft,'write');await db.query('SELECT id FROM tender.document_drafts WHERE id=$1 FOR UPDATE',[draft]);
  const previous=(await db.query<JobRow>('SELECT * FROM tender.tsc_workspaces WHERE draft_id=$1 AND actor_id=$2 AND operation_key=$3',[draft,actor.id,key])).rows[0];if(previous){if(previous.request_hash!==hash)throw new AppError(409,'This operation key belongs to another import');return previous.id}
  if(d.version!==x.version)throw new AppError(409,'Tender changed; preview the document again');
  const {row:f,questions}=await format(db,x),parsed=parseWord(x.data,questions),rows=new Map(parsed.rows.map(r=>[r.id,r]));
  for(const m of mapping)if(!rows.has(m.row_id)||!questions.some(q=>q.id===m.question_id))throw new AppError(422,'An import match is no longer valid');
  if(parsed.rows.some(r=>r.answer&&!mapping.some(m=>m.row_id===r.id))&&!x.r.skip_unmatched_confirmed)throw new AppError(422,'Match all supplied answers or confirm skipping unmatched rows');
  const matched=mapping.filter(m=>rows.get(m.row_id)!.answer.trim());if(!matched.length)throw new AppError(422,'Match at least one filled answer before saving');
  const retained=Number((await db.query('SELECT (SELECT count(*) FROM tender.tsc_jobs WHERE draft_id=$1)+(SELECT count(*) FROM tender.tsc_workspaces WHERE draft_id=$1) AS count',[draft])).rows[0].count);if(retained>=50)throw new AppError(422,'This tender has reached its 50-draft retention limit');
  const snapshot:Snapshot={schema:'tsc-cloud-v2',draft_id:draft,draft_version:x.version,name:d.name,format_id:x.formatID,format_version:x.formatVersion,format_name:f.name,questions,sources:[],mode:'manual',auto_escalate:false,source_fingerprint:sourceFingerprint([]),preparation_method:'upload',imported_document:{name:x.name,sha256:x.sha256,rows:parsed.rows.length,matched:matched.length}};
  const answers=questions.map(q=>{const m=matched.find(m=>m.question_id===q.id);return m?{...blank(q),answer:rows.get(m.row_id)!.answer,status:x.r.reviewed?'answered' as const:'needs_review' as const,accepted:x.r.reviewed===true,origin:'upload' as const}:blank(q)}),a=encrypt(answers),s=encrypt(snapshot),job=randomUUID();
  const row=(await db.query<JobRow>(`INSERT INTO tender.tsc_workspaces(id,draft_id,draft_version,actor_id,operation_key,request_hash,mode,state,snapshot_cipher,snapshot_hash,answers_cipher,answers_hash)VALUES($1,$2,$3,$4,$5,$6,'manual','ready',$7,$8,$9,$10)RETURNING *`,[job,draft,x.version,actor.id,key,hash,s.cipher,s.hash,a.cipher,a.hash])).rows[0];
  await event(db,row,'word_imported',`${matched.length} answers imported from Word${x.r.reviewed?'; supplied answers explicitly reviewed by uploader':'; answers awaiting review'}`,answers,actor.id,{name:x.name,sha256:x.sha256,base64:x.data.toString('base64'),mapping});return job;
 });return get(token,draft,savedID);
}
export function uploadedDocument(token:string,draft:string,job:string){return transaction(async db=>{await authorizeTender(db,token,draft,'export');const row=await getJob(db,draft,job);if(decodeJob(row).snapshot.preparation_method!=='upload')throw new AppError(404,'No imported Word document for this draft');const retained=(await db.query("SELECT payload_cipher,payload_hash FROM tender.tsc_workspace_events WHERE job_id=$1 AND action='word_imported' ORDER BY version LIMIT 1",[job])).rows[0];if(!retained)throw new AppError(404,'Imported document not found');const value=decrypt<{document:{name:string;sha256:string;base64:string}}>(retained.payload_cipher,retained.payload_hash).document,data=Buffer.from(value.base64,'base64');if(digest(data)!==value.sha256)throw new Error('Retained import integrity failed');return{name:value.name,data}})}
