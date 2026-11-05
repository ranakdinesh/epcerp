import type { DB } from '../../db';
import { requireAnyPermission } from '../../auth/access';
import { AppError } from '../../errors';
import type { Actor } from '../core/access';
import { inspect } from '../workflow/reader';
import { stageAccess } from '../core/stage-access';
import { released } from '../orders/reader';
import { acceptedFact } from '../acceptance/service';
import { clearance } from '../technical/service';
import { invalid } from '../core/validation';
import type { Source } from './types';
export const actor = (db: DB, token: string) => requireAnyPermission(db, token, ['projects.purchase_inspection.read'], true);
export const access = (db: DB, a: Actor, p: string, id: string) => stageAccess(db, a, p, id, 'projects.purchase_inspection', 12);
const blank = (p: string, id: string): Source => ({ project_id: p, package_id: id, order_id: '', order_version: 0, number: '', date: '', supplier_id: '', supplier_name: '', description: '', quantity: '', uom: '', acceptance_id: '', acceptance_version: 0, technical_id: '', technical_version: 0, technical_revision: 0, qap_reference: '', qap_date: '', qap_file_id: '', signed_approval_file_id: '', available: false, blockers: [] });
const block = (s: Source, message: string) => { s.available = false; s.blockers.push(message); return s; };
const failure = (s: Source, e: unknown) => { if (e instanceof AppError && [422, 409, 404].includes(e.status))
    return block(s, e.message); throw e; };
export async function source(db: DB, p: string, id: string): Promise<Source> { const { record, current } = await inspect(db, p, id), out: Source = { ...blank(p, id), available: current.project_available && current.eligible, blockers: [...current.blockers] }; for (const step of record.steps)
    if (step.number === 12 && step.state === 'blocked')
        out.blockers.push('Material inspection has an unresolved blocker'); let po; try {
    po = await released(db, p, id);
}
catch (e) {
    return failure(out, e);
} Object.assign(out, { order_id: po.id, order_version: po.version, number: po.number, date: po.details.date, supplier_id: po.source_snapshot.supplier.id, supplier_name: po.source_snapshot.supplier.name, description: po.source_snapshot.description, quantity: po.source_snapshot.quantity, uom: po.source_snapshot.uom }); let acceptance; try {
    acceptance = await acceptedFact(db, p, id);
}
catch (e) {
    return failure(out, e);
} if (acceptance.order_snapshot.id !== po.id || acceptance.order_snapshot.version !== po.version)
    return block(out, 'Record current unconditional vendor acceptance for this PO'); out.acceptance_id = acceptance.id; out.acceptance_version = acceptance.version; let approval; try {
    approval = await clearance(db, p, id);
}
catch (e) {
    return failure(out, e);
} if (approval.project_id !== p || approval.package_id !== id)
    invalid('source', 'Technical evidence belongs to another inspection package'); Object.assign(out, { technical_id: approval.case_id, technical_version: approval.version, technical_revision: approval.revision, qap_reference: approval.approval.reference, qap_date: approval.approval.date, signed_approval_file_id: approval.approval.file_id }); for (const doc of approval.documents)
    if (doc.is_active && doc.kind === 'qap' && doc.status === 'received') {
        out.qap_file_id = doc.file_id;
        break;
    } if (!out.qap_file_id)
    return block(out, 'Retain the actual approved QAP document before recording inspection'); return out; }
