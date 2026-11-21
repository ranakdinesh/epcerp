import type { PackageRecord } from '../core/package-store';
import type { Source as DispatchSource } from '../dispatch/types';
export type Source = DispatchSource & {
    dispatch_id: string;
    dispatch_version: number;
    dispatch_revision: number;
    dispatch_date: string;
    lr_number: string;
    transporter_name: string;
};
export type Receipt = {
    status: string;
    arrival_date: string;
    receiving_officer: string;
    packages_received: number;
    packages_condition: string;
    transit_damage_notes: string;
    gate_entry_ref: string;
};
export type Record = PackageRecord & {
    revision: number;
    source_snapshot: Source;
    receipt: Receipt;
};
export type Command = {
    version: number;
    action: string;
    reason: string;
    receipt: Receipt | null;
};
