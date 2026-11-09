BEGIN;

-- 1. Pre-bid queries & Corrigendum Clarifications (LG-03 / LG-04)
CREATE TABLE IF NOT EXISTS tender.prebid_queries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    draft_id uuid NOT NULL REFERENCES tender.document_drafts(id),
    category varchar(32) NOT NULL DEFAULT 'technical' CHECK(category IN ('technical', 'commercial', 'make_list', 'boq', 'legal', 'other')),
    clause_reference varchar(255) NOT NULL DEFAULT '',
    tender_specification text NOT NULL,
    query_description text NOT NULL,
    justification text NOT NULL DEFAULT '',
    status varchar(32) NOT NULL DEFAULT 'draft' CHECK(status IN ('draft', 'submitted', 'answered', 'accepted', 'rejected')),
    client_response text NOT NULL DEFAULT '',
    corrigendum_reference varchar(255) NOT NULL DEFAULT '',
    submitted_at timestamptz,
    responded_at timestamptz,
    created_by_id uuid REFERENCES auth.users(id),
    updated_by_id uuid REFERENCES auth.users(id),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    version integer NOT NULL DEFAULT 1
);

-- 2. Tender Working BOQ Items & Approved Make List (CV-01 / CV-02)
CREATE TABLE IF NOT EXISTS tender.bid_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    draft_id uuid NOT NULL REFERENCES tender.document_drafts(id),
    item_code varchar(100) NOT NULL DEFAULT '',
    description text NOT NULL,
    category varchar(100) NOT NULL DEFAULT 'General',
    unit varchar(32) NOT NULL DEFAULT 'NOS',
    quantity numeric(16, 4) NOT NULL DEFAULT 1.0000,
    approved_makes text NOT NULL DEFAULT '',
    estimated_rate numeric(16, 2) NOT NULL DEFAULT 0.00,
    l1_vendor_name varchar(255) NOT NULL DEFAULT '',
    l1_unit_rate numeric(16, 2) NOT NULL DEFAULT 0.00,
    l1_total_amount numeric(16, 2) NOT NULL DEFAULT 0.00,
    sort_order integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    version integer NOT NULL DEFAULT 1
);

-- 3. Pre-Bid Vendor Quotations & L-1 Comparisons (CV-03 / CV-04 / CV-05)
CREATE TABLE IF NOT EXISTS tender.vendor_quotations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    draft_id uuid NOT NULL REFERENCES tender.document_drafts(id),
    bid_item_id uuid NOT NULL REFERENCES tender.bid_items(id) ON DELETE CASCADE,
    vendor_id uuid REFERENCES public.suppliers(id),
    vendor_name varchar(255) NOT NULL,
    make_offered varchar(255) NOT NULL DEFAULT '',
    unit_rate numeric(16, 2) NOT NULL DEFAULT 0.00,
    freight_and_insurance numeric(16, 2) NOT NULL DEFAULT 0.00,
    gst_percent numeric(5, 2) NOT NULL DEFAULT 18.00,
    delivery_weeks integer NOT NULL DEFAULT 4,
    is_technically_compliant boolean NOT NULL DEFAULT true,
    is_l1 boolean NOT NULL DEFAULT false,
    remarks text NOT NULL DEFAULT '',
    quote_ref varchar(100) NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    version integer NOT NULL DEFAULT 1
);

-- 4. Final Bid Working Costing Sheet & Budget Estimation (CV-07 / CV-08)
CREATE TABLE IF NOT EXISTS tender.bid_costings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    draft_id uuid NOT NULL UNIQUE REFERENCES tender.document_drafts(id),
    material_cost numeric(18, 2) NOT NULL DEFAULT 0.00,
    freight_cost numeric(18, 2) NOT NULL DEFAULT 0.00,
    civil_and_erection_cost numeric(18, 2) NOT NULL DEFAULT 0.00,
    contingency_percent numeric(5, 2) NOT NULL DEFAULT 3.00,
    contingency_amount numeric(18, 2) NOT NULL DEFAULT 0.00,
    overhead_percent numeric(5, 2) NOT NULL DEFAULT 5.00,
    overhead_amount numeric(18, 2) NOT NULL DEFAULT 0.00,
    margin_percent numeric(5, 2) NOT NULL DEFAULT 10.00,
    margin_amount numeric(18, 2) NOT NULL DEFAULT 0.00,
    total_bid_value numeric(18, 2) NOT NULL DEFAULT 0.00,
    status varchar(32) NOT NULL DEFAULT 'draft' CHECK(status IN ('draft', 'approved', 'submitted', 'awarded', 'lost')),
    submission_date date,
    portal_reference varchar(255) NOT NULL DEFAULT '',
    award_date date,
    contract_reference varchar(255) NOT NULL DEFAULT '',
    project_id uuid REFERENCES public.projects(id),
    approved_by_id uuid REFERENCES auth.users(id),
    approved_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    version integer NOT NULL DEFAULT 1
);

COMMIT;
