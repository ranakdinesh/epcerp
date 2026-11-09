import 'server-only';
import { pool, transaction } from '../../db';
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

  const db = pool();
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
  return res.rows[0];
}

export async function updatePrebidQuery(
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
  const db = pool();
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

  let submittedAt = existing.submitted_at;
  if (status === 'submitted' && !existing.submitted_at) {
    submittedAt = new Date().toISOString();
  }

  let respondedAt = existing.responded_at;
  if ((status === 'answered' || status === 'accepted' || status === 'rejected') && !existing.responded_at) {
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
  return res.rows[0];
}

export async function deletePrebidQuery(draftId: string, queryId: string): Promise<{ ok: boolean }> {
  const db = pool();
  const res = await db.query(
    `DELETE FROM tender.prebid_queries WHERE id = $1 AND draft_id = $2`,
    [queryId, draftId]
  );
  if (res.rowCount === 0) throw new AppError(404, 'Pre-bid query not found');
  return { ok: true };
}
