import type { Vendor, Line } from '../groups/types';
export type Terms = {
    source: string;
    notes: string[];
    conditions: string[];
    delivery_basis: string;
};
export type Data = {
    terms?: Terms;
    generated_by_name: string;
    project_name: string;
    group_name: string;
    scope_revision: number;
    vendor: Vendor;
    lines: Line[];
    number: string;
    generated_at: string;
    generated_by_id: string;
    print_requests: {
        requested_at: string;
        actor_id: string;
    }[];
    sent_date: string | null;
    channel: string;
    dispatch_reference: string;
    sent_recorded_at: string | null;
    sent_by_id: string | null;
    is_current: boolean;
};
export type Document = {
    id: string;
    project_id: string;
    bo_id: string;
    group_id: string;
    vendor_id: string;
    version: number;
    data: Data;
    is_active: boolean;
    created_at: string;
    updated_at: string;
    created_by_id: string | null;
    updated_by_id: string | null;
};
export type Command = {
    action: string;
    group_id: string | null;
    vendor_id: string | null;
    document_id: string | null;
    version: number;
    group_version: number;
    bo_version: number;
    sent_date: string | null;
    channel: string;
    dispatch_reference: string;
};
