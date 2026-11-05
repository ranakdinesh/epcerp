import type { PackageRecord } from '../core/package-store';
import type { Source as EngineerSource } from '../engineer-confirmations/types';
export type Source = EngineerSource & {
    engineer_id: string;
    engineer_version: number;
    engineer_revision: number;
    confirmation_date: string;
    engineer_name: string;
    designation: string;
    mbook_ref: string;
    work_package_ref: string;
    engineer_remarks: string;
    currency: string;
};
export type Acceptance = {
    status: string;
    acceptance_date: string;
    officer_name: string;
    voucher_ref: string;
    booked_amount: string;
    notes: string;
};
export type Record = PackageRecord & {
    revision: number;
    source_snapshot: Source;
    acceptance: Acceptance;
};
export type Command = {
    version: number;
    action: string;
    reason: string;
    acceptance: Acceptance | null;
};
