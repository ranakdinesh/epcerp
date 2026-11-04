import 'server-only';
import type { DB } from '../../db';
import { AppError } from '../../errors';
import { directory, canManage, type Actor } from './access';
export type RecordRow = { id: string; title: string; owner_id: string; sales_creator_id: string; manager_assignment_id: string; business_version: string; content_revision: string; assignment_version: string; record_kind: string; is_active: boolean; version: number; updated_at_exact: string };
export async function read(db: DB, actor: Actor, id: string) {
    const r = (await db.query<RecordRow>('SELECT *,updated_at::text AS updated_at_exact FROM tender.tender_fms_intakes WHERE id=$1 FOR UPDATE',[id])).rows[0];
    if (!r || r.record_kind !== 'named') throw new AppError(404,'Tender record not found');
    if (r.owner_id !== actor.id && r.sales_creator_id !== actor.id && !actor.allows('tenders.portfolio.read')) {
        const grant = (await db.query('SELECT is_active AND (valid_until IS NULL OR valid_until>clock_timestamp()) AS active FROM tender.tender_fms_participant_grants WHERE intake_id=$1 AND user_id=$2 ORDER BY version DESC LIMIT 1',[id,actor.id])).rows[0];
        if (!grant?.active) throw new AppError(404,'Tender record not found');
    }
    return r;
}
export async function projection(db: DB, actor: Actor, r: RecordRow) {
    const p = (await db.query('SELECT * FROM tender.tender_provision_operations WHERE tender_id=$1',[r.id])).rows[0];
    if (!p) throw new AppError(404,'Tender capture not found');
    const manager = await directory(db,r.owner_id), identity = await directory(db,actor.id);
    const scope = r.owner_id === actor.id || r.sales_creator_id === actor.id || actor.allows('tenders.portfolio.write');
    return { id:r.id, name:r.title, business_version:Number(r.business_version), content_revision:Number(r.content_revision), assignment_version:Number(r.assignment_version), manager_id:r.owner_id,
        manager_display_name:manager?.name ?? '', can_edit:scope && actor.allows('tenders.register.write'), can_assign:scope && actor.allows('tenders.register.assign') && (r.owner_id !== actor.id || canManage(identity) || actor.allows('tenders.portfolio.write')),
        project_id:p.project_id ?? null, estimate_id:p.estimate_id ?? null, provisioning_version:Number(p.version), provisioning_state:p.state, blocker_code:p.blocker_code || null,
        deliveries:(await db.query(`SELECT a.id AS source_action_id,a.notice_id,j.id AS email_job_id,j.state AS email_state FROM tender.tender_source_actions a JOIN tender.tender_email_jobs j ON j.action_id=a.id WHERE a.tender_id=$1 AND a.state='pending' ORDER BY a.created_at DESC LIMIT 10`,[r.id])).rows };
}
export const visibleSQL = `i.record_kind='named' AND i.receipt_id IS NOT NULL AND ($1::boolean OR i.sales_creator_id=$2 OR i.owner_id=$2 OR COALESCE((SELECT g.is_active AND (g.valid_until IS NULL OR g.valid_until>clock_timestamp()) FROM tender.tender_fms_participant_grants g WHERE g.intake_id=i.id AND g.user_id=$2 ORDER BY version DESC LIMIT 1),false)) AND ($3::text='' OR position(lower($3) in lower(i.title))>0)`;
