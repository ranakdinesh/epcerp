import type { DB } from '../../db';
import { requireAnyPermission } from '../../auth/access';
import { AppError } from '../../errors';
import type { Actor } from '../core/access';
import { inspect } from '../workflow/reader';
import { stageAccess } from '../core/stage-access';
import { dispatchedFact } from '../dispatch/service';
import { blankSource as dispatchBlank } from '../dispatch/context';
import { invalid } from '../core/validation';
import type { Source } from './types';
export const actor = (db: DB, token: string) => requireAnyPermission(db, token, ['projects.purchase_site_receipts.read'], 'business');
export const access = (db: DB, a: Actor, p: string, id: string) => stageAccess(db, a, p, id, 'projects.purchase_site_receipts', 17);
export const blankSource = (p: string, id: string): Source => ({ ...dispatchBlank(p, id), dispatch_id: '', dispatch_version: 0, dispatch_revision: 0, dispatch_date: '', lr_number: '', transporter_name: '' });
export async function source(db: DB, p: string, id: string): Promise<Source> { const { record, current } = await inspect(db, p, id), out: Source = { ...blankSource(p, id), available: current.project_available && current.eligible, blockers: [...current.blockers] }; for (const step of record.steps)
    if (step.number === 17 && step.state === 'blocked')
        out.blockers.push('Site material receipt has an unresolved blocker'); let actual; try {
    actual = await dispatchedFact(db, p, id);
}
catch (e) {
    if (e instanceof AppError && [404, 409, 422].includes(e.status)) {
        out.available = false;
        out.blockers.push(e.message);
        return out;
    }
    throw e;
} if (actual.project_id !== p || actual.package_id !== id)
    invalid('source', 'Dispatch belongs to another project/package'); const { available, blockers, ...facts } = actual.source_snapshot; void available; void blockers; return { ...out, ...facts, dispatch_id: actual.id, dispatch_version: actual.version, dispatch_revision: actual.revision, dispatch_date: actual.dispatch.dispatch_date, lr_number: actual.dispatch.lr_number, transporter_name: actual.dispatch.transporter_name }; }
