import { AppError } from '../../errors';
import { trim } from '../../projects/validation';
import { lower } from '../../tenders/intake/receipts';
export { trim, lower };
export class PurchaseError extends AppError {
    constructor(status: number, message: string, public field = '') { super(status, message); }
}
export function invalid(field: string, message: string): never { throw new PurchaseError(422, message, field); }
export function uuid(value: unknown, field = 'id', message = 'Select a valid record'): string {
    if (typeof value !== 'string' || !/^[a-f\d]{8}(?:-[a-f\d]{4}){3}-[a-f\d]{12}$/i.test(value)) invalid(field, message);
    return value.toLowerCase();
}
export function string(value: unknown, field: string, fallback?: string): string {
    if (value === undefined && fallback !== undefined) return fallback;
    if (typeof value !== 'string') invalid(field, 'Use text');
    return value;
}
export function requiredText(value: unknown, field: string, message = 'Provide a value within 2000 characters') {
    const text = trim(string(value, field, ''));
    if (!text || [...text].length > 2000) invalid(field, message);
    return text;
}
export const reason = (value: unknown, message = 'Enter a reason within 2000 characters') => requiredText(value, 'reason', message);
export function version(value: unknown, field = 'version', minimum = 0) {
    if (!Number.isInteger(value) || Number(value) < minimum || Number(value) > 2147483647) invalid(field, 'Use the current saved version');
    return Number(value);
}
export function isoDate(value: string) {
    if (!/^\d{4}-\d{2}-\d{2}$/.test(value) || value.slice(0,4) === '0000') return false;
    const date = new Date(value+'T00:00:00Z');
    return Number.isFinite(date.getTime()) && date.toISOString().slice(0,10) === value;
}
export function object(value: unknown, allowed: readonly string[]) {
    if (!value || typeof value !== 'object' || Array.isArray(value) || Object.keys(value).some(key => !allowed.includes(key))) invalid('', 'Use a single JSON object with supported fields');
    return value as Record<string, unknown>;
}
