import 'server-only';
import { transaction } from '../../db';
import { requireAnyPermission } from '../../auth/access';
import { scoped } from '../sourcing/context';
import { uuid, PurchaseError } from '../core/validation';
import { source } from './source';
import { build } from './policy';
export async function report(token: string, p: string) { return transaction(async (db) => { const a = await requireAnyPermission(db, token, ['projects.purchase_progress.view'], true); p = uuid(p); if (!a.allows('projects.purchase_progress.read') || !await scoped(db, a, p, 'projects.purchase_progress.read', false))
    throw new PurchaseError(403, 'Current purchase progress permission and project scope required'); return build((await source(db, p)).source); }); }
