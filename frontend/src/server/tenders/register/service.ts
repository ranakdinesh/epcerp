import 'server-only';
import { randomUUID } from 'node:crypto';
import { transaction, lock, type DB } from '../../db';
import { AppError } from '../../errors';
import { tenderAssignmentNotice } from '../../inbox/tender-assignment';
import { digest } from '../tsc/crypto';
import { goJSON } from '../intake/receipts';
import { access, admit, capturePermissions, candidates, directory, canManage, forbidden, type Actor } from './access';
import { integer } from '../tsc/validation';
import { id, object, name, optional, version, reason, query, invalid } from './validation';
import { read, projection, visibleSQL, type RecordRow } from './repository';
import { recordEvent, journal } from './events';
import { consumeSources } from './sources';
async function operation(db: DB, actor: string, command: string, context: string, key: string, fp: string) {
    await lock(db,`${actor}:${command}:${context}:${key}`);
    const old = (await db.query('SELECT * FROM tender.tender_capture_operations WHERE actor_id=$1 AND command=$2 AND context_key=$3 AND idempotency_key=$4',[actor,command,context,key])).rows[0];
    if (old && old.fingerprint !== fp) throw new AppError(409,'Operation key was used with different input');
    return old;
}
async function save(db: DB, actor: string, command: string, context: string, key: string, fp: string, tender: string) {
    await db.query('INSERT INTO tender.tender_capture_operations(actor_id,command,context_key,idempotency_key,fingerprint,tender_id)VALUES($1,$2,$3,$4,$5,$6)',[actor,command,context,key,fp,tender]);
}
async function managerWork(db: DB, r: RecordRow, actor: string) {
    const action = randomUUID(), event = randomUUID();
    const a = (await db.query(`INSERT INTO tender.tender_source_actions(id,tender_id,assignment_id,recipient_id,source_event_id,assignment_version,created_at,updated_at,created_by_id,updated_by_id)
      VALUES($1,$2,$3,$4,$5,$6,clock_timestamp(),clock_timestamp(),$7,$7) RETURNING created_at::text AS at`,[action,r.id,r.manager_assignment_id,r.owner_id,event,r.assignment_version,actor])).rows[0];
    await journal(db,event,'tender_fms','tender_assignment',r.manager_assignment_id,r.assignment_version,'manager_assigned',actor,a.at);
    // Durable delivery intent only. This migration does not send email.
    await db.query('INSERT INTO tender.tender_email_jobs(id,action_id,recipient_id,created_by_id,updated_by_id)VALUES($1,$2,$3,$4,$4)',[randomUUID(),action,r.owner_id,actor]);
    const notice = await tenderAssignmentNotice(db,action);
    await db.query('UPDATE tender.tender_source_actions SET notice_id=$2 WHERE id=$1',[action,notice]);
}
export async function options(token: string, params: URLSearchParams) {
    await admit(token,'read',capturePermissions);
    const q = query(params,true);
    return transaction(async db => { await access(db,token,capturePermissions); const all = await candidates(db); return { items:all.slice((q.page-1)*q.size,q.page*q.size), total:all.length, page:q.page, page_size:q.size }; });
}
export async function get(token: string, target: string) {
    await admit(token,'read'); id(target);
    return transaction(async db => { const actor = await access(db,token); return projection(db,actor,await read(db,actor,target)); });
}
export async function list(token: string, params: URLSearchParams) {
    await admit(token,'read'); const q = query(params);
    return transaction(async db => {
        const actor = await access(db,token), values = [actor.allows('tenders.portfolio.read'),actor.id,q.search];
        const total = Number((await db.query(`SELECT count(*) FROM tender.tender_fms_intakes i WHERE ${visibleSQL}`,values)).rows[0].count);
        const order = q.sort === 'name' ? `i.title ${q.direction},i.created_at DESC,i.id` : `i.created_at ${q.direction},i.id`;
        const rows = (await db.query(`SELECT i.id FROM tender.tender_fms_intakes i WHERE ${visibleSQL} ORDER BY ${order} LIMIT $4 OFFSET $5`,[...values,q.size,(q.page-1)*q.size])).rows;
        const items = [];
        for (const r of rows) items.push(await projection(db,actor,await read(db,actor,r.id)));
        return { items,total,page:q.page,page_size:q.size };
    });
}
export async function capture(token: string, key: string, value: unknown) {
    await admit(token,'capture',capturePermissions); id(key);
    const b = object(value,['name','receipt_id','expected_receipt_version','manager_id','external_reference','source_url']);
    // Go struct field order/casing is part of persisted operation fingerprints.
    const input = { Name:name(b.name), ReceiptID:id(b.receipt_id), ManagerID:id(b.manager_id), ReceiptVersion:integer(b.expected_receipt_version,1), ExternalReference:optional(b.external_reference), SourceURL:optional(b.source_url,true) };
    const fp = digest(goJSON(input));
    return transaction(async db => {
        const actor = await access(db,token,capturePermissions), old = await operation(db,actor.id,'capture','named',key,fp);
        if (old) return { value:await projection(db,actor,await read(db,actor,old.tender_id)), replay:true };
        if (!canManage(await directory(db,input.ManagerID))) invalid('Select a ready, designated Manager with the required capabilities');
        const tender = randomUUID(), assignment = randomUUID();
        const r = (await db.query<RecordRow>(`INSERT INTO tender.tender_fms_intakes(id,title,owner_id,record_kind,sales_creator_id,manager_assignment_id,created_by_id,updated_by_id)
          VALUES($1,$2,$3,'named',$4,$5,$4,$4) RETURNING *,created_at::text AS updated_at_exact`,[tender,input.Name,input.ManagerID,actor.id,assignment])).rows[0];
        await db.query("INSERT INTO tender.tender_fms_manager_assignments(id,intake_id,manager_id,version,reason,created_by_id)VALUES($1,$2,$3,1,'Initial named Tender Manager assignment',$4)",[assignment,tender,input.ManagerID,actor.id]);
        await recordEvent(db,r,actor.id,'named_record_created');
        await consumeSources(db,actor.id,input.ReceiptID,input.ReceiptVersion,tender);
        await db.query('UPDATE tender.tender_fms_intakes SET receipt_id=$2,external_reference=$3,source_url=$4 WHERE id=$1',[tender,input.ReceiptID,input.ExternalReference,input.SourceURL]);
        await db.query('INSERT INTO tender.tender_provision_operations(id,tender_id,created_by_id,updated_by_id)VALUES($1,$2,$3,$3)',[randomUUID(),tender,actor.id]);
        await managerWork(db,r,actor.id);
        await save(db,actor.id,'capture','named',key,fp,tender);
        return { value:await projection(db,actor,r),replay:false };
    });
}
async function editScope(db: DB, actor: Actor, r: RecordRow, assignment: boolean) {
    if (r.sales_creator_id !== actor.id && r.owner_id !== actor.id && !actor.allows('tenders.portfolio.write')) forbidden();
    if (assignment && r.owner_id === actor.id && !actor.allows('tenders.portfolio.write') && !canManage(await directory(db,actor.id))) forbidden();
}
export async function patch(token: string, target: string, key: string, value: unknown) {
    await admit(token,'assignment'); id(target); id(key);
    const b = object(value,['name','expected_version','external_reference','source_url']);
    const input = { SetReference:Object.hasOwn(b,'external_reference'), SetURL:Object.hasOwn(b,'source_url'), Version:version(b.expected_version), Name:name(b.name), ExternalReference:optional(b.external_reference), SourceURL:optional(b.source_url,true) }, fp = digest(goJSON(input));
    return transaction(async db => {
        const actor = await access(db,token,['tenders.register.view','tenders.register.read','tenders.register.write']);
        const old = await operation(db,actor.id,'patch',target,key,fp), r = await read(db,actor,target);
        await editScope(db,actor,r,false);
        if (old) return { value:await projection(db,actor,r),replay:true };
        if (!r.is_active || Number(r.business_version) !== input.Version) throw new AppError(409,'Tender changed; refresh before editing');
        const saved = (await db.query<RecordRow>(`UPDATE tender.tender_fms_intakes SET title=$2,external_reference=CASE WHEN $3 THEN $4 ELSE external_reference END,source_url=CASE WHEN $5 THEN $6 ELSE source_url END,
          business_version=business_version+1,version=version+1,updated_at=clock_timestamp(),updated_by_id=$7 WHERE id=$1 RETURNING *,updated_at::text AS updated_at_exact`,[target,input.Name,input.SetReference,input.ExternalReference,input.SetURL,input.SourceURL,actor.id])).rows[0];
        await recordEvent(db,saved,actor.id,'metadata_updated');
        await save(db,actor.id,'patch',target,key,fp,target);
        return { value:await projection(db,actor,saved),replay:false };
    });
}
export async function changeManager(token: string, target: string, key: string, value: unknown) {
    await admit(token,'assignment'); id(target); id(key);
    const b = object(value,['expected_version','expected_assignment_version','manager_id','reason']);
    const input = { Version:version(b.expected_version), AssignmentVersion:version(b.expected_assignment_version), ManagerID:id(b.manager_id), Reason:reason(b.reason) }, fp = digest(goJSON(input));
    return transaction(async db => {
        const actor = await access(db,token,['tenders.register.view','tenders.register.read','tenders.register.assign']);
        const old = await operation(db,actor.id,'manager',target,key,fp), r = await read(db,actor,target);
        await editScope(db,actor,r,true);
        if (old) return { value:await projection(db,actor,r),replay:true };
        if (!r.is_active || Number(r.business_version) !== input.Version || Number(r.assignment_version) !== input.AssignmentVersion || r.owner_id === input.ManagerID) throw new AppError(409,'Refresh the active record and select a different Manager');
        if (!canManage(await directory(db,input.ManagerID))) invalid('Select a ready, designated Manager with the required capabilities');
        const assignment = randomUUID();
        await db.query('INSERT INTO tender.tender_fms_manager_assignments(id,intake_id,manager_id,version,reason,created_by_id)VALUES($1,$2,$3,$4,$5,$6)',[assignment,target,input.ManagerID,input.AssignmentVersion+1,input.Reason,actor.id]);
        const saved = (await db.query<RecordRow>(`UPDATE tender.tender_fms_intakes SET owner_id=$2,manager_assignment_id=$3,assignment_version=assignment_version+1,business_version=business_version+1,version=version+1,updated_at=clock_timestamp(),updated_by_id=$4 WHERE id=$1 RETURNING *,updated_at::text AS updated_at_exact`,[target,input.ManagerID,assignment,actor.id])).rows[0];
        await db.query("UPDATE tender.tender_email_jobs SET state='cancelled_obsolete',version=version+1,claim_id=NULL,lease_until=NULL,updated_at=clock_timestamp(),updated_by_id=$2 WHERE action_id IN(SELECT id FROM tender.tender_source_actions WHERE tender_id=$1) AND state IN('queued','attempting')",[target,actor.id]);
        await db.query("UPDATE tender.tender_source_actions SET state='cancelled_obsolete',is_active=false,updated_at=clock_timestamp(),updated_by_id=$2 WHERE tender_id=$1 AND state='pending'",[target,actor.id]);
        await recordEvent(db,saved,actor.id,'manager_assigned');
        await managerWork(db,saved,actor.id);
        await save(db,actor.id,'manager',target,key,fp,target);
        return { value:await projection(db,actor,saved),replay:false };
    });
}
