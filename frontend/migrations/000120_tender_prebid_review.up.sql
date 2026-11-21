BEGIN;

ALTER TABLE tender.document_drafts
    ADD COLUMN basic_details jsonb NOT NULL DEFAULT '{}'::jsonb
        CHECK (jsonb_typeof(basic_details) = 'object'),
    ADD COLUMN prebid_reviewed_at timestamptz,
    ADD COLUMN prebid_reviewed_by_id uuid REFERENCES auth.users(id);

-- Preserve completed scrutiny when all existing queries are already resolved.
WITH completed AS (
    SELECT DISTINCT ON (draft_id) draft_id, verified_at, verified_by_id
    FROM tender.tsc_eligibility_verdicts
    WHERE human_verified AND verified_at IS NOT NULL
    ORDER BY draft_id, verified_at DESC
)
UPDATE tender.document_drafts draft
SET prebid_reviewed_at = completed.verified_at,
    prebid_reviewed_by_id = completed.verified_by_id
FROM completed
WHERE draft.id = completed.draft_id
  AND NOT EXISTS (
      SELECT FROM tender.prebid_queries query
      WHERE query.draft_id = draft.id AND query.status IN ('draft', 'submitted')
  );

COMMIT;
