export type Evidence = {
    number: number;
    state: string;
    details: string;
};
export type Coordination = {
    number: number;
    state: string;
    planned_date: string;
    block_reason: string;
    doer_id: string;
    resolver_id: string;
    resolution_due: string;
};
export type Package = {
    id: string;
    demand_id: string;
    quantity: string;
    evidence: Evidence[];
    coordination: Coordination[];
    source_blockers: string[];
};
export type Line = {
    id: string;
    demand_id: string;
    quantity: string;
};
export type BO = {
    id: string;
    name: string;
    state: string;
    lines: Line[];
    grouped: Evidence[];
    packages: Package[];
};
export type Source = {
    project_id: string;
    project_start: string;
    project_end: string;
    bos: BO[];
    blockers: string[];
};
export type Stage = {
    number: number;
    title: string;
    weight: number;
    contribution_percent: number;
    state: string;
    details: string[];
    due_date: string;
    signal: string;
    missing_dates: boolean;
    days_overdue: number;
    blockers: string[];
};
export type Summary = {
    bo_id: string;
    name: string;
    progress_percent: number;
    signal: string;
    completion_date: string;
    issues: string[];
    scope_covered: boolean;
    steps: Stage[];
};
