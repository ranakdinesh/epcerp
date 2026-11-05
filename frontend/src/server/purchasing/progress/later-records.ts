import type { DB } from '../../db';
import { AppError } from '../../errors';
import { sameSource } from '../orders/policy';
import type { Evidence } from './types';
type Source = {
    project_id: string;
    package_id: string;
    available: boolean;
    blockers: string[];
};
type Record<S> = {
    project_id: string;
    package_id: string;
    is_active: boolean;
    source_snapshot: S;
};
// Reporting alone ignores live availability flags. Current owned source identities and
// validation still determine whether stored completion evidence earns credit.
async function owned<S extends Source, T extends Record<S>>(db: DB, p: string, id: string, c: {
    number: number;
    get: (db: DB, p: string, id: string) => Promise<T | null>;
    source: (db: DB, p: string, id: string) => Promise<S>;
    guard: (s: S) => void;
    pending: (r: T) => boolean;
    complete: (r: T, s: S) => unknown;
    details: (r: T) => string;
    obsolete?: string;
    incomplete?: string;
}): Promise<Evidence> { const out = { number: c.number, state: 'not_started', details: '' }, r = await c.get(db, p, id); if (!r?.is_active)
    return out; const current = await c.source(db, p, id), s = { ...current, available: true, blockers: [] }; try {
    c.guard(s);
    if (r.project_id !== p || r.package_id !== id || !sameSource(r.source_snapshot, s))
        throw new AppError(422, 'Obsolete');
}
catch (e) {
    if (!(e instanceof AppError))
        throw e;
    return { ...out, state: 'blocked', details: c.obsolete ?? 'Recorded evidence does not match current upstream records' };
} if (c.pending(r))
    return out; try {
    c.complete(r, s);
}
catch (e) {
    if (!(e instanceof AppError))
        throw e;
    return { ...out, state: 'blocked', details: c.incomplete ?? 'Stored completion evidence is incomplete' };
} return { ...out, state: 'completed', details: c.details(r) }; }
import { store as reportsStore } from '../inspection-reports/repository';
import { source as reportsSource } from '../inspection-reports/context';
import { guard as reportsGuard, issued as reportsFinal } from '../inspection-reports/policy';
import { store as clearanceStore } from '../client-clearance/repository';
import { source as clearanceSource } from '../client-clearance/context';
import { guard as clearanceGuard, cleared as clearanceFinal } from '../client-clearance/policy';
import { store as dispatchStore } from '../dispatch/repository';
import { source as dispatchSource } from '../dispatch/context';
import { guard as dispatchGuard, dispatched as dispatchFinal } from '../dispatch/policy';
import { store as receiptsStore } from '../site-receipts/repository';
import { source as receiptsSource } from '../site-receipts/context';
import { guard as receiptsGuard, delivered as receiptsFinal } from '../site-receipts/policy';
import { store as grnsStore } from '../grns/repository';
import { source as grnsSource } from '../grns/context';
import { guard as grnsGuard, issued as grnsFinal } from '../grns/policy';
import { store as engineerStore } from '../engineer-confirmations/repository';
import { source as engineerSource } from '../engineer-confirmations/context';
import { guard as engineerGuard, confirmed as engineerFinal } from '../engineer-confirmations/policy';
import { store as accountsStore } from '../accounts-acceptance/repository';
import { source as accountsSource } from '../accounts-acceptance/context';
import { guard as accountsGuard, accepted as accountsFinal } from '../accounts-acceptance/policy';
export const reports = (db: DB, p: string, id: string) => owned(db, p, id, { number: 13, get: reportsStore.get, source: reportsSource, guard: reportsGuard, pending: r => r.report.status === 'pending', complete: reportsFinal, details: r => r.report.irn_number, obsolete: 'Report evidence does not match the current cleared inspection', incomplete: 'Stored IR / IRN issuance evidence is incomplete' });
export const clearance = (db: DB, p: string, id: string) => owned(db, p, id, { number: 15, get: clearanceStore.get, source: clearanceSource, guard: clearanceGuard, pending: r => r.clearance.status === 'pending', complete: clearanceFinal, details: r => r.clearance.mdcc_ref });
export const dispatch = (db: DB, p: string, id: string) => owned(db, p, id, { number: 16, get: dispatchStore.get, source: dispatchSource, guard: dispatchGuard, pending: r => r.dispatch.status === 'pending', complete: dispatchFinal, details: r => r.dispatch.lr_number });
export const receipts = (db: DB, p: string, id: string) => owned(db, p, id, { number: 17, get: receiptsStore.get, source: receiptsSource, guard: receiptsGuard, pending: r => r.receipt.status === 'pending' || r.receipt.status === 'in_transit', complete: receiptsFinal, details: r => r.receipt.arrival_date });
const grnBase = (db: DB, p: string, id: string) => owned(db, p, id, { number: 18, get: grnsStore.get, source: grnsSource, guard: grnsGuard, pending: r => r.grn.status === 'pending', complete: grnsFinal, details: r => r.grn.grn_number });
export const engineer = (db: DB, p: string, id: string) => owned(db, p, id, { number: 19, get: engineerStore.get, source: engineerSource, guard: engineerGuard, pending: r => r.confirmation.status === 'pending', complete: engineerFinal, details: r => r.confirmation.engineer_name });
export const accounts = (db: DB, p: string, id: string) => owned(db, p, id, { number: 20, get: accountsStore.get, source: accountsSource, guard: accountsGuard, pending: r => r.acceptance.status === 'pending', complete: accountsFinal, details: r => r.acceptance.officer_name });
export async function grns(db: DB, p: string, id: string) { const result = await grnBase(db, p, id); if (result.state === 'completed' && !(await grnsStore.get(db, p, id))!.grn.warranty_ref)
    return { ...result, state: 'in_progress', details: 'Issued GRN awaits warranty reference' }; return result; }
