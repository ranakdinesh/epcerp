import type { DB } from '../../db';

export async function tenderAwardPending(db: DB, project: string): Promise<boolean> {
    const { rows } = await db.query<{ pending: boolean }>(`
        SELECT EXISTS (
            SELECT 1 FROM tender.bidding_authorizations a
            WHERE a.project_id = $1
              AND NOT EXISTS (
                SELECT 1 FROM tender.bid_costings c
                WHERE c.draft_id = a.draft_id AND c.project_id = $1 AND c.status = 'awarded'
              )
        ) AS pending
    `, [project]);
    return rows[0].pending;
}

export const tenderAwardMessage = 'Record the company award in Tender FMS before committing to a vendor.';
