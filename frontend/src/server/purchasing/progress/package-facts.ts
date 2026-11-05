import { isDeepStrictEqual } from 'node:util';
import type { DB } from '../../db';
import { AppError } from '../../errors';
import { isoDate } from '../core/validation';
import { indiaDate } from '../../personal-tasks/validation';
import * as sourcingRepo from '../sourcing/repository';
import * as sourcingContext from '../sourcing/context';
import { guard as sourcingGuard, sameSource as sourcingSame, released as sourcingReleased } from '../sourcing/policy';
import * as technicalRepo from '../technical/repository';
import { source as technicalSource } from '../technical/context';
import { sameSource as technicalSame, fileGuard, readyWithEvidence } from '../technical/policy';
import * as ordersRepo from '../orders/repository';
import { released as orderReleased } from '../orders/reader';
import { clientCleared } from '../orders/policy';
import * as acceptanceRepo from '../acceptance/repository';
import { source as acceptanceSource } from '../acceptance/context';
import { validate as acceptanceValidate } from '../acceptance/policy';
import * as advancesRepo from '../advances/repository';
import { source as advancesSource } from '../advances/context';
import { guard as advancesGuard, sameSource as advancesSame, financeReady } from '../advances/policy';
import { store as inspectionStore } from '../inspections/repository';
import { source as inspectionSource } from '../inspections/context';
import { guard as inspectionGuard, inspection, cleared } from '../inspections/policy';
import { store as qualityStore } from '../quality/repository';
import { source as qualitySource } from '../quality/context';
import { guard as qualityGuard, readiness, definitions } from '../quality/policy';
import { sameSource } from '../orders/policy';
import * as later from './later-records';
import type { Evidence } from './types';
const fact = (number: number, state = 'not_started', details = ''): Evidence => ({ number, state, details });
const blocked = (number: number, details: string) => fact(number, 'blocked', details);
function check(work: () => unknown) { try {
    work();
    return true;
}
catch (e) {
    if (!(e instanceof AppError))
        throw e;
    return false;
} }
export async function packageFacts(db: DB, p: string, id: string, includeClientPO = false) {
    const out: Evidence[] = [], sourcing = await sourcingRepo.get(db, p, id);
    let loiCurrent = false;
    if (sourcing?.is_active) {
        const current = await sourcingContext.source(db, p, id), s = { ...current, available: true, eligible: true, blockers: [] };
        let f = fact(5, 'in_progress');
        if (!check(() => sourcingGuard(s)) || s.package_id !== id || sourcing.project_id !== p || sourcing.package_id !== id || !sourcingSame(sourcing.source_snapshot, s))
            f = blocked(5, 'Sourcing evidence does not match the current item demand');
        else if (sourcing.decision_snapshot && !sourcingSame(sourcing.decision_snapshot.source, s))
            f = blocked(5, 'Selection evidence belongs to an obsolete demand revision');
        else if (!['draft', 'pending', 'approved', 'released'].includes(sourcing.state))
            f = blocked(5, 'Stored sourcing state is invalid');
        else if (sourcing.state !== 'draft' && !sourcing.decision_snapshot)
            f = blocked(5, 'Recorded selection has no decision snapshot');
        else if (sourcing.state === 'released') {
            let issued;
            try {
                issued = sourcingReleased(sourcing);
            }
            catch (e) {
                if (!(e instanceof AppError))
                    throw e;
            }
            if (!issued)
                f = blocked(5, 'Stored LOI release evidence is incomplete');
            else if (!isoDate(sourcing.loi_sent_date) || !sourcing.loi_channel || !sourcing.loi_dispatch_reference)
                f = blocked(5, 'Actual LOI dispatch evidence is missing');
            else
                f = fact(5, 'completed', issued.loi_reference);
        }
        out.push(f);
        loiCurrent = f.state === 'completed';
    }
    const technical = await technicalRepo.get(db, p, id), tf = [fact(6), fact(7), fact(8)];
    if (technical?.is_active) {
        const current = await technicalSource(db, p, id), files = await technicalRepo.files(db, technical.id);
        if (!loiCurrent || !technicalSame(technical.source_snapshot, current))
            for (const f of tf)
                Object.assign(f, blocked(f.number, 'Technical evidence does not match the current upstream LOI'));
        else {
            let required = 0, received = 0, any = false;
            for (const d of technical.documents) {
                if (!d.is_active)
                    continue;
                const valid = d.status === 'received' && check(() => fileGuard(files, d.file_id, technical.id, 'document'));
                any = any || valid;
                if (d.required) {
                    required++;
                    if (valid)
                        received++;
                }
            }
            const kit = required > 0 && required === received;
            if (kit)
                Object.assign(tf[0], fact(6, 'completed', 'Required technical documents received with current file evidence'));
            else if (any)
                Object.assign(tf[0], fact(6, 'in_progress', 'Required technical kit remains incomplete'));
            const submitted = kit && technical.submission?.revision === technical.revision && isDeepStrictEqual(technical.documents, technical.submission.documents) && check(() => fileGuard(files, technical.submission!.file_id, technical.id, 'transmittal'));
            if (submitted)
                Object.assign(tf[1], fact(7, 'completed', technical.submission!.reference));
            if (technical.state === 'revision_required')
                Object.assign(tf[1], blocked(7, technical.team_review?.remarks || 'Project team requires a revised technical submission'));
            if (readyWithEvidence(technical, files))
                Object.assign(tf[2], fact(8, 'completed', technical.approval!.reference));
            else if (technical.state === 'rejected')
                Object.assign(tf[2], blocked(8, technical.approval?.remarks || 'Client technical approval rejected'));
            else if (submitted || technical.team_review || technical.approval)
                Object.assign(tf[2], fact(8, 'in_progress', 'Client approval, signed QAP or condition closure remains pending'));
        }
    }
    out.push(...tf);
    const order = await ordersRepo.get(db, p, id);
    let orderFact = fact(9);
    if (order?.is_active) {
        if (order.project_id !== p || order.package_id !== id || order.source_snapshot.project_id !== p || order.source_snapshot.package_id !== id)
            orderFact = blocked(9, 'Recorded order does not match the reporting scope');
        else if (order.state === 'draft')
            orderFact = fact(9, 'in_progress', 'Purchase order draft awaits authorized release');
        else {
            try {
                const issued = await orderReleased(db, p, id);
                orderFact = fact(9, 'completed', issued.number);
            }
            catch (e) {
                if (!(e instanceof AppError))
                    throw e;
                orderFact = blocked(9, 'Recorded release evidence is incomplete');
            }
        }
    }
    out.push(orderFact);
    if (includeClientPO)
        out.push(fact(21, order ? order.state === 'released' || clientCleared(order) && loiCurrent && tf[2].state === 'completed' ? 'completed' : 'in_progress' : 'not_started', 'Client stamp/sign on the exact draft PO'));
    const acceptance = await acceptanceRepo.get(db, p, id);
    let af = fact(10);
    if (acceptance?.is_active) {
        const current = await acceptanceSource(db, p, id);
        if (!current.order || acceptance.project_id !== p || acceptance.package_id !== id || !isDeepStrictEqual(acceptance.order_snapshot, current.order))
            af = blocked(10, 'Vendor response does not match the current released order');
        else if (acceptance.response.status !== 'pending') {
            if (!check(() => acceptanceValidate({ version: acceptance.version, action: 'record', reason: 'Validate stored reporting evidence', response: acceptance.response })) || acceptance.response.accepted_date < acceptance.order_snapshot.date)
                af = blocked(10, 'Recorded vendor acknowledgement evidence is incomplete');
            else
                af = fact(10, 'completed', acceptance.response.status === 'accepted' ? acceptance.response.document_reference : 'Acknowledged with exceptions: ' + acceptance.response.remarks);
        }
    }
    out.push(af);
    const advance = await advancesRepo.get(db, p, id);
    if (advance?.is_active) {
        const src = await advancesSource(db, p, id), s = { ...src, available: true, blockers: [] };
        let f = fact(11);
        if (!check(() => advancesGuard(s)) || advance.project_id !== p || advance.package_id !== id || !advancesSame(advance.source_snapshot, s))
            f = blocked(11, 'Advance evidence does not match the current order, acceptance and supplier verification');
        else if (advance.state === 'rejected')
            f = blocked(11, 'Finance rejected this advance request');
        else if (advance.state !== 'draft') {
            if (!['requested', 'approved', 'disbursed', 'not_applicable'].includes(advance.state))
                f = blocked(11, 'Stored advance state is invalid');
            else if (!check(() => financeReady(advance, s, indiaDate(new Date()))))
                f = blocked(11, 'Submitted advance evidence or exact amount is incomplete');
            else if (advance.state !== 'requested' && (!advance.decision_id || !advance.reviewed_by_id || !advance.reviewed_at || advance.reviewed_by_id === advance.submitted_by_id))
                f = blocked(11, 'Actual independent Finance decision is missing');
            else if (advance.state === 'not_applicable' && advance.details.kind !== 'not_applicable' || ['approved', 'disbursed'].includes(advance.state) && advance.details.kind !== 'payment')
                f = blocked(11, 'Advance disposition does not match the Finance decision');
            else if (advance.state === 'disbursed' && (!advance.payment || !isoDate(advance.payment.disbursed_date) || advance.payment.disbursed_date < indiaDate(new Date(advance.reviewed_at!)) || advance.payment.disbursed_date > indiaDate(new Date())))
                f = blocked(11, 'Actual payment evidence is missing');
            else if (advance.state === 'disbursed' && !advance.payment?.voucher_ref.trim())
                f = blocked(11, 'Actual payment voucher is missing');
            else
                f = fact(11, advance.state === 'not_applicable' ? 'not_applicable' : 'completed', advance.state === 'not_applicable' ? advance.details.notes : advance.details.request_no);
        }
        out.push(f);
    }
    const ir = await inspectionStore.get(db, p, id);
    if (ir?.is_active) {
        const src = await inspectionSource(db, p, id), s = { ...src, available: true, blockers: [] };
        let f = fact(12);
        if (!check(() => inspectionGuard(s)) || ir.project_id !== p || ir.package_id !== id || !sameSource(ir.source_snapshot, s))
            f = blocked(12, 'Inspection evidence does not match the current PO, acceptance and approved technical kit');
        else if (ir.inspection.status === 'cleared')
            f = check(() => cleared(ir, s)) ? fact(12, 'completed', ir.inspection.findings) : blocked(12, 'Stored inspection clearance evidence is incomplete');
        else if (ir.inspection.status === 'rejected')
            f = blocked(12, ir.inspection.findings);
        else if (['call_received', 'scheduled'].includes(ir.inspection.status))
            f = check(() => inspection(ir.inspection, s, indiaDate(new Date(ir.updated_at)))) ? fact(12, 'in_progress', ir.inspection.call_notice_ref) : blocked(12, 'Stored inspection scheduling evidence is incomplete');
        else if (ir.inspection.status !== 'not_scheduled')
            f = blocked(12, 'Stored inspection state is invalid');
        out.push(f);
    }
    out.push(await later.reports(db, p, id));
    const q = await qualityStore.get(db, p, id);
    if (q?.is_active) {
        const src = await qualitySource(db, p, id), s = { ...src, available: true, blockers: [] };
        let f = fact(14);
        if (!check(() => qualityGuard(s)) || q.project_id !== p || q.package_id !== id || !sameSource(q.source_snapshot, s))
            f = blocked(14, 'Quality evidence does not match the current issued IR / IRN');
        else {
            const ready = readiness(q, s, indiaDate(new Date()));
            if (ready.ready)
                f = fact(14, 'completed', 'All required certificates are verified and valid');
            else {
                f.details = ready.blockers.join('; ');
                const codes = q.certificates.map(c => c.code), flags = q.certificates.filter(c => c.verified).length;
                if (q.version < 1 || q.revision < 1 || q.certificates.length !== definitions.length || new Set(codes).size !== codes.length || codes.some(code => !definitions.some(([c]) => c === code)) || flags > ready.verified_count)
                    f.state = 'blocked';
                else if (ready.verified_count > 0)
                    f.state = 'in_progress';
            }
        }
        out.push(f);
    }
    for (const read of [later.clearance, later.dispatch, later.receipts, later.grns, later.engineer, later.accounts])
        out.push(await read(db, p, id));
    return out;
}
