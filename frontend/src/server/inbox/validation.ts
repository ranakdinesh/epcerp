import { AppError } from '../errors';
export class InboxError extends AppError {
    constructor(status: number, message: string, public field = '') { super(status,message); }
}
export const invalid = (field: string, message: string): never => { throw new InboxError(422,message,field); };
export function noticeID(value: string) {
    const id = value.toLowerCase();
    if (!/^[a-f0-9]{8}(-[a-f0-9]{4}){3}-[a-f0-9]{12}$/.test(id)) invalid('id','Use a valid notice identifier');
    return id;
}
export function query(params: URLSearchParams) {
    for (const key of params.keys()) if (!['page','size','status'].includes(key) || params.getAll(key).length !== 1) invalid(key,'Unsupported inbox query');
    const number = (key: string, fallback: number) => {
        const raw = params.get(key); if (!raw) return fallback;
        if (!/^[+-]?\d+$/.test(raw) || !Number.isSafeInteger(Number(raw)) || Number(raw) < -2147483648 || Number(raw) > 2147483647) invalid(key,'Use a whole page or size');
        return Number(raw);
    };
    const page = number('page',1), size = number('size',20), status = params.get('status') ?? '';
    if (page < 1 || page > 100000 || ![20,50,100].includes(size)) invalid('page','Use bounded inbox pages of 20, 50 or 100 rows');
    if (!['','unread','read','acknowledged'].includes(status)) invalid('status','Unknown inbox status');
    return {page,size,status};
}
export function receiptVersion(value: unknown) {
    if (!value || typeof value !== 'object' || Array.isArray(value) || Object.keys(value).length !== 1 || !Object.hasOwn(value,'version')) invalid('version','Provide only the current notice version');
    const version = (value as {version: unknown}).version;
    if (typeof version !== 'number' || !Number.isInteger(version) || version < 1 || version > 2147483647) invalid('version','A positive whole notice version is required');
    return version;
}
