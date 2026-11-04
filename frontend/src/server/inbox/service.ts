import 'server-only';
import { transaction } from '../db';
import { requireAnyPermission } from '../auth/access';
import { query, noticeID, receiptVersion, InboxError, invalid } from './validation';
import { ownNotice, history, notice, exactColumns, timeSQL, timestamp, receiptEvent } from './repository';
async function access(db: Parameters<typeof requireAnyPermission>[0], token: string, write = false) {
    const actor = await requireAnyPermission(db,token,['communication.inbox.read'],true);
    if (write && !actor.allows('communication.inbox.write')) throw new InboxError(403,'Personal inbox permission denied');
    return actor;
}
export const authorize = (token: string) => transaction(db => access(db,token));
export async function list(token: string, params: URLSearchParams) {
    const q = query(params);
    return transaction(async db => {
        const actor = await access(db,token), values = [actor.id,q.status];
        const filter = `recipient_id=$1 AND ($2::text='' OR ($2='unread' AND read_at IS NULL) OR ($2='read' AND read_at IS NOT NULL AND acknowledged_at IS NULL) OR ($2='acknowledged' AND acknowledged_at IS NOT NULL))`;
        const total = Number((await db.query(`SELECT count(*) FROM communication_notices WHERE ${filter}`,values)).rows[0].count);
        const items = (await db.query(`SELECT n.*,${exactColumns} FROM communication_notices n WHERE ${filter} ORDER BY n.created_at DESC,n.id DESC LIMIT $3 OFFSET $4`,[...values,q.size,(q.page-1)*q.size])).rows.map(notice);
        return {items,total,page:q.page,size:q.size,can_write:actor.allows('communication.inbox.write')};
    });
}
export async function read(token: string, target: string) {
    const id = noticeID(target);
    return transaction(async db => { const actor = await access(db,token); return {notice:await ownNotice(db,id,actor.id),history:await history(db,id,actor.id),can_write:actor.allows('communication.inbox.write')}; });
}
export async function receipt(token: string, target: string, value: unknown, action: 'read' | 'acknowledge') {
    const id = noticeID(target), version = receiptVersion(value);
    return transaction(async db => {
        const actor = await access(db,token,true), before = await ownNotice(db,id,actor.id);
        if (version !== before.version) throw new InboxError(409,'Notice changed; reload before updating the receipt');
        // Acknowledgements are retained receipts, never source-work approval/completion.
        // Superseded assignment notices remain available only to their original recipient.
        const now = (await db.query(`WITH t AS (SELECT clock_timestamp() AS at) SELECT ${timeSQL('t.at')} AS at,t.at>=$1::timestamptz AS valid FROM t`,[before.updated_at])).rows[0];
        if (!now.valid) invalid('time','Receipt time unavailable');
        const unchanged = action === 'read' ? !!before.read_at : !!before.acknowledged_at;
        let after = before;
        if (!unchanged) {
            if (before.version === 2147483647) throw new InboxError(409,'Notice version limit reached');
            const at = timestamp(now.at)!;
            const saved = (await db.query(`UPDATE communication_notices SET version=version+1,read_at=$4,acknowledged_at=$5,updated_at=$6,updated_by_id=$2 WHERE id=$1 AND recipient_id=$2 AND version=$3 RETURNING *,${exactColumns}`,[id,actor.id,version,before.read_at ?? at,action === 'acknowledge' ? at : before.acknowledged_at,at])).rows[0];
            if (!saved) throw new InboxError(409,'Notice changed; reload before updating the receipt');
            after = notice(saved);
            await receiptEvent(db,before,after,action,actor.id);
        }
        return {notice:after,history:await history(db,id,actor.id),can_write:true};
    });
}
