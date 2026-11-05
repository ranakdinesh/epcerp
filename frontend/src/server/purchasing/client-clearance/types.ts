import type { PackageRecord } from '../core/package-store';
import type { Source as QualitySource } from '../quality/types';
export type Source = QualitySource & {
    quality_id: string;
    quality_version: number;
    quality_revision: number;
};
export type Clearance = {
    status: string;
    mdcc_ref: string;
    clearance_date: string;
    cleared_by_authority: string;
    remarks: string;
    file_name: string;
};
export type Record = PackageRecord & {
    revision: number;
    source_snapshot: Source;
    clearance: Clearance;
};
export type Command = {
    version: number;
    action: string;
    reason: string;
    clearance: Clearance | null;
};
