import { packageStore } from '../core/package-store';
import { stale } from './policy';
import type { Record } from './types';
export const store = packageStore<Record>({ table: 'purchase_engineer_confirmations', events: 'purchase_engineer_confirmations_events', module: 'purchase_engineer_confirmations', type: 'purchase_engineer_confirmations', stale, columns: r => ({ grn_id: r.source_snapshot.grn_id, grn_version: r.source_snapshot.grn_version, revision: r.revision, status: r.confirmation.status, source_snapshot: r.source_snapshot }), decode: (r, row) => ({ ...r, revision: Number(row.revision), source_snapshot: row.source_snapshot as Record['source_snapshot'], confirmation: { ...r.confirmation, status: row.status as string } }) });
