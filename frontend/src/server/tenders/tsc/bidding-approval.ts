import 'server-only';
import { pool, transaction } from '../../db';
import { AppError } from '../../errors';

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

export type TenderFlowStatus = {
  draft_id: string;
  completed_steps: number[];
  next_step: string;
  status_label: string;
  has_documents: boolean;
  has_verified_batches: boolean;
  tsc_completed: boolean;
  tsc_decision?: 'pending' | 'accepted' | 'rejected' | 'overridden';
  bidding_authorized: boolean;
  bidding_decision?: string;
};

export async function getTenderFlowStatus(draftId: string): Promise<TenderFlowStatus> {
  const db = pool();

  // 1. Check batches in draft
  const draftRes = await db.query<{
    id: string;
    has_verified: boolean;
    has_sources: boolean;
  }>(
    `SELECT d.id,
            EXISTS(
              SELECT 1 FROM document_package_receipts r
              WHERE r.draft_id = d.id AND r.state IN ('consumed', 'ready')
            ) as has_verified,
            EXISTS(
              SELECT 1 FROM tender.draft_sources s
              WHERE s.draft_id = d.id
            ) as has_sources
     FROM tender.document_drafts d
     WHERE d.id = $1`,
    [draftId]
  );

  const draft = draftRes.rows[0];
  if (!draft) throw new AppError(404, 'Tender draft not found');

  const hasVerified = draft.has_verified || draft.has_sources;

  // 2. Check TSC Eligibility Verdict
  const verdictRes = await db.query<{
    human_verified: boolean;
    human_decision: 'pending' | 'accepted' | 'rejected' | 'overridden';
  }>(
    `SELECT human_verified, human_decision
     FROM tender.tsc_eligibility_verdicts
     WHERE draft_id = $1
     ORDER BY evaluated_at DESC
     LIMIT 1`,
    [draftId]
  );
  const verdict = verdictRes.rows[0];
  const tscCompleted = Boolean(verdict && verdict.human_verified);

  // 3. Check Bidding Authorization
  const authRes = await db.query<{
    decision: 'authorized' | 'rejected' | 'conditional';
  }>(
    `SELECT decision
     FROM tender.bidding_authorizations
     WHERE draft_id = $1`,
    [draftId]
  );
  const bidding = authRes.rows[0];
  const biddingAuthorized = Boolean(
    bidding && (bidding.decision === 'authorized' || bidding.decision === 'conditional')
  );

  // 4. Check downstream stages: prebid_queries, bid_items, quotations, costing
  const downstreamRes = await db.query<{
    has_queries: boolean;
    has_answered_queries: boolean;
    has_items: boolean;
    has_quotes: boolean;
    has_l1: boolean;
    costing_status: string | null;
  }>(
    `SELECT
       EXISTS(SELECT 1 FROM tender.prebid_queries WHERE draft_id = $1) as has_queries,
       EXISTS(SELECT 1 FROM tender.prebid_queries WHERE draft_id = $1 AND status IN ('answered', 'accepted', 'rejected')) as has_answered_queries,
       EXISTS(SELECT 1 FROM tender.bid_items WHERE draft_id = $1) as has_items,
       EXISTS(SELECT 1 FROM tender.vendor_quotations WHERE draft_id = $1) as has_quotes,
       EXISTS(SELECT 1 FROM tender.vendor_quotations WHERE draft_id = $1 AND is_l1 = true) as has_l1,
       (SELECT status FROM tender.bid_costings WHERE draft_id = $1) as costing_status`,
    [draftId]
  );
  const down = downstreamRes.rows[0];

  // Compute completed steps
  const completedSteps: number[] = [];
  if (hasVerified) completedSteps.push(1);
  if (tscCompleted) completedSteps.push(2);
  if (biddingAuthorized) completedSteps.push(3);

  if (down?.has_queries) completedSteps.push(6); // LG-03
  if (down?.has_answered_queries) completedSteps.push(7); // LG-04
  if (down?.has_items) {
    completedSteps.push(8); // CV-01
    completedSteps.push(9); // CV-02
  }
  if (down?.has_quotes) {
    completedSteps.push(10); // CV-03
    completedSteps.push(11); // CV-04
  }
  if (down?.has_l1) completedSteps.push(12); // CV-05
  if (down?.costing_status && ['approved', 'submitted', 'awarded'].includes(down.costing_status)) {
    completedSteps.push(14); // CV-07
  }
  if (down?.costing_status && ['submitted', 'awarded'].includes(down.costing_status)) {
    completedSteps.push(15); // CV-08
  }

  let nextStep = 'TS-01';
  let statusLabel = 'Upload & verify documents';

  if (down?.costing_status === 'awarded') {
    nextStep = 'AWARD';
    statusLabel = 'Tender Awarded · Project & Purchase Handoff Ready';
  } else if (down?.costing_status === 'submitted') {
    nextStep = 'CV-08';
    statusLabel = 'Bid Submitted · Awaiting Commercial Award';
  } else if (down?.has_l1) {
    nextStep = 'CV-07';
    statusLabel = 'L-1 Rates Finalized · Prepare Working Costing & Bid Budget';
  } else if (down?.has_items) {
    nextStep = 'CV-03';
    statusLabel = 'Tender BOQ & Makes Logged · Floating RFQs & Vendor Quotes';
  } else if (biddingAuthorized) {
    nextStep = 'LG-03';
    statusLabel = 'Bidding Authorized · Pre-Bid Queries & Sourcing';
  } else if (tscCompleted) {
    nextStep = 'TS-02';
    statusLabel = `TSC Verified (${verdict?.human_decision.toUpperCase()}) · Next: Step 3 · Approval for Bidding the Tender`;
  } else if (hasVerified) {
    nextStep = 'TS-TSC';
    statusLabel = 'Documents Verified · Next: Step 2 · Prepare TSC';
  }

  return {
    draft_id: draftId,
    completed_steps: completedSteps,
    next_step: nextStep,
    status_label: statusLabel,
    has_documents: hasVerified,
    has_verified_batches: hasVerified,
    tsc_completed: tscCompleted,
    tsc_decision: verdict?.human_decision,
    bidding_authorized: biddingAuthorized,
    bidding_decision: bidding?.decision,
  };
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
  },
  actorId: string
): Promise<BiddingAuthorization> {
  return transaction(async (db) => {
    const res = await db.query<BiddingAuthorization>(
      `INSERT INTO tender.bidding_authorizations (
        draft_id, decision, manager_id, estimated_value, emd_amount,
        emd_exemption_claimed, tender_fee, tender_fee_exemption_claimed,
        due_date, approval_notes, authorized_by_id, authorized_at, version
      ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, clock_timestamp(), 1)
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
    };
  });
}
