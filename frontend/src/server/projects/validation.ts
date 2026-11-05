import { AppError } from '../errors';
export class ProjectError extends AppError {
    constructor(status: number, message: string, public field = '') { super(status, message); }
}
export function invalid(field: string, message: string): never { throw new ProjectError(422, message, field); }
export const trim = (s: string) => s.replace(/^[\t\n\v\f\r \u0085\u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000]+|[\t\n\v\f\r \u0085\u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000]+$/g, '');
export function id(value: unknown, field = 'id') {
    if (typeof value !== 'string' || !/^[a-f\d]{8}(?:-[a-f\d]{4}){3}-[a-f\d]{12}$/i.test(value)) invalid(field, 'Select a valid record');
    return value.toLowerCase();
}
export function text(value: unknown, field: string, max: number, required = false) {
    if (value == null) value = '';
    if (typeof value !== 'string') invalid(field, 'Use text');
    const s = trim(value);
    if ([...s].length > max || /[\uD800-\uDFFF]/u.test(s) || required && !s) invalid(field, 'Enter a nonempty value within the permitted length');
    return s;
}
export function version(value: unknown) { if (typeof value !== 'number' || !Number.isInteger(value) || value < 1 || value >= 2147483647) invalid('expected_version', 'A current project version is required'); return value; }
export function projectName(value: unknown) { const s = text(value, 'name', 255, true); if ([...s].length < 3) invalid('name', 'Enter at least three characters'); return s; }
export function filter(params: URLSearchParams) {
    for (const k of params.keys()) if (!['search', 'page', 'page_size'].includes(k) || params.getAll(k).length !== 1) invalid(k, 'Unknown or repeated query parameter');
    const number = (k: string, fallback: number) => { const raw = params.get(k); if (raw === null) return fallback; if (!/^[+-]?\d+$/.test(raw)) invalid(k, 'Use an integer'); return Number(raw); };
    const page = number('page', 1), size = number('page_size', 50), raw = params.get('search') ?? '';
    if (!Number.isSafeInteger(page) || page < 1 || page > 100000 || !Number.isSafeInteger(size) || size < 1 || size > 100 || [...raw].length > 255) invalid('query', 'Invalid pagination or search');
    return { page, size, search: trim(raw) };
}
export const captureFields = ['purchase_enabled','actual_end_date','short_name','project_type','site_name','site_state','site_city','scope_summary','contract_value','currency','loi_number','po_number','schedule_start_basis','planned_start_date','planned_completion_date','loi_date','po_date','actual_start_date','site_handover_date','planned_duration_days','safety_buffer_percent','safety_buffer_days','remarks'] as const;
export type Capture = Record<Exclude<typeof captureFields[number], 'purchase_enabled'>, string> & { purchase_enabled: boolean; operations_manager_id: string; schedule_calculation: Calculation };
export type Calculation = { contractual_start: string; contractual_start_gap: number|null; planned_start_gap: number|null; calendar_days: number|null; weekend_days: number|null; holiday_days: number|null; completion_days: number|null; reserved_buffer_days: number|null; execution_days: number|null };
export function capture(value: Record<string, unknown>): Capture {
    const out = Object.fromEntries(captureFields.filter(k => k !== 'purchase_enabled').map(k => { const s = value[k] ?? ''; if (typeof s !== 'string') invalid(k,'Use text'); return [k,s]; })) as Omit<Capture, 'purchase_enabled' | 'operations_manager_id' | 'schedule_calculation'>;
    if (value.purchase_enabled !== undefined && typeof value.purchase_enabled !== 'boolean') invalid('purchase_enabled','Use a boolean');
    return { ...out, purchase_enabled: value.purchase_enabled === true, operations_manager_id:'', schedule_calculation:blankCalculation() };
}
export function blankCalculation(): Calculation { return {contractual_start:'',contractual_start_gap:null,planned_start_gap:null,calendar_days:null,weekend_days:null,holiday_days:null,completion_days:null,reserved_buffer_days:null,execution_days:null}; }
