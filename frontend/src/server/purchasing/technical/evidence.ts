import 'server-only';
import { randomUUID } from 'node:crypto';
import { transaction,type DB } from '../../db';
import { seal,open,putObject,readStoredObject,documentBucket } from '../../master/documents/files';
import { inspectTechnical } from '../core/evidence-inspector';
import { id } from '../sourcing/values';
import { text } from '../workflow/policy';
import { invalid } from '../core/validation';
import { now } from '../core/time';
import * as context from './context';
import * as repo from './repository';
import { uploadGuard,forbidden,missing } from './policy';
import type { File } from './types';
async function uploadScope(db:DB,token:string,p:string,idValue:string,q:{purpose:string;version:number}){const a=await context.actor(db,token);id(p);id(idValue);const caps=await context.access(db,a,p,idValue),record=await repo.get(db,p,idValue);if(!record)throw missing();const source=await context.source(db,p,idValue);if(source.project_id!==p||source.package_id!==idValue||record.project_id!==p||record.package_id!==idValue)throw missing();uploadGuard(record,a,caps,source,q);return{a,record};}
export async function authorizeUpload(token:string,p:string,record:string,q:{purpose:string;version:number}){await transaction(db=>uploadScope(db,token,p,record,q));}
export async function upload(token:string,p:string,record:string,q:{purpose:string;version:number;reason:string;name:string;media_type:string;data:Buffer}){
    const reason=text('reason',q.reason,2000);if(!q.data.length||q.data.length>10*1024*1024)invalid('file','Upload nonempty PDF or image evidence of at most 10 MiB');await authorizeUpload(token,p,record,q);await inspectTechnical(q.name,q.media_type,q.data);
    return transaction(async db=>{const{a,record:kit}=await uploadScope(db,token,p,record,q),encrypted=seal(q.data),file:File={id:randomUUID(),case_id:kit.id,purpose:q.purpose,name:q.name,media_type:q.media_type,size:q.data.length,sha256:encrypted.sha256,is_active:true},at=now(),bucket=documentBucket(),key='purchase-technical/'+p+'/'+kit.id+'/'+file.id;await putObject(bucket,key,encrypted.encrypted);await db.query('INSERT INTO purchase_technical_files(id,case_id,purpose,name,media_type,byte_size,sha256,ciphertext,object_bucket,object_key,is_active,created_at,updated_at,created_by_id,updated_by_id)VALUES($1,$2,$3,$4,$5,$6,$7,NULL,$8,$9,true,$10,$10,$11,$11)',[file.id,kit.id,q.purpose,file.name,file.media_type,file.size,file.sha256,bucket,key,at,a.id]);await repo.fileEvent(db,p,kit.id,file.id,'uploaded',reason,a.id,at);return file;});
}
export async function download(token:string,p:string,record:string,fileID:string){return transaction(async db=>{const a=await context.actor(db,token);id(p);id(record);id(fileID);const caps=await context.access(db,a,p,record);if(!caps.read)throw forbidden();const kit=await repo.get(db,p,record);if(!kit?.is_active||kit.project_id!==p||kit.package_id!==record)throw missing();const row=(await db.query('SELECT * FROM purchase_technical_files WHERE case_id=$1 AND id=$2 AND is_active',[kit.id,fileID])).rows[0];if(!row)throw missing();const encrypted=row.object_key?await readStoredObject(row.object_bucket,row.object_key):row.ciphertext,data=open(encrypted,row.sha256),file:File={id:row.id,case_id:kit.id,purpose:row.purpose,name:row.name,media_type:row.media_type,size:Number(row.byte_size),sha256:row.sha256,is_active:row.is_active};await repo.fileEvent(db,p,kit.id,file.id,'downloaded','Authorized technical evidence read',a.id,now());return{file,data};});}
