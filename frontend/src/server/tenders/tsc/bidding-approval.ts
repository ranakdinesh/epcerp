import 'server-only';
import { pool, transaction } from '../../db';
import { AppError } from '../../errors';
import { requireClosedPrebidReview, tenderFlow, tenderFlowColumns } from '../flow';
import { authorizeTender } from '../documents';
import { biddingProjectId, ensureBiddingProject } from './bidding-project';

export type BiddingAuthorization = {
  id: string;
  draft_id: string;
  decision: 'authorized' | 'rejected' | 'conditional';
  manager_id: string | null;
  manager_name?: string;
  estimated_value: number | null;
  emd_amount: number | null;
  emd_exemption_claimed: boolean;
  tender_fee: number | null;
  tender_fee_exemption_claimed: boolean;
  due_date: string | null;
  approval_notes: string;
  authorized_by_id: string;
  authorized_by_name?: string;
  authorized_at: string;
  version: number;
};

export async function getTenderFlowStatus(draftId: string) {
  const { rows: [draft] } = await pool().query(
    `SELECT d.*, ${tenderFlowColumns} FROM tender.document_drafts d WHERE d.id=$1`,
    [draftId]
  );
  if (!draft) throw new AppError(404, 'Tender draft not found');
  return { draft_id: draftId, ...tenderFlow(draft) };
}

export async function getBiddingApproval(draftId: string): Promise<BiddingAuthorization | null> {
  const db = pool();
  const res = await db.query<BiddingAuthorization>(
    `SELECT a.id, a.draft_id, a.decision, a.manager_id,
            m.username as manager_name,
            a.estimated_value, a.emd_amount, a.emd_exemption_claimed,
            a.tender_fee, a.tender_fee_exemption_claimed,
            to_char(a.due_date, 'YYYY-MM-DD') as due_date,
            a.approval_notes, a.authorized_by_id,
            u.username as authorized_by_name,
            to_char(a.authorized_at, 'YYYY-MM-DD"T"HH24:MI:SSZ') as authorized_at,
            a.version
     FROM tender.bidding_authorizations a
     LEFT JOIN auth.users m ON m.id = a.manager_id
     LEFT JOIN auth.users u ON u.id = a.authorized_by_id
     WHERE a.draft_id = $1`,
    [draftId]
  );
  return res.rows[0] ?? null;
}

export async function saveBiddingApproval(
  draftId: string,
  payload: {
    decision: 'authorized' | 'rejected' | 'conditional';
    manager_id?: string | null;
    estimated_value?: number | null;
    emd_amount?: number | null;
    emd_exemption_claimed?: boolean;
    tender_fee?: number | null;
    tender_fee_exemption_claimed?: boolean;
    due_date?: string | null;
    approval_notes?: string;
    project?: Record<string, unknown>;
  },
  token: string
) {
  if (!payload || !['authorized', 'rejected', 'conditional'].includes(payload.decision)) {
    throw new AppError(422, 'Select a valid bidding decision');
  }
  if (payload.project != null && (typeof payload.project !== 'object' || Array.isArray(payload.project))) {
    throw new AppError(422, 'Provide valid project details');
  }
  return transaction(async (db) => {
    const scope = await authorizeTender(db, token, draftId, 'write');
    await requireClosedPrebidReview(db, draftId);
    const { rows: [verdict] } = await db.query(
      `SELECT human_verified AS current
       FROM tender.tsc_eligibility_verdicts WHERE draft_id=$1 ORDER BY evaluated_at DESC, id DESC LIMIT 1`,
      [draftId],
    );
    if (!verdict?.current) throw new AppError(409, 'Finalize the TSC before recording the bidding decision.');
    const actorId = scope.actor.id;
    const project = payload.decision === 'authorized'
      ? await ensureBiddingProject(db, scope, await biddingProjectId(db, draftId), payload.manager_id || null, {
        estimated_value: payload.estimated_value ?? null, due_date: payload.due_date || null, approval_notes: payload.approval_notes || '',
      }, payload.project ?? {})
      : null;
    const res = await db.query<BiddingAuthorization>(
      `INSERT INTO tender.bidding_authorizations (
        draft_id, decision, manager_id, estimated_value, emd_amount,
        emd_exemption_claimed, tender_fee, tender_fee_exemption_claimed,
        due_date, approval_notes, authorized_by_id, authorized_at, version, project_id
      ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, clock_timestamp(), 1, $12)
      ON CONFLICT (draft_id) DO UPDATE SET
        decision = EXCLUDED.decision,
        manager_id = EXCLUDED.manager_id,
        estimated_value = EXCLUDED.estimated_value,
        emd_amount = EXCLUDED.emd_amount,
        emd_exemption_claimed = EXCLUDED.emd_exemption_claimed,
        tender_fee = EXCLUDED.tender_fee,
        tender_fee_exemption_claimed = EXCLUDED.tender_fee_exemption_claimed,
        due_date = EXCLUDED.due_date,
        approval_notes = EXCLUDED.approval_notes,
        authorized_by_id = EXCLUDED.authorized_by_id,
        authorized_at = clock_timestamp(),
        project_id = COALESCE(tender.bidding_authorizations.project_id, EXCLUDED.project_id),
        version = tender.bidding_authorizations.version + 1
      RETURNING id, draft_id, decision, manager_id, estimated_value, emd_amount,
                emd_exemption_claimed, tender_fee, tender_fee_exemption_claimed,
                to_char(due_date, 'YYYY-MM-DD') as due_date,
                approval_notes, authorized_by_id,
                to_char(authorized_at, 'YYYY-MM-DD"T"HH24:MI:SSZ') as authorized_at,
                version`,
      [
        draftId,
        payload.decision,
        payload.manager_id || null,
        payload.estimated_value || null,
        payload.emd_amount || null,
        Boolean(payload.emd_exemption_claimed),
        payload.tender_fee || null,
        Boolean(payload.tender_fee_exemption_claimed),
        payload.due_date || null,
        payload.approval_notes || '',
        actorId,
        project?.id ?? null,
      ]
    );

    // Fetch user names
    const row = res.rows[0];
    const userNames = await db.query<{ mname?: string; aname?: string }>(
      `SELECT m.username as mname, u.username as aname
       FROM (SELECT $1::uuid as mid, $2::uuid as aid) q
       LEFT JOIN auth.users m ON m.id = q.mid
       LEFT JOIN auth.users u ON u.id = q.aid`,
      [row.manager_id, row.authorized_by_id]
    );

    return {
      ...row,
      manager_name: userNames.rows[0]?.mname,
      authorized_by_name: userNames.rows[0]?.aname,
      project,
    };
  });
}
