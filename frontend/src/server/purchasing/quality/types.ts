import type { PackageRecord } from '../core/package-store';
export type Source = {
    project_id: string;
    package_id: string;
    report_id: string;
    report_version: number;
    report_revision: number;
    ir_number: string;
    irn_number: string;
    issue_date: string;
    cleared_quantity: string;
    uom: string;
    order_id: string;
    order_version: number;
    number: string;
    supplier_name: string;
    description: string;
    inspection_id: string;
    inspection_version: number;
    available: boolean;
    blockers: string[];
};
export type Details = {
    cert_number: string;
    issued_by: string;
    valid_until: string;
    file_name: string;
};
export type Certificate = Details & {
    code: string;
    title: string;
    verified: boolean;
    verified_at: string | null;
    verified_by_id: string;
    is_active: boolean;
    created_at: string;
    updated_at: string;
    created_by_id: string;
    updated_by_id: string;
};
export type Record = PackageRecord & {
    revision: number;
    source_snapshot: Source;
    certificates: Certificate[];
};
export type Command = {
    version: number;
    action: string;
    reason: string;
    certificate_code: string;
    details: Details | null;
};
export type Capabilities = {
    read: boolean;
    write: boolean;
    verify: boolean;
    work: boolean;
    scoped: boolean;
};
