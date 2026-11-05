import { randomUUID } from 'node:crypto';
import type { DB } from '../../db';
import { quotationNotice } from '../../inbox/purchasing-notices';
import type { Attempt } from './types';
import type { Event } from './repository';
export async function notify(db:DB,e:Event,a:Attempt,recipients:string[],label:string){
    const submitted=e.action==='submit',reviewLink=`/dashboard?screen=quotation-approvals&project=${e.project_id}&bo=${e.bo_id}&group=${a.group_id}&attempt=${a.id}`,executiveLink=`/dashboard?screen=purchase-fms&project=${e.project_id}&step=PUR-04`,link=submitted?reviewLink:executiveLink,title=[...(submitted?`Quotation shortlist submitted: ${label} · ${a.group_name}`:`Quotation ${a.state}: ${label} · ${a.group_name} · ${a.return_action??''}`)].slice(0,300).join('');
    for(const recipient of recipients)await quotationNotice(db,e.id,recipient,title,link);
    if(submitted)await db.query("UPDATE purchase_quotation_work_tasks SET state='completed',updated_at=clock_timestamp(),updated_by_id=$3 WHERE bo_id=$1 AND group_id=$2 AND state<>'completed'",[e.bo_id,a.group_id,e.created_by_id]);else await db.query("UPDATE purchase_quotation_work_tasks SET state='completed',updated_at=clock_timestamp(),updated_by_id=$2 WHERE attempt_id=$1 AND state<>'completed'",[a.id,e.created_by_id]);
    const create=async(recipient:string,kind:string,state:string,title:string,link:string)=>{await db.query('INSERT INTO purchase_quotation_work_tasks(id,project_id,bo_id,group_id,attempt_id,recipient_id,kind,state,title,link,created_by_id,updated_by_id)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$11)',[randomUUID(),e.project_id,e.bo_id,a.group_id,a.id,recipient,kind,state,title,link,e.created_by_id]);};
    if(submitted){for(const recipient of recipients)await create(recipient,'finance','pending','Review quotation shortlist · '+a.group_name,reviewLink);await create(a.submitted_by_id,'executive','waiting','Waiting for Finance · '+a.group_name,executiveLink);}
    else if(a.state==='rejected'){const change=a.return_action==='change_vendor',returned=`/dashboard?screen=purchase-fms&project=${e.project_id}&step=${change?'PUR-02':'PUR-03'}&bo=${e.bo_id}&group=${a.group_id}${change?'&indentTab=vendors':'&quote='+a.quote_key}`;await db.query("UPDATE purchase_quotation_work_tasks SET state=$2,title=$3,link=$4,updated_at=clock_timestamp(),updated_by_id=$5 WHERE attempt_id=$1 AND kind='executive'",[a.id,a.return_action,'Returned quotation · '+a.group_name,returned,e.created_by_id]);}
}
