import 'server-only';
import type { DB } from '../../db';
import { transaction } from '../../db';
import { requireAnyPermission } from '../../auth/access';
import { limitPublic } from '../../auth/rate-limit';
import { AppError } from '../../errors';
export type Area = 'draft' | 'receipt' | 'format';
export async function access(db: DB, token: string, area: Area, write = false) {
    const required = area === 'format' ? ['settings.tsc_formats.read', ...(write ? ['settings.tsc_formats.write'] : [])] : ['tenders.register.view', 'tenders.register.read', ...(area === 'draft' ? ['tenders.processing.read'] : []), ...(write ? ['tenders.register.write', 'tenders.processing.write'] : [])];
    const principal = await requireAnyPermission(db, token, [required[0]], true);
    if (required.some(p => !principal.allows(p)))
        throw new AppError(403, 'Tender permission required');
    return principal;
}
export async function admit(token: string, area: Area, operation: string, write = false) { const principal = await transaction(db => access(db, token, area, write)); const [maximum, seconds] = area === 'receipt' ? (operation === 'create' || operation === 'seal' ? [10, 600] : operation === 'upload' ? [20, 60] : [120, 60]) : area === 'format' ? (write ? [30, 60] : operation === 'preview' ? [5, 60] : [120, 60]) : write ? [10, 60] : [120, 60]; await limitPublic(`tender:${area}:${operation}:${principal.id}`, maximum, seconds); return principal.id; }
