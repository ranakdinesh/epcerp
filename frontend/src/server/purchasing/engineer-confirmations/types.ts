import type { PackageRecord } from '../core/package-store';
import type { Source as GRNSource } from '../grns/types';
export type Source = GRNSource & {
    grn_id: string;
    grn_version: number;
    grn_revision: number;
    grn_number: string;
    grn_date: string;
    received_qty: string;
    accepted_qty: string;
    rejected_qty: string;
    warranty_ref: string;
    warranty_valid_until: string;
};
export type Confirmation = {
    status: string;
    confirmation_date: string;
    engineer_name: string;
    designation: string;
    mbook_ref: string;
    work_package_ref: string;
    remarks: string;
};
export type Record = PackageRecord & {
    revision: number;
    source_snapshot: Source;
    confirmation: Confirmation;
};
export type Command = {
    version: number;
    action: string;
    reason: string;
    confirmation: Confirmation | null;
};
