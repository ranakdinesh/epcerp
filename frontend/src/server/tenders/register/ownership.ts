import 'server-only';
import { randomUUID } from 'node:crypto';
import { transaction, type DB } from '../../db';
import { AppError } from '../../errors';
import { access, candidates, directory, ownerPermissions, forbidden, type Actor } from './access';
import { id, object, version, reason, invalid } from './validation';
import { journal } from './events';
async function legacy(db: DB, actor: Actor, target: string) {
    const r = (await db.query("SELECT * FROM tender.tender_fms_intakes WHERE id=$1 AND record_kind='legacy' FOR UPDATE",[target])).rows[0];
    if (!r || (r.owner_id !== actor.id && !actor.allows('tenders.portfolio.read'))) throw new AppError(404,'Tender intake not found');
    if (r.owner_id !== actor.id && !actor.allows('tenders.portfolio.write')) forbidden();
    return r;
}
export async function options(token: string, target: string) {
    id(target);
    return transaction(async db => {
        const actor = await access(db,token,['tenders.register.assign']), r = await legacy(db,actor,target);
        return { owner_id:r.owner_id,version:r.version,candidates:(await candidates(db,true)).filter(c => c.id !== r.owner_id).map(c => ({ id:c.id,name:c.display_name })) };
    });
}
export async function handover(token: string, target: string, value: unknown) {
    id(target); const b = object(value,['owner_id','version','reason']), owner = id(typeof b.owner_id === 'string' ? b.owner_id.toLowerCase() : b.owner_id), expected = version(b.version), why = reason(b.reason);
    if (expected >= 2147483647) invalid('Current intake version is required');
    return transaction(async db => {
        const actor = await access(db,token,['tenders.register.assign']), r = await legacy(db,actor,target);
        if (!r.is_active || r.version !== expected || r.owner_id === owner) invalid('Refresh the active intake and choose a different owner');
        const user = await directory(db,owner);
        if (!user?.active || !ownerPermissions.every(p => user.allows(p))) invalid('Choose an active owner with all tender reading and processing permissions');
        const saved = (await db.query('UPDATE tender.tender_fms_intakes SET owner_id=$2,version=version+1,updated_at=clock_timestamp(),updated_by_id=$3 WHERE id=$1 RETURNING version,updated_at::text AS at',[target,owner,actor.id])).rows[0], event = randomUUID();
        await db.query('INSERT INTO tender.tender_fms_owner_changes(id,intake_id,previous_owner_id,owner_id,version,reason,created_by_id,created_at)VALUES($1,$2,$3,$4,$5,$6,$7,$8)',[event,target,r.owner_id,owner,saved.version,why,actor.id,saved.at]);
        await journal(db,event,'tender_fms','tender_intake',target,saved.version,'owner_handover',actor.id,saved.at);
        return { owner_id:owner,version:saved.version };
    });
}
