import { packageStore } from '../core/package-store';
import { stale } from './policy';
import type { Record } from './types';
export const store = packageStore<Record>({ table: 'purchase_client_clearances', events: 'purchase_client_clearance_events', module: 'purchase_client_clearance', type: 'purchase_client_clearance', stale, columns: r => ({ quality_id: r.source_snapshot.quality_id, quality_version: r.source_snapshot.quality_version, revision: r.revision, status: r.clearance.status, source_snapshot: r.source_snapshot }), decode: (r, row) => ({ ...r, revision: Number(row.revision), source_snapshot: row.source_snapshot as Record['source_snapshot'], clearance: { ...r.clearance, status: row.status as string } }) });
