import { packageStore } from '../core/package-store';
import { stale } from './policy';
import type { Record } from './types';
export const store = packageStore<Record>({ table: 'purchase_inspection_reports', events: 'purchase_inspection_report_events', module: 'purchase_inspection_reports', type: 'purchase_inspection_reports', stale, columns: r => ({ inspection_id: r.source_snapshot.inspection_id, inspection_version: r.source_snapshot.inspection_version, revision: r.revision, status: r.report.status, source_snapshot: r.source_snapshot }), decode: (r, row) => ({ ...r, revision: Number(row.revision), source_snapshot: row.source_snapshot as Record['source_snapshot'], report: { ...r.report, status: row.status as string } }) });
