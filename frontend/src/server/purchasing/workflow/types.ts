export type Member = {
    id: string;
    label: string;
    can_execute: boolean;
    can_resolve: boolean;
};
export type Source = {
    project_name: string;
    site: string;
    indent_number: string;
    demand_id: string;
    description: string;
    quantity: string;
    uom: string;
    required_date: string;
    full_kit: string;
    release_at: string;
    project_available: boolean;
    eligible: boolean;
    blockers: string[];
};
export type Step = {
    number: number;
    title: string;
    phase: string;
    evidence_required: string;
    team: string;
    doer_id: string | null;
    resolver_id: string | null;
    planned_date: string | null;
    state: string;
    notes: string;
    block_reason: string;
    blocked_at: string | null;
    blocked_resolver_id: string | null;
    resolution_due: string | null;
    updated_at: string;
    updated_by_id: string;
};
export type Package = {
    id: string;
    project_id: string;
    indent_id: string;
    demand_id: string;
    code: string;
    description: string;
    source_snapshot: Source;
    version: number;
    is_active: boolean;
    created_at: string;
    updated_at: string;
    created_by_id: string | null;
    updated_by_id: string | null;
    steps: Step[];
};
export type Assignment = {
    number: number;
    team: string;
    doer_id: string;
    resolver_id: string;
    planned_date: string;
};
export type Capabilities = {
    read: boolean;
    write: boolean;
    execute: boolean;
    resolve: boolean;
    recovery: boolean;
};
export type Command = {
    version: number;
    action: string;
    notes: string;
    reason: string;
    resolution_due: string | null;
};
