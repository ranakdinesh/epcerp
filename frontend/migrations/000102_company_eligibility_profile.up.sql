BEGIN;

-- Deployment-wide Company Profile (Singleton)
CREATE TABLE IF NOT EXISTS public.company_profile (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    singleton boolean NOT NULL DEFAULT true UNIQUE CHECK (singleton = true),
    legal_name varchar(255) NOT NULL,
    trade_name varchar(255) NOT NULL DEFAULT '',
    cin varchar(32) NOT NULL DEFAULT '',
    pan varchar(16) NOT NULL DEFAULT '',
    gstin varchar(20) NOT NULL DEFAULT '',
    incorporation_date date,
    registered_address text NOT NULL DEFAULT '',
    registered_state varchar(100) NOT NULL DEFAULT '',
    registered_city varchar(100) NOT NULL DEFAULT '',
    registered_pincode varchar(16) NOT NULL DEFAULT '',
    epfo_registration varchar(50) NOT NULL DEFAULT '',
    esic_registration varchar(50) NOT NULL DEFAULT '',
    msme_udyam_number varchar(50) NOT NULL DEFAULT '',
    msme_category varchar(20) NOT NULL DEFAULT '' CHECK(msme_category IN ('', 'micro', 'small', 'medium')),
    msme_activity varchar(20) NOT NULL DEFAULT '' CHECK(msme_activity IN ('', 'manufacturing', 'services', 'trading')),
    startup_recognition_number varchar(50) NOT NULL DEFAULT '',
    iso_9001 boolean NOT NULL DEFAULT false,
    iso_14001 boolean NOT NULL DEFAULT false,
    iso_45001 boolean NOT NULL DEFAULT false,
    non_blacklisted_declaration boolean NOT NULL DEFAULT true,
    clean_insolvency_declaration boolean NOT NULL DEFAULT true,
    version integer NOT NULL DEFAULT 1,
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_by_id uuid REFERENCES auth.users(id)
);

-- Company Financial Years (Turnover, Net Worth, Profitability)
CREATE TABLE IF NOT EXISTS public.company_financial_years (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    financial_year varchar(16) NOT NULL UNIQUE,
    turnover_excluding_gst numeric(16, 2) NOT NULL DEFAULT 0.00,
    net_worth numeric(16, 2) NOT NULL DEFAULT 0.00,
    profit_after_tax numeric(16, 2) NOT NULL DEFAULT 0.00,
    is_audited boolean NOT NULL DEFAULT true,
    version integer NOT NULL DEFAULT 1,
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_by_id uuid REFERENCES auth.users(id)
);

-- Company Financial & Accreditation Certificates (Solvency, Working Capital, ISO)
CREATE TABLE IF NOT EXISTS public.company_certificates (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    kind varchar(32) NOT NULL CHECK(kind IN ('solvency', 'working_capital', 'bank_guarantee_limit', 'iso_certificate')),
    certificate_number varchar(100) NOT NULL DEFAULT '',
    issuer_name varchar(255) NOT NULL,
    amount numeric(16, 2) NOT NULL DEFAULT 0.00,
    issued_on date,
    valid_until date,
    is_active boolean NOT NULL DEFAULT true,
    version integer NOT NULL DEFAULT 1,
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_by_id uuid REFERENCES auth.users(id)
);

-- Company Electrical Contractor Licences (State-wise with Voltage Authorization)
CREATE TABLE IF NOT EXISTS public.company_licences (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    licence_number varchar(100) NOT NULL,
    state varchar(100) NOT NULL,
    class_grade varchar(50) NOT NULL DEFAULT 'Class A',
    maximum_voltage_kv numeric(6, 2) NOT NULL DEFAULT 33.00,
    issued_on date,
    valid_until date NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    version integer NOT NULL DEFAULT 1,
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_by_id uuid REFERENCES auth.users(id),
    UNIQUE(state, licence_number)
);

-- Company Work Experience / Executed Projects Repository
CREATE TABLE IF NOT EXISTS public.company_work_experience (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_title varchar(255) NOT NULL,
    client_name varchar(255) NOT NULL,
    client_type varchar(32) NOT NULL DEFAULT 'psu' CHECK(client_type IN ('central_govt', 'state_govt', 'psu', 'public_ltd', 'private')),
    scope_description text NOT NULL DEFAULT '',
    executed_value_excluding_gst numeric(16, 2) NOT NULL DEFAULT 0.00,
    state varchar(100) NOT NULL DEFAULT '',
    voltage_kv numeric(6, 2) DEFAULT NULL,
    capacity_description varchar(100) NOT NULL DEFAULT '',
    company_share_percent numeric(5, 2) NOT NULL DEFAULT 100.00,
    completed_on date NOT NULL,
    completion_certificate_ref varchar(100) NOT NULL DEFAULT '',
    is_active boolean NOT NULL DEFAULT true,
    version integer NOT NULL DEFAULT 1,
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_by_id uuid REFERENCES auth.users(id)
);

-- Tender Eligibility Verdicts & Human Verification
CREATE TABLE IF NOT EXISTS tender.tsc_eligibility_verdicts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    draft_id uuid NOT NULL REFERENCES tender.document_drafts(id),
    job_id uuid NOT NULL REFERENCES tender.tsc_workspaces(id),
    verdict varchar(32) NOT NULL CHECK(verdict IN ('eligible', 'not_eligible', 'conditionally_eligible')),
    overall_summary text NOT NULL,
    criteria_matrix jsonb NOT NULL DEFAULT '[]',
    gaps_and_risks jsonb NOT NULL DEFAULT '[]',
    model_used varchar(100) NOT NULL DEFAULT '',
    evaluated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    human_verified boolean NOT NULL DEFAULT false,
    human_decision varchar(32) NOT NULL DEFAULT 'pending' CHECK(human_decision IN ('pending', 'accepted', 'rejected', 'overridden')),
    human_remarks text NOT NULL DEFAULT '',
    verified_by_id uuid REFERENCES auth.users(id),
    verified_at timestamptz,
    version integer NOT NULL DEFAULT 1,
    UNIQUE(draft_id, job_id)
);

-- Tender Bidding Approvals (Step 3: TS-02)
CREATE TABLE IF NOT EXISTS tender.bidding_authorizations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    draft_id uuid NOT NULL REFERENCES tender.document_drafts(id),
    decision varchar(32) NOT NULL CHECK(decision IN ('authorized', 'rejected', 'conditional')),
    manager_id uuid REFERENCES auth.users(id),
    estimated_value numeric(18, 2),
    emd_amount numeric(18, 2),
    emd_exemption_claimed boolean NOT NULL DEFAULT false,
    tender_fee numeric(18, 2),
    tender_fee_exemption_claimed boolean NOT NULL DEFAULT false,
    due_date date,
    approval_notes text NOT NULL DEFAULT '',
    authorized_by_id uuid REFERENCES auth.users(id),
    authorized_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    version integer NOT NULL DEFAULT 1,
    UNIQUE(draft_id)
);

COMMIT;
