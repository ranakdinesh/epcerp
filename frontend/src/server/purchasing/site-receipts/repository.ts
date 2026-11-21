import { packageStore } from '../core/package-store';
import { stale } from './policy';
import type { Record } from './types';
export const store = packageStore<Record>({ table: 'purchase_site_receipts', events: 'purchase_site_receipts_events', module: 'purchase_site_receipts', type: 'purchase_site_receipts', stale, columns: r => ({ dispatch_id: r.source_snapshot.dispatch_id, dispatch_version: r.source_snapshot.dispatch_version, revision: r.revision, status: r.receipt.status, source_snapshot: r.source_snapshot }), decode: (r, row) => ({ ...r, revision: Number(row.revision), source_snapshot: row.source_snapshot as Record['source_snapshot'], receipt: { ...r.receipt, status: row.status as string } }) });
