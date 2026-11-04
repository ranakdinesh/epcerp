export type Provider = 'gemini' | 'openai';
export type Mode = Provider | 'manual';
export type Question = {
    id: string;
    section: string;
    printed_number: string;
    question: string;
    manual_review: boolean;
};
export type Source = {
    id: string;
    receipt_id: string;
    sha256: string;
    path: string;
    kind: string;
    byte_size: number;
    pages: number;
    claim_id: string;
};
export type Citation = {
    document_id: string;
    page: number;
    quote: string;
};
export type Answer = {
    question_id: string;
    answer: string;
    status: 'answered' | 'needs_review' | 'not_found' | 'conflicting';
    citations: Citation[];
    accepted: boolean;
    origin: 'gemini' | 'openai' | 'manual';
};
export type Snapshot = {
    schema: 'tsc-cloud-v2';
    draft_id: string;
    draft_version: number;
    name: string;
    format_id: string;
    format_version: number;
    format_name: string;
    questions: Question[];
    sources: Source[];
    mode: Mode;
    auto_escalate: boolean;
    source_fingerprint: string;
};
export type Original = Source & {
    data: Buffer;
};
export type Usage = {
    input_tokens: number;
    output_tokens: number;
    cached_tokens: number;
    cache_write_tokens: number;
    thought_tokens: number;
};
export type Price = {
    version: string;
    model: string;
    provider: Provider;
    input_usd: number;
    output_usd: number;
    cached_usd: number;
    write_usd: number;
    usd_inr: number;
    fx_date: string;
    tier: 'free' | 'paid';
    source: string;
    max_input_tokens: number;
    max_output_tokens: number;
};
export type JobRow = {
    id: string;
    draft_id: string;
    draft_version: string;
    actor_id: string;
    mode: Mode;
    state: string;
    version: string;
    snapshot_cipher: Buffer;
    snapshot_hash: string;
    answers_cipher: Buffer;
    answers_hash: string;
    claim_id: string | null;
    lease_until: Date | null;
    error_code: string;
    created_at: Date;
    updated_at: Date;
    request_hash: string;
    operation_key: string;
};
