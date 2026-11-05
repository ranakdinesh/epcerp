import type { PackageRecord } from '../core/package-store';
import type { Source as ReceiptSource } from '../site-receipts/types';
export type Source = ReceiptSource & {
    receipt_id: string;
    receipt_version: number;
    receipt_revision: number;
    arrival_date: string;
    receiving_officer: string;
    packages_received: number;
    packages_condition: string;
    transit_damage_notes: string;
    gate_entry_ref: string;
};
export type GRN = {
    status: string;
    grn_number: string;
    grn_date: string;
    received_qty: string;
    accepted_qty: string;
    rejected_qty: string;
    store_keeper_name: string;
    remarks: string;
    warranty_ref: string;
    warranty_valid_until: string;
    file_name: string;
    uom: string;
};
export type Record = PackageRecord & {
    revision: number;
    source_snapshot: Source;
    grn: GRN;
};
export type Command = {
    version: number;
    action: string;
    reason: string;
    grn: Omit<GRN, 'uom'> | null;
};
