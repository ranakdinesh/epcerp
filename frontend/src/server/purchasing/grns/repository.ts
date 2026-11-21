import { packageStore } from '../core/package-store';
import { stale } from './policy';
import type { Record } from './types';
export const store = packageStore<Record>({ table: 'purchase_grns', events: 'purchase_grns_events', module: 'purchase_grns', type: 'purchase_grns', stale, columns: r => ({ receipt_id: r.source_snapshot.receipt_id, receipt_version: r.source_snapshot.receipt_version, revision: r.revision, status: r.grn.status, source_snapshot: r.source_snapshot }), decode: (r, row) => ({ ...r, revision: Number(row.revision), source_snapshot: row.source_snapshot as Record['source_snapshot'], grn: { ...r.grn, status: row.status as string } }) });
