import 'server-only';
import { randomUUID } from 'node:crypto';
import { transaction, lock } from '../../db';
import { userCapabilities } from '../../auth/access';
// Verified source publication is independent of OCR and never queues a local reading.
export async function publishOnce() {
    const candidates = await transaction(async (db) => (await db.query("SELECT id,draft_id,owner_id FROM document_package_receipts WHERE draft_id IS NOT NULL AND state='ready' AND expires_at>clock_timestamp() ORDER BY created_at,id LIMIT 100")).rows);
    for (const candidate of candidates) {
        const published = await transaction(async (db) => {
            await lock(db, 'auth.administration');
            const user = await userCapabilities(db, candidate.owner_id);
            if (!user?.active || !user.ready || ['tenders.register.view', 'tenders.register.read', 'tenders.register.write', 'tenders.processing.read', 'tenders.processing.write'].some(p => !user.allows(p)))
                return false;
            const draft = (await db.query('SELECT id FROM tender.document_drafts WHERE id=$1 AND owner_id=$2 AND is_active FOR UPDATE', [candidate.draft_id, candidate.owner_id])).rows[0];
            if (!draft)
                return false;
            const receipt = (await db.query("SELECT * FROM document_package_receipts WHERE id=$1 AND draft_id=$2 AND state='ready' AND expires_at>clock_timestamp() FOR UPDATE", [candidate.id, draft.id])).rows[0];
            if (!receipt)
                return false;
            const members = Number((await db.query('SELECT count(*) FROM document_package_members WHERE receipt_id=$1 AND validation_claim_id=$2', [receipt.id, receipt.claim_id])).rows[0].count);
            if (members < 1 || members > 100)
                return false;
            const saved = (await db.query("UPDATE document_package_receipts SET state='consumed',published_at=clock_timestamp(),version=version+1,updated_at=clock_timestamp(),updated_by_id=owner_id WHERE id=$1 RETURNING version", [receipt.id])).rows[0];
            await db.query("INSERT INTO document_package_events(id,receipt_id,action,version,created_by_id)VALUES($1,$2,'draft_published',$3,$4)", [randomUUID(), receipt.id, saved.version, user.id]);
            await db.query("INSERT INTO tender.draft_events(id,draft_id,action,receipt_id,created_by_id)VALUES($1,$2,'source_documents_ready',$3,$4)", [randomUUID(), draft.id, receipt.id, user.id]);
            return true;
        });
        if (published)
            return true;
    }
    return false;
}
const globalPublisher = globalThis as typeof globalThis & {
    tenderPublisherStarted?: boolean;
};
export function startPublisher() { if (globalPublisher.tenderPublisherStarted || process.env.EPCERP_TENDER_INTAKE_OWNER !== 'nextjs' || process.env.EPCERP_TSC_OWNER !== 'nextjs')
    return; globalPublisher.tenderPublisherStarted = true; let running = false; const timer = setInterval(async () => { if (running)
    return; running = true; try {
    await publishOnce();
}
catch {
    console.error('Tender source publication unavailable');
}
finally {
    running = false;
} }, 3000); timer.unref(); }
