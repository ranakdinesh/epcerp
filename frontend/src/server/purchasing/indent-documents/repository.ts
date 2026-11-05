import type { DB } from '../../db';
import { timeSQL,timestamp } from '../../inbox/repository';
import type { Document,Data } from './types';
import { journal } from '../core/journal';
import { PurchaseError } from '../core/validation';
import { randomUUID } from 'node:crypto';
const fields=`id,project_id,bo_id,group_id,vendor_id,version,data,is_active,${timeSQL('created_at')} AS created_at,${timeSQL('updated_at')} AS updated_at,created_by_id,updated_by_id`;
export const stale=()=>new PurchaseError(409,'Vendor indent or groups changed; reload before saving');
const view=(r:Document)=>({...r,created_at:timestamp(r.created_at)!,updated_at:timestamp(r.updated_at)!});
export async function list(db:DB,project:string,bo:string){
    const rows=(await db.query(`SELECT ${fields} FROM purchase_indent_documents WHERE project_id=$1 AND bo_id=$2 ORDER BY created_at,id`,[project,bo])).rows as Document[];
    const legacy=(await db.query('SELECT l.document_id,l.terms FROM purchase_indent_legacy_terms l JOIN purchase_indent_documents d ON d.id=l.document_id WHERE d.project_id=$1 AND d.bo_id=$2',[project,bo])).rows;
    return rows.map(r=>{const d=view(r);if(!d.data.terms){const terms=legacy.find(l=>l.document_id===d.id)?.terms;if(terms)d.data.terms=terms;else delete d.data.terms;}return d;});
}
export async function save(db:DB,doc:Document,actor:string){
    doc.data.is_current=false;const data={...doc.data};if(data.terms?.source==='legacy_template_v1')delete data.terms;
    const result=doc.version===1?await db.query(`INSERT INTO purchase_indent_documents(id,project_id,bo_id,group_id,vendor_id,scope_revision,version,data,is_active,created_by_id,updated_by_id)VALUES($1,$2,$3,$4,$5,$6,1,$7,true,$8,$8)RETURNING ${fields}`,[doc.id,doc.project_id,doc.bo_id,doc.group_id,doc.vendor_id,doc.data.scope_revision,JSON.stringify(data),actor]):await db.query(`UPDATE purchase_indent_documents SET version=version+1,data=$4,updated_at=clock_timestamp(),updated_by_id=$5 WHERE id=$1 AND project_id=$2 AND bo_id=$3 AND version=$6 RETURNING ${fields}`,[doc.id,doc.project_id,doc.bo_id,JSON.stringify(data),actor,doc.version-1]);
    if(!result.rowCount)throw stale();const saved=view(result.rows[0]);if(!saved.data.terms&&doc.data.terms)saved.data.terms=doc.data.terms;return saved;
}
export async function event(db:DB,before:Document|null,after:Document,action:string,reason:string,actor:string){const id=randomUUID();await db.query('INSERT INTO purchase_indent_document_events(id,document_id,action,reason,before_snapshot,after_snapshot,created_by_id)VALUES($1,$2,$3,$4,$5,$6,$7)',[id,after.id,action,reason,JSON.stringify(before),JSON.stringify(after),actor]);await journal(db,{id,module:'purchase_indent_documents',type:'purchase_indent_document',entity:after.id,version:after.version,action,actor,project:after.project_id,at:after.updated_at});}
export async function history(db:DB,project:string,bo:string){return(await db.query(`SELECT e.id,e.document_id,e.action,e.reason,e.before_snapshot AS before,e.after_snapshot AS after,e.created_by_id,${timeSQL('e.created_at')} AS created_at FROM purchase_indent_document_events e JOIN purchase_indent_documents d ON d.id=e.document_id WHERE d.project_id=$1 AND d.bo_id=$2 ORDER BY e.created_at DESC,e.id LIMIT 200`,[project,bo])).rows.map(r=>({...r,created_at:timestamp(r.created_at),created_by_id:r.created_by_id??'00000000-0000-0000-0000-000000000000'}));}
export async function notify(db:DB,document:string,recipients:string[],actor:string){for(const id of recipients)await db.query('INSERT INTO purchase_indent_dispatch_notices(id,document_id,recipient_id,created_by_id)VALUES($1,$2,$3,$4)ON CONFLICT(document_id,recipient_id)DO NOTHING',[randomUUID(),document,id,actor]);}
export async function resolve(db:DB,document:string,actor:string){await db.query('UPDATE purchase_indent_dispatch_notices SET resolved_at=clock_timestamp(),updated_by_id=$2 WHERE document_id=$1 AND resolved_at IS NULL',[document,actor]);}
export async function notices(db:DB,actor:string){return(await db.query(`SELECT n.id,d.project_id,d.bo_id,d.data,${timeSQL('n.created_at')} AS created_at FROM purchase_indent_dispatch_notices n JOIN purchase_indent_documents d ON d.id=n.document_id WHERE n.recipient_id=$1 AND n.resolved_at IS NULL ORDER BY n.created_at DESC,n.id LIMIT 200`,[actor])).rows.map(r=>({id:r.id,project_id:r.project_id,bo_id:r.bo_id,number:(r.data as Data).number,group:(r.data as Data).group_name,vendor:(r.data as Data).vendor.name,created_at:timestamp(r.created_at)}));}
