import { packageStore } from '../core/package-store';
import { stale } from './policy';
import type { Record } from './types';
export const store = packageStore<Record>({ table: 'purchase_dispatches', events: 'purchase_dispatch_events', module: 'purchase_dispatch', type: 'purchase_dispatch', stale, columns: r => ({ clearance_id: r.source_snapshot.clearance_id, clearance_version: r.source_snapshot.clearance_version, revision: r.revision, status: r.dispatch.status, source_snapshot: r.source_snapshot }), decode: (r, row) => ({ ...r, revision: Number(row.revision), source_snapshot: row.source_snapshot as Record['source_snapshot'], dispatch: { ...r.dispatch, status: row.status as string } }) });
