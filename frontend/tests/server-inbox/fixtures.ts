import { randomUUID } from 'node:crypto';
import { pool, transaction } from '../../src/server/db';
import { supplierReviewNotice } from '../../src/server/inbox/supplier-review';
import { tenderAssignmentNotice } from '../../src/server/inbox/tender-assignment';
export async function project(actor: string) {
    const client = randomUUID(), id = randomUUID(), code = 'QA'+randomUUID().replaceAll('-','').slice(0,12);
    await pool().query(`INSERT INTO clients(id,client_code,legal_name,normalized_legal_name,display_name,parent_organization,client_type,industry,gstin,pan,cin,website,address_line_1,address_line_2,city,state,postal_code,country,legacy_source,legacy_source_id,is_active)
      VALUES($1,$2,$2,$2,$2,'','','','','','','','','','','','','India','','',true)`,[client,code]);
    await pool().query('INSERT INTO projects(id,project_number,name,normalized_name,client_id,project_manager_id)VALUES($1,$2,$2,$2,$3,$4)',[id,code,client,actor]);
    return id;
}
export async function fixture(kind: 'director' | 'quotation' | 'supplier_review' | 'tender_manager', recipient: string, actor: string, project: string, at = '2026-01-01T01:02:03.123456Z') {
    return transaction(async db => {
        const id = randomUUID(), event = randomUUID(), entity = randomUUID(), action = randomUUID();
        let sourceModule: string, type: string, decision: string;
        if (kind === 'supplier_review') {
            const supplier = randomUUID(), code = 'QA'+supplier.slice(0,12);
            await db.query("INSERT INTO suppliers(id,code,name,data,is_active)VALUES($1,$2,$2,'{}',true)",[supplier,code]);
            await db.query('INSERT INTO supplier_review_assignments(id,supplier_id,user_id,version,is_active)VALUES($1,$2,$3,1,true)',[entity,supplier,recipient]);
            sourceModule='supplier_review_assignments';type='supplier_review_assignment';decision='assigned';
            await source(); await supplierReviewNotice(db,event,recipient,supplier);
            return (await db.query('SELECT id FROM communication_notices WHERE source_event_id=$1',[event])).rows[0].id as string;
        }
        if (kind === 'tender_manager') {
            const tender = randomUUID();
            await db.query("INSERT INTO tender.tender_fms_intakes(id,title,owner_id,record_kind,sales_creator_id,manager_assignment_id)VALUES($1,'Synthetic Inbox Tender',$2,'named',$3,$4)",[tender,recipient,actor,entity]);
            await db.query("INSERT INTO tender.tender_fms_manager_assignments(id,intake_id,manager_id,version,reason,created_by_id)VALUES($1,$2,$3,1,'Synthetic initial assignment',$4)",[entity,tender,recipient,actor]);
            sourceModule='tender_fms';type='tender_assignment';decision='manager_assigned';
            await source();
            await db.query('INSERT INTO tender.tender_source_actions(id,tender_id,assignment_id,recipient_id,source_event_id,assignment_version,created_at,updated_at,created_by_id,updated_by_id)VALUES($1,$2,$3,$4,$5,1,$6,$6,$7,$7)',[action,tender,entity,recipient,event,at,actor]);
            const notice = await tenderAssignmentNotice(db,action);
            await db.query('UPDATE tender.tender_source_actions SET notice_id=$2 WHERE id=$1',[action,notice]);
            return notice;
        }
        sourceModule=kind==='director'?'finance_approvals':'quotation_approvals';type=kind==='director'?'financial_decision':'quotation_approval_set';decision=kind==='director'?'approved':'submit';
        await source();
        let director: string | null = null;
        if (kind === 'director') {
            await db.query('INSERT INTO organisation_director_designation(id,user_id,version)VALUES($1,$2,1)ON CONFLICT(slot)DO NOTHING',[randomUUID(),recipient]);
            director=(await db.query('SELECT id FROM organisation_director_designation WHERE user_id=$1',[recipient])).rows[0].id;
        }
        await db.query(`INSERT INTO communication_notices(id,source_event_id,recipient_id,director_designation_id,director_version,decision_id,project_id,source_version,decision_action,created_at,updated_at,created_by_id,updated_by_id,notice_kind,title,link)
          VALUES($1,$2,$3,$4,$5,$6,$7,1,$8,$9,$9,$10,$10,$11,$12,$13)`,[id,event,recipient,director,kind==='director'?1:0,entity,project,decision,at,actor,kind,kind==='director'?'':'Synthetic quotation notice',kind==='director'?'':'/dashboard?screen=quotations']);
        return id;
        async function source() {
            const platform=['supplier_review','tender_manager'].includes(kind);
            await db.query(`INSERT INTO business_events(id,source_module,entity_type,entity_id,entity_version,delivery_count,action,actor_id,actor_kind,correlation_id,scope_kind,scope_id,visibility,occurred_at)
              VALUES($1,$2,$3,$4,1,0,$5,$6,'user',$1,$7,$8,'restricted',$9)`,[event,sourceModule,type,entity,decision,actor,platform?'platform':'project',platform?null:project,at]);
        }
    });
}
