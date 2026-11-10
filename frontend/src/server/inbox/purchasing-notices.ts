import 'server-only';
import { randomUUID } from 'node:crypto';
import type { DB } from '../db';
import { timeSQL,timestamp } from './repository';
import { assignmentIdentity } from '../auth/access';
import { AppError } from '../errors';
type Source={id:string;entity_id:string;entity_version:string|number;source_module:string;entity_type:string;action:string;actor_id:string;scope_kind:string;scope_id:string;visibility:string;at:string};
async function source(db:DB,event:string){const row=(await db.query(`SELECT *,${timeSQL('occurred_at')} AS at FROM business_events WHERE id=$1 FOR SHARE`,[event])).rows[0] as Source|undefined;if(!row||row.scope_kind!=='project'||!row.scope_id||row.visibility!=='restricted'||Number(row.entity_version)<1)throw new AppError(409,'The financial decision event does not match a valid notice source');return row;}
async function deliver(db:DB,s:Source,recipient:string,kind:'director'|'quotation',director:{id:string;version:number}|null,title='',link=''){
    const id=randomUUID(),at=timestamp(s.at)!;
    const inserted=(await db.query(`INSERT INTO communication_notices(id,source_event_id,recipient_id,director_designation_id,director_version,decision_id,project_id,source_version,decision_action,created_at,updated_at,created_by_id,updated_by_id,notice_kind,title,link)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$10,$11,$11,$12,$13,$14)ON CONFLICT(source_event_id,recipient_id)DO NOTHING RETURNING id`,[id,s.id,recipient,director?.id??null,director?.version??0,s.entity_id,s.scope_id,s.entity_version,s.action,at,s.actor_id,kind,title,link])).rows[0];
    if(!inserted)return;
    const snapshot={notice_kind:kind,...(title?{title}:{}),...(link?{link}:{}),id,source_event_id:s.id,recipient_id:recipient,director_designation_id:director?.id??'',director_version:director?.version??0,decision_id:s.entity_id,project_id:s.scope_id,source_version:Number(s.entity_version),decision_action:s.action,version:1,is_active:true,read_at:null,acknowledged_at:null,created_at:at,updated_at:at,created_by_id:s.actor_id,updated_by_id:s.actor_id},event=randomUUID();
    await db.query("INSERT INTO communication_notice_events(id,notice_id,action,notice_version,before_snapshot,after_snapshot,created_at,created_by_id)VALUES($1,$2,'delivered',1,'null'::jsonb,$3,$4,$5)",[event,id,snapshot,at,s.actor_id]);
    await db.query(`INSERT INTO business_events(id,source_module,entity_type,entity_id,entity_version,delivery_count,action,actor_id,actor_kind,correlation_id,causation_id,scope_kind,scope_id,visibility,occurred_at)VALUES($1,'communication_inbox','director_notice',$2,1,0,'delivered',$3,'user',$4,$4,'project',$5,'restricted',$6)`,[event,id,s.actor_id,s.id,s.scope_id,at]);
}
export async function quotationNotice(db:DB,event:string,recipient:string,title:string,link:string){const s=await source(db,event);if(s.source_module!=='quotation_approvals'||s.entity_type!=='quotation_approval_set'||!['submit','accounts_decision'].includes(s.action)||[...title].length>300||link.length>2000)throw new AppError(409,'The financial decision event does not match a valid notice source');await deliver(db,s,recipient,'quotation',null,title,link);}
export async function directorDecisionNotice(db:DB,event:string){
    const s=await source(db,event);if(!['finance_approvals','finance_accounts_acceptance'].includes(s.source_module)||s.entity_type!=='financial_decision'||!['approved','rejected','returned','cancelled'].includes(s.action))throw new AppError(409,'The financial decision event does not match a valid notice source');
    const existing=(await db.query('SELECT decision_id,project_id,source_version,decision_action FROM communication_notices WHERE source_event_id=$1 LIMIT 1',[event])).rows[0];
    if(existing){if(existing.decision_id!==s.entity_id||existing.project_id!==s.scope_id||Number(existing.source_version)!==Number(s.entity_version)||existing.decision_action!==s.action)throw new AppError(409,'The financial decision event does not match a valid notice source');return;}
    await db.query("SELECT pg_advisory_xact_lock(hashtext('epcerp.organisation.director'))");
    const d=(await db.query('SELECT id,user_id,version FROM organisation_director_designation WHERE slot=true AND is_active')).rows[0];if(!d||!(await assignmentIdentity(db,d.user_id))?.active)throw new AppError(409,'An effective Director must be designated before the financial decision');
    await deliver(db,s,d.user_id,'director',{id:d.id,version:d.version});
}

export async function procurementManagerProjectNotice(db: DB, eventId: string, projectId: string, projectNumber: string, projectName: string) {
    const s = await source(db, eventId);
    if (s.source_module !== 'purchase_fms' || s.entity_type !== 'purchase_workspace') {
        throw new AppError(409, 'Event does not match purchase workspace creation');
    }
    const managers = (await db.query(`
        SELECT DISTINCT u.id
        FROM auth.users u
        JOIN auth.user_roles ur ON ur.user_id = u.id
        JOIN auth.roles r ON r.id = ur.role_id AND r.is_active
        WHERE u.is_active
          AND EXISTS (
            SELECT 1 FROM auth.role_permissions rp
            JOIN auth.permissions p ON p.id = rp.permission_id
            WHERE rp.role_id = r.id AND p.code = 'projects.purchase_assignments.write'
          )
    `)).rows as { id: string }[];

    const title = `Purchase FMS enabled: ${projectNumber}${projectName ? ' · ' + projectName : ''}`;
    const link = `/dashboard?screen=purchase-fms&project=${encodeURIComponent(projectId)}`;

    for (const m of managers) {
        const id = randomUUID(), at = timestamp(s.at)!;
        const inserted = (await db.query(`
            INSERT INTO communication_notices(
                id, source_event_id, recipient_id, director_designation_id, director_version,
                decision_id, project_id, source_version, decision_action,
                created_at, updated_at, created_by_id, updated_by_id,
                notice_kind, title, link
            )
            VALUES($1, $2, $3, NULL, 0, $4, $4, $5, 'assigned', $6, $6, $7, $7, 'procurement_manager', $8, $9)
            ON CONFLICT(source_event_id, recipient_id) DO NOTHING
            RETURNING id
        `, [id, s.id, m.id, projectId, Number(s.entity_version), at, s.actor_id, title, link])).rows[0];

        if (inserted) {
            const snapshot = {
                notice_kind: 'procurement_manager',
                title,
                link,
                id,
                source_event_id: s.id,
                recipient_id: m.id,
                director_designation_id: '',
                director_version: 0,
                decision_id: projectId,
                project_id: projectId,
                source_version: Number(s.entity_version),
                decision_action: 'assigned',
                version: 1,
                is_active: true,
                read_at: null,
                acknowledged_at: null,
                created_at: at,
                updated_at: at,
                created_by_id: s.actor_id,
                updated_by_id: s.actor_id
            };
            const event = randomUUID();
            await db.query("INSERT INTO communication_notice_events(id,notice_id,action,notice_version,before_snapshot,after_snapshot,created_at,created_by_id)VALUES($1,$2,'delivered',1,'null'::jsonb,$3,$4,$5)", [event, id, snapshot, at, s.actor_id]);
            await db.query(`INSERT INTO business_events(id,source_module,entity_type,entity_id,entity_version,delivery_count,action,actor_id,actor_kind,correlation_id,causation_id,scope_kind,scope_id,visibility,occurred_at)VALUES($1,'communication_inbox','director_notice',$2,1,0,'delivered',$3,'user',$4,$4,'project',$5,'restricted',$6)`, [event, id, s.actor_id, s.id, projectId, at]);
        }
    }
}
