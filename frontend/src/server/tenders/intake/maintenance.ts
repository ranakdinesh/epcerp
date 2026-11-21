import 'server-only';
import { randomUUID } from 'node:crypto';
import { transaction } from '../../db';
import { removeObject } from './objects';
import sql from './maintenance-sql.json';
// Retention policy and lease fencing match the original package worker.
export async function maintainOnce() {
    const claim = randomUUID(), objects = await transaction(async (db) => {
        const rows = (await db.query(sql.claim, [claim])).rows;
        for (const o of rows) {
            const expired = (await db.query("UPDATE document_package_receipts SET state='expired',error_code='receipt_expired',version=version+1,updated_at=clock_timestamp(),updated_by_id=owner_id WHERE id=$1 AND state NOT IN('consumed','expired')AND expires_at<clock_timestamp()RETURNING version,owner_id", [o.receipt_id])).rows[0];
            if (expired)
                await db.query("INSERT INTO document_package_events(id,receipt_id,action,version,error_code,created_by_id)VALUES($1,$2,'expired',$3,'receipt_expired',$4)", [randomUUID(), o.receipt_id, expired.version, expired.owner_id]);
        }
        return rows;
    });
    for (const o of objects) {
        await removeObject(o.id);
        await transaction(async (db) => {
            const updated = await db.query("UPDATE document_package_objects SET purged_at=clock_timestamp(),purge_lease_until=NULL WHERE id=$1 AND purge_claim_id=$2 AND purge_lease_until>clock_timestamp()AND purged_at IS NULL", [o.id, claim]);
            if (updated.rowCount !== 1)
                throw Error('Object purge lease expired');
            await db.query("INSERT INTO document_package_events(id,receipt_id,file_id,action,version,created_by_id)SELECT $1,id,$3,'unconsumed_object_purged',version,owner_id FROM document_package_receipts WHERE id=$2", [randomUUID(), o.receipt_id, o.file_id]);
        });
    }
    return objects.length;
}
const state = globalThis as typeof globalThis & {
    packageMaintenanceStarted?: boolean;
};
export function startMaintenance() {
    if (state.packageMaintenanceStarted || process.env.EPCERP_PLATFORM_OWNER !== 'nextjs')
        return;
    state.packageMaintenanceStarted = true;
    let running = false;
    const timer = setInterval(async () => {
        if (running)
            return;
        running = true;
        try {
            await maintainOnce();
        }
        catch {
            console.error('Package retention maintenance unavailable');
        }
        finally {
            running = false;
        }
    }, 10 * 60 * 1000);
    timer.unref();
}
