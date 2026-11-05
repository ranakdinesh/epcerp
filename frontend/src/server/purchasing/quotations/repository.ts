import type { DB } from '../../db';
import { timeSQL,timestamp } from '../../inbox/repository';
import type { QuoteRecord,Quote,Attachment } from './types';
import { PurchaseError } from '../core/validation';
import { journal } from '../core/journal';
import { randomUUID } from 'node:crypto';
import { now } from '../core/time';
const fields=`project_id,bo_id,version,quotes,is_active,${timeSQL('created_at')} AS created_at,${timeSQL('updated_at')} AS updated_at,created_by_id,updated_by_id`;
const fileFields=`id,document_id,filename,content_type,byte_size,sha256,is_active,${timeSQL('created_at')} AS created_at,created_by_id`;
export const stale=()=>new PurchaseError(409,'Quotations changed; reload before saving');
const view=(r:QuoteRecord)=>({...r,created_at:timestamp(r.created_at)!,updated_at:timestamp(r.updated_at)!});
export const fileView=(r:Record<string,unknown>):Attachment=>({id:String(r.id),document_id:String(r.document_id),filename:String(r.filename),content_type:String(r.content_type),byte_size:Number(r.byte_size),sha256:String(r.sha256),is_active:r.is_active===true,created_at:timestamp(r.created_at as string)!,created_by_id:String(r.created_by_id??'00000000-0000-0000-0000-000000000000')});
export async function get(db:DB,project:string,bo:string){const row=(await db.query(`SELECT ${fields} FROM purchase_group_quotations WHERE project_id=$1 AND bo_id=$2`,[project,bo])).rows[0];return row?view(row):null;}
export async function save(db:DB,record:QuoteRecord,actor:string){
    const values=[record.bo_id,record.project_id,JSON.stringify(record.quotes),actor],result=record.version===1?await db.query(`INSERT INTO purchase_group_quotations(bo_id,project_id,version,quotes,is_active,created_by_id,updated_by_id)VALUES($1,$2,1,$3,true,$4,$4)RETURNING ${fields}`,values):await db.query(`UPDATE purchase_group_quotations SET quotes=$3,updated_by_id=$4,version=version+1,updated_at=clock_timestamp() WHERE bo_id=$1 AND project_id=$2 AND version=$5 RETURNING ${fields}`,[...values,record.version-1]);
    if(!result.rowCount)throw stale();return view(result.rows[0]);
}
export async function event(db:DB,before:QuoteRecord|null,after:QuoteRecord,reason:string,actor:string){
    const id=randomUUID(),old=new Map(before?.quotes.map(q=>[q.document_id,q.revision])??[]);
    for(const q of after.quotes)if(q.revision>(old.get(q.document_id)??0))await db.query('INSERT INTO purchase_quotation_revisions(document_id,revision,bo_id,attachment_id,quote,created_by_id)VALUES($1,$2,$3,$4,$5,$6)',[q.document_id,q.revision,after.bo_id,q.attachment_id,JSON.stringify(q),actor]);
    await db.query("INSERT INTO purchase_quotation_events(id,bo_id,action,reason,before_snapshot,after_snapshot,created_by_id)VALUES($1,$2,'quote_saved',$3,$4,$5,$6)",[id,after.bo_id,reason,JSON.stringify(before),JSON.stringify(after),actor]);await journal(db,{id,module:'group_quotations',type:'purchase_group_quotations',entity:after.bo_id,version:after.version,action:'quote_saved',actor,project:after.project_id,at:after.updated_at});
}
export async function revisions(db:DB,project:string,bo:string):Promise<Quote[]>{return(await db.query('SELECT r.quote FROM purchase_quotation_revisions r JOIN purchase_group_quotations q ON q.bo_id=r.bo_id WHERE q.project_id=$1 AND q.bo_id=$2 ORDER BY r.created_at,r.document_id,r.revision',[project,bo])).rows.map(r=>r.quote);}
export async function history(db:DB,project:string,bo:string){return(await db.query(`SELECT e.id,e.bo_id,e.action,e.reason,e.before_snapshot AS before,e.after_snapshot AS after,e.created_by_id,${timeSQL('e.created_at')} AS created_at FROM purchase_quotation_events e JOIN purchase_group_quotations q ON q.bo_id=e.bo_id WHERE q.project_id=$1 AND q.bo_id=$2 ORDER BY e.created_at DESC,e.id LIMIT 200`,[project,bo])).rows.map(r=>({...r,created_at:timestamp(r.created_at),created_by_id:r.created_by_id??'00000000-0000-0000-0000-000000000000'}));}
export async function attachments(db:DB,document:string){return(await db.query(`SELECT ${fileFields} FROM purchase_quotation_attachments WHERE document_id=$1 ORDER BY created_at,id`,[document])).rows.map(fileView);}
export async function attachment(db:DB,document:string,id:string){const r=(await db.query(`SELECT ${fileFields},ciphertext FROM purchase_quotation_attachments WHERE document_id=$1 AND id=$2`,[document,id])).rows[0];return r?{metadata:fileView(r),encrypted:r.ciphertext as Buffer}:null;}
export async function insertAttachment(db:DB,document:string,name:string,mime:string,bytes:number,sha256:string,encrypted:Buffer,actor:string){const row=(await db.query(`INSERT INTO purchase_quotation_attachments(id,document_id,filename,content_type,byte_size,sha256,ciphertext,is_active,created_by_id,updated_by_id)VALUES($1,$2,$3,$4,$5,$6,$7,true,$8,$8)RETURNING ${fileFields}`,[randomUUID(),document,name,mime,bytes,sha256,encrypted,actor])).rows[0];return fileView(row);}
export async function attachmentEvent(db:DB,file:Attachment,action:string,actor:string){const id=randomUUID();await db.query('INSERT INTO purchase_quotation_evidence_events(id,attachment_id,action,created_by_id)VALUES($1,$2,$3,$4)',[id,file.id,action,actor]);const p=(await db.query('SELECT project_id FROM purchase_indent_documents WHERE id=$1',[file.document_id])).rows[0];await journal(db,{id,module:'group_quotations',type:'purchase_quotation_attachment',entity:file.id,version:Number((await db.query('SELECT count(*)::text AS n FROM purchase_quotation_evidence_events WHERE attachment_id=$1',[file.id])).rows[0].n),action,actor,project:p.project_id,at:now()});}
