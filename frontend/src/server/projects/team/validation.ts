import { ProjectError, trim } from '../validation';
import { lower } from '../../tenders/intake/receipts';
export { ProjectError as TeamError } from '../validation';
export const denied = () => new ProjectError(403, 'Project responsibility access denied');
export const missing = () => new ProjectError(404, 'Project or responsibility not found');
export const stale = () => new ProjectError(409, 'Responsibility changed; reload before saving');
export function invalid(field: string, message: string): never { throw new ProjectError(422, message, field); }
export function id(value: unknown): string {
    if (typeof value !== 'string' || !/^[a-f\d]{8}(?:-[a-f\d]{4}){3}-[a-f\d]{12}$/i.test(value)) invalid('id', 'Choose a valid record identifier');
    return value.toLowerCase();
}
export function scoped(code: string) {
    if (['accounts.purchase_acceptance.', 'accounts.quotation_approvals.', 'accounts.purchase_advances.'].some(prefix => code.startsWith(prefix))) return true;
    return code.startsWith('projects.') && code !== 'projects.team.write' && !['projects.portfolio.', 'projects.purchase_portfolio.', 'projects.master.', 'projects.manager_assignments.', 'projects.purchase_assignments.', 'projects.processes.bypass'].some(prefix => code.startsWith(prefix));
}
export const fields = ['user_id', 'bundle_id', 'responsibility', 'permission_codes', 'is_active', 'version', 'reason'];
export function input(raw: Record<string, unknown>) {
    for (const key of fields) {
        if (!(key in raw)) invalid(key, 'This field is required');
        if (raw[key] === null) invalid(key, 'This field must not be null');
    }
    if (typeof raw.responsibility !== 'string' || typeof raw.reason !== 'string' || typeof raw.is_active !== 'boolean' || !Number.isInteger(raw.version) || Number(raw.version) < -2147483648 || Number(raw.version) > 2147483647 || !Array.isArray(raw.permission_codes) || raw.permission_codes.some(v => typeof v !== 'string')) invalid('', 'Invalid responsibility input');
    const user_id = id(raw.user_id), bundle_id = id(raw.bundle_id), responsibility = trim(raw.responsibility), reason = trim(raw.reason);
    if (!responsibility || [...responsibility].length > 80 || ['project manager', 'pm', 'procurement executive', 'purchase manager'].includes(lower(responsibility))) invalid('responsibility', 'Choose a specialist responsibility; manage PM and Procurement Executive in their owning workspaces');
    if (!reason || [...reason].length > 2000) invalid('reason', 'Record a reason of at most 2000 characters');
    if (Number(raw.version) < 0 || !raw.permission_codes.length || raw.permission_codes.length > 100) invalid('permission_codes', 'Provide a bounded permission profile and current version');
    const permission_codes = [...raw.permission_codes as string[]].sort();
    if (permission_codes.some((code, index) => !scoped(code) || index > 0 && permission_codes[index - 1] === code)) invalid('permission_codes', 'Use distinct project-scoped permission codes');
    return { user_id, bundle_id, responsibility, reason, permission_codes, is_active: raw.is_active, version: Number(raw.version) };
}
