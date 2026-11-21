import { object, string, version } from '../core/validation';
import type { Command, Details, Submission, Decision } from './types';
import { validate } from './policy';
export function input(raw: Record<string, unknown>): Command { const r = object(raw, ['version', 'action', 'reason', 'details', 'submission', 'client_decision']), c: Command = { version: version(r.version ?? 0), action: string(r.action, 'action', ''), reason: string(r.reason, 'reason', ''), details: null, submission: null, client_decision: null }; const fields = { details: ['date', 'issuer_name', 'issuer_address', 'issuer_tax_reference', 'issuer_registration', 'issuer_email', 'bill_to', 'delivery_address'], submission: ['reference', 'date'], client_decision: ['status', 'reference', 'date', 'signatory', 'designation', 'file_id'] }; for (const key of ['details', 'submission', 'client_decision'] as const)
    if (r[key] != null) {
        const value = object(r[key], fields[key]), out: Record<string, string> = {};
        for (const k of fields[key])
            out[k] = string(value[k], k, '');
        if (key === 'details')
            c.details = out as Details;
        else if (key === 'submission')
            c.submission = out as Submission;
        else
            c.client_decision = out as Decision;
    } return validate(c); }
