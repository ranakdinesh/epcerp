import 'server-only';
import { tenderProcess } from '../../components/processes/process-directory';
import type { DB } from '../db';
import { AppError } from '../errors';

// A skip applies only to the review closure it recorded. Query changes reopen that review.
const skippedReplies = `EXISTS(
  SELECT FROM tender.draft_events e
  WHERE e.draft_id=d.id AND e.action='prebid_replies_skipped' AND e.created_at=d.prebid_reviewed_at
)`;

// The draft row is aliased as d. Read the same facts for both the register and detail.
export const tenderFlowColumns = `
  EXISTS(SELECT FROM document_package_receipts r WHERE r.draft_id=d.id AND r.state IN ('consumed','ready'))
    OR EXISTS(SELECT FROM tender.draft_sources s WHERE s.draft_id=d.id) AS has_verified,
  (SELECT json_build_object(
    'human_decision', v.human_decision,
    'current', v.human_verified,
    'previously_verified', v.verified_at IS NOT NULL
  ) FROM tender.tsc_eligibility_verdicts v WHERE v.draft_id=d.id ORDER BY evaluated_at DESC, id DESC LIMIT 1) AS verdict,
  (SELECT b.decision FROM tender.bidding_authorizations b WHERE b.draft_id=d.id) AS bidding_decision,
  EXISTS(SELECT FROM tender.bidding_authorizations b
    WHERE b.draft_id=d.id AND b.authorized_at >= d.prebid_reviewed_at AND b.authorized_at >= (
      SELECT v.verified_at FROM tender.tsc_eligibility_verdicts v
      WHERE v.draft_id=d.id ORDER BY evaluated_at DESC, id DESC LIMIT 1
    )) AS bidding_is_current,
  EXISTS(SELECT FROM tender.prebid_queries q WHERE q.draft_id=d.id) AS has_queries,
  EXISTS(SELECT FROM tender.prebid_queries q WHERE q.draft_id=d.id AND q.status='draft') AS has_draft_queries,
  EXISTS(SELECT FROM tender.prebid_queries q WHERE q.draft_id=d.id AND q.status='submitted') AS has_pending_replies,
  ${skippedReplies} AS skipped_replies,
  EXISTS(SELECT FROM tender.bid_items i WHERE i.draft_id=d.id) AS has_items,
  EXISTS(SELECT FROM tender.vendor_quotations q WHERE q.draft_id=d.id) AS has_quotes,
  EXISTS(SELECT FROM tender.vendor_quotations q WHERE q.draft_id=d.id AND q.is_l1) AS has_l1,
  (SELECT c.status FROM tender.bid_costings c WHERE c.draft_id=d.id) AS costing_status`;

const steps = new Map(tenderProcess.steps.map(step => [step.code, step]));
type Verdict = { current?: boolean; previously_verified?: boolean; human_decision?: string };

export function tenderFlow(row: Record<string, unknown>) {
  const verdict = row.verdict as Verdict | null;
  const reviewed = Boolean(row.prebid_reviewed_at && !row.has_draft_queries && (row.skipped_replies || !row.has_pending_replies));
  const skipped = reviewed && !row.has_queries;
  const repliesSkipped = reviewed && (skipped || Boolean(row.skipped_replies));
  const finalized = Boolean(verdict?.current);
  const bidding = typeof row.bidding_decision === 'string' ? row.bidding_decision : undefined;
  const approvalCurrent = Boolean(reviewed && finalized && row.bidding_is_current);
  const authorized = approvalCurrent && (bidding === 'authorized' || bidding === 'conditional');
  const submitted = row.costing_status === 'submitted' || row.costing_status === 'awarded';
  const completed = ['LG-01'];
  if (row.has_verified) completed.push('TS-01');
  if (reviewed || (row.has_queries && !row.has_draft_queries)) completed.push('LG-03');
  if (reviewed) completed.push('LG-04');
  if (finalized) completed.push('TS-TSC');
  if (approvalCurrent) completed.push('TS-02');
  if (row.has_items) completed.push('CV-01');
  if (row.has_quotes) completed.push('CV-02', 'CV-03');
  if (row.has_l1) completed.push('CV-04', 'CV-05');
  if (row.costing_status === 'approved' || submitted) completed.push('CV-07');
  if (submitted) completed.push('CV-08');

  let next = 'TS-01';
  if (submitted) next = 'CV-08';
  else if (authorized) next = row.costing_status === 'approved' ? 'CV-08' : row.has_l1 ? 'CV-07' : row.has_items ? 'CV-03' : 'CV-01';
  else if (reviewed) next = repliesSkipped || finalized ? 'TS-02' : 'TS-TSC';
  else if (!finalized && !verdict?.previously_verified) {
    if (row.has_verified) next = 'TS-TSC';
  } else {
    next = row.has_draft_queries || !row.has_queries ? 'LG-03' : 'LG-04';
  }

  const step = steps.get(next)!;
  return {
    next_step: next,
    status_label: row.costing_status === 'awarded' ? 'Awarded · Project setup ready'
      : submitted ? 'Bid submitted · Awaiting award'
      : next === 'TS-02' && approvalCurrent && bidding === 'rejected' ? 'Bidding declined'
      : `Step ${step.number} · ${step.label}`,
    completed_steps: completed.map(code => steps.get(code)!.number).sort((a, b) => a - b),
    has_documents: Boolean(row.has_verified),
    has_verified_batches: Boolean(row.has_verified),
    prebid_review_closed: reviewed,
    prebid_review_skipped: skipped,
    prebid_replies_skipped: repliesSkipped,
    tsc_completed: finalized,
    tsc_decision: finalized ? verdict?.human_decision : undefined,
    bidding_authorized: authorized,
    bidding_decision: bidding,
  };
}

export async function requireClosedPrebidReview(db: DB, draftId: string) {
  const { rows: [draft] } = await db.query(`
    SELECT prebid_reviewed_at,
      EXISTS(SELECT FROM tender.prebid_queries q WHERE q.draft_id=d.id AND q.status='draft') OR
      (NOT ${skippedReplies} AND EXISTS(
        SELECT FROM tender.prebid_queries q WHERE q.draft_id=d.id AND q.status='submitted'
      )) AS open
    FROM tender.document_drafts d WHERE id=$1 FOR UPDATE
  `, [draftId]);
  if (!draft?.prebid_reviewed_at || draft.open) {
    throw new AppError(409, 'Close pre-bid review before recording the bidding decision.');
  }
}
