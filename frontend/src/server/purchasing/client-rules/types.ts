export type Data = {
    project_id: string | null;
    scope_code: string;
    mode: string;
    allowed: string[];
    excluded: string[];
    effective_from: string;
    effective_until: string | null;
    evidence: string;
};
export type Record = {
    id: string;
    client_id: string;
    data: Data;
    is_active: boolean;
    version: number;
    created_at: string;
    updated_at: string;
    created_by_id: string | null;
    updated_by_id: string | null;
};
