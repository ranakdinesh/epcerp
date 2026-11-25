import 'server-only';
import { pool, type DB } from '../../db';
import { AppError } from '../../errors';

export type PrebidQuery = {
  id: string;
  draft_id: string;
  category: 'technical' | 'commercial' | 'make_list' | 'boq' | 'legal' | 'other';
  clause_reference: string;
  tender_specification: string;
  query_description: string;
  justification: string;
  status: 'draft' | 'submitted' | 'answered' | 'accepted' | 'rejected';
  client_response: string;
  corrigendum_reference: string;
  submitted_at: string | null;
  responded_at: string | null;
  created_at: string;
  updated_at: string;
  version: number;
};

function isResolved(status: PrebidQuery['status']) {
  return status === 'answered' || status === 'accepted' || status === 'rejected';
}

export async function listPrebidQueries(draftId: string): Promise<PrebidQuery[]> {
  const db = pool();
  const res = await db.query<PrebidQuery>(
    `SELECT *
     FROM tender.prebid_queries
     WHERE draft_id = $1
     ORDER BY created_at ASC`,
    [draftId]
  );
  return res.rows;
}

export async function createPrebidQuery(
  db: DB,
  draftId: string,
  input: {
    category: PrebidQuery['category'];
    clause_reference?: string;
    tender_specification: string;
    query_description: string;
    justification?: string;
  },
  actorId: string
): Promise<PrebidQuery> {
  if (!input.tender_specification?.trim()) {
    throw new AppError(422, 'Tender specification / clause text is required');
  }
  if (!input.query_description?.trim()) {
    throw new AppError(422, 'Query description is required');
  }

  const res = await db.query<PrebidQuery>(
    `INSERT INTO tender.prebid_queries(
       draft_id, category, clause_reference, tender_specification,
       query_description, justification, status, created_by_id, updated_by_id
     ) VALUES ($1, $2, $3, $4, $5, $6, 'draft', $7, $7)
     RETURNING *`,
    [
      draftId,
      input.category || 'technical',
      input.clause_reference?.trim() || '',
      input.tender_specification.trim(),
      input.query_description.trim(),
      input.justification?.trim() || '',
      actorId,
    ]
  );
  await reopenPrebidReview(db, draftId);
  return res.rows[0];
}

export async function updatePrebidQuery(
  db: DB,
  draftId: string,
  queryId: string,
  input: {
    category?: PrebidQuery['category'];
    clause_reference?: string;
    tender_specification?: string;
    query_description?: string;
    justification?: string;
    status?: PrebidQuery['status'];
    client_response?: string;
    corrigendum_reference?: string;
  },
  actorId: string
): Promise<PrebidQuery> {
  const existingRes = await db.query<PrebidQuery>(
    `SELECT * FROM tender.prebid_queries WHERE id = $1 AND draft_id = $2`,
    [queryId, draftId]
  );
  const existing = existingRes.rows[0];
  if (!existing) throw new AppError(404, 'Pre-bid query not found');

  const category = input.category ?? existing.category;
  const clauseRef = input.clause_reference !== undefined ? input.clause_reference.trim() : existing.clause_reference;
  const tenderSpec = input.tender_specification !== undefined ? input.tender_specification.trim() : existing.tender_specification;
  const queryDesc = input.query_description !== undefined ? input.query_description.trim() : existing.query_description;
  const justification = input.justification !== undefined ? input.justification.trim() : existing.justification;
  const status = input.status ?? existing.status;
  const clientResponse = input.client_response !== undefined ? input.client_response.trim() : existing.client_response;
  const corrigendumRef = input.corrigendum_reference !== undefined ? input.corrigendum_reference.trim() : existing.corrigendum_reference;

  if (isResolved(status) && !clientResponse && !corrigendumRef) {
    throw new AppError(422, 'Record the issuer reply or corrigendum reference before resolving the query');
  }

  let submittedAt = existing.submitted_at;
  if (status === 'submitted' && !existing.submitted_at) {
    submittedAt = new Date().toISOString();
  }

  let respondedAt = existing.responded_at;
  if (isResolved(status) && !existing.responded_at) {
    respondedAt = new Date().toISOString();
  }

  const res = await db.query<PrebidQuery>(
    `UPDATE tender.prebid_queries
     SET category = $1, clause_reference = $2, tender_specification = $3,
         query_description = $4, justification = $5, status = $6,
         client_response = $7, corrigendum_reference = $8,
         submitted_at = $9, responded_at = $10,
         updated_by_id = $11, updated_at = clock_timestamp(), version = version + 1
     WHERE id = $12 AND draft_id = $13
     RETURNING *`,
    [
      category,
      clauseRef,
      tenderSpec,
      queryDesc,
      justification,
      status,
      clientResponse,
      corrigendumRef,
      submittedAt,
      respondedAt,
      actorId,
      queryId,
      draftId,
    ]
  );
  const updated = res.rows[0];
  const clarificationChanged = (isResolved(existing.status) || isResolved(updated.status)) &&
    (['category', 'clause_reference', 'tender_specification', 'query_description', 'status',
      'client_response', 'corrigendum_reference'] as const).some(field => existing[field] !== updated[field]);
  await reopenPrebidReview(db, draftId, clarificationChanged);
  return updated;
}

export async function deletePrebidQuery(db: DB, draftId: string, queryId: string): Promise<{ ok: boolean }> {
  const { rows: [deleted] } = await db.query<Pick<PrebidQuery, 'status'>>(
    `DELETE FROM tender.prebid_queries WHERE id = $1 AND draft_id = $2 RETURNING status`,
    [queryId, draftId]
  );
  if (!deleted) throw new AppError(404, 'Pre-bid query not found');
  await reopenPrebidReview(db, draftId, isResolved(deleted.status));
  return { ok: true };
}

async function reopenPrebidReview(db: DB, draftId: string, clarificationChanged = false) {
  if (clarificationChanged) {
    await db.query(`
      UPDATE tender.tsc_eligibility_verdicts SET human_verified=false, version=version+1
      WHERE draft_id=$1 AND human_verified
    `, [draftId]);
  }
  await db.query(`
    UPDATE tender.document_drafts SET prebid_reviewed_at=NULL, prebid_reviewed_by_id=NULL
    WHERE id=$1 AND prebid_reviewed_at IS NOT NULL
  `, [draftId]);
}

export async function closePrebidReview(db: DB, draftId: string, actorId: string, { skip }: { skip: boolean }) {
  const { rows: [row] } = await db.query(`
    SELECT EXISTS(SELECT FROM tender.prebid_queries WHERE draft_id=$1) AS has_queries,
           EXISTS(SELECT FROM tender.prebid_queries WHERE draft_id=$1 AND status IN ('draft','submitted')) AS open
  `, [draftId]);
  if (skip && row.has_queries) throw new AppError(409, 'This tender has pre-bid queries. Review them before continuing.');
  if (row.open) throw new AppError(409, 'Resolve all draft queries and outstanding issuer replies before closing pre-bid review.');
  await db.query(`
    UPDATE tender.document_drafts SET prebid_reviewed_at=clock_timestamp(), prebid_reviewed_by_id=$2
    WHERE id=$1 AND prebid_reviewed_at IS NULL
  `, [draftId, actorId]);
  return { prebid_review_closed: true, prebid_review_skipped: !row.has_queries };
}
