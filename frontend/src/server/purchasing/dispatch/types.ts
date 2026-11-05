import type { PackageRecord } from '../core/package-store';
import type { Source as ClearanceSource } from '../client-clearance/types';
export type Source = ClearanceSource & {
    clearance_id: string;
    clearance_version: number;
    clearance_revision: number;
    mdcc_ref: string;
    clearance_date: string;
};
export type Dispatch = {
    status: string;
    dispatch_date: string;
    transporter_name: string;
    lr_number: string;
    lr_date: string;
    vehicle_number: string;
    eway_bill_number: string;
    eway_bill_valid_until: string;
    invoice_number: string;
    invoice_date: string;
    driver_contact: string;
    file_name: string;
};
export type Record = PackageRecord & {
    revision: number;
    source_snapshot: Source;
    dispatch: Dispatch;
};
export type Command = {
    version: number;
    action: string;
    reason: string;
    dispatch: Dispatch | null;
};
