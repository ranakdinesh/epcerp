import 'server-only';
import { randomUUID } from 'node:crypto';
import { transaction, type DB } from '../../db';
import { assignmentIdentities } from '../../auth/access';
import type { Actor } from '../core/access';
import { PurchaseError, invalid, object, string, version } from '../core/validation';
import { now } from '../core/time';
import { actor, actions } from './access';
import { id, command } from './validation';
import { source } from './source';
import { missing, evaluate, redact, pendingFinanceBudget, snapshot, proposalStaleReason, decide, financeBudget } from './policy';
import { shortlist, selectProposal, validateDraft } from './shortlist';
import type { ApprovalRecord, Attempt, Draft } from './types';
import * as repo from './repository';
import { decide as financialDecision } from './finance';
import { notify } from './work';
export const forbidden = () => new PurchaseError(403, 'Quotation approval permission or project scope denied');
export async function workspaceBound(db: DB, a: Actor, p: string, b: string) {
    const { caps, project } = await actions(db, a, p);
    if (!caps.read)
        throw forbidden();
    const s = await source(db, p, b), r = await repo.get(db, p, b);
    if (r && !r.is_active)
        throw missing();
    const evaluated = evaluate(r?.attempts ?? [], s), history = await repo.history(db, p, b);
    if (!caps.review_read)
        for (const h of history) {
            h.after.attempts = redact(h.after.attempts);
            if (h.before)
                h.before.attempts = redact(h.before.attempts);
        }
    return { drafts: await repo.drafts(db, p, b), project_name: project.name, project_number: project.project_number, version: r?.version ?? 0, attempts: caps.review_read ? evaluated.attempts.map(a => a.state === 'pending_accounts' ? pendingFinanceBudget(a, s) : a) : redact(evaluated.attempts), vendor_statuses: s.verifications, ready: evaluated.ready, can_submit: caps.submit && s.available, can_review_read: caps.review_read, can_review: caps.review, can_read_budget: caps.budget_read, can_update_budget: caps.budget_write && s.available, can_verify: caps.verify, currency: s.currency, history };
}
export async function workspace(token: string, p: string, b: string) { return transaction(async (db) => { const a = await actor(db, token); id(p); id(b); return workspaceBound(db, a, p, b); }); }
export async function execute(token: string, p: string, b: string, action: string, raw: Record<string, unknown>) {
    return transaction(async (db) => {
        const a = await actor(db, token);
        id(p);
        id(b);
        const c = command(raw, action), { caps, project } = await actions(db, a, p);
        if (!caps.read || action === 'submit' && !caps.submit || action === 'review' && (!caps.review_read || !caps.review))
            throw forbidden();
        const s = await source(db, p, b);
        if (!s.available)
            invalid('project', 'Project purchasing is inactive, held or closed');
        const existing = await repo.get(db, p, b);
        if (existing && !existing.is_active)
            throw missing();
        const before = existing ? structuredClone(existing) : null, r: ApprovalRecord = existing ?? { project_id: p, bo_id: b, version: 0, attempts: [], is_active: true, created_at: '', updated_at: '', created_by_id: null, updated_by_id: null };
        if (c.version !== r.version)
            throw repo.stale();
        const { latest } = evaluate(r.attempts, s);
        let changed: Attempt;
        const recipients: string[] = [];
        if (action === 'submit') {
            const chosen = s.quotes.find(q => q.key === c.quote_key || c.proposals.length > 0 && q.key === c.proposals[0].quote_key);
            if (!chosen)
                throw missing();
            const prior = latest.get(chosen.group_id);
            if (prior && ['pending_accounts', 'approved'].includes(prior.effective_state ?? ''))
                invalid('quote_key', 'This group already has a pending or approved quote; Accounts must decide first');
            if (r.attempts.length >= 1000)
                invalid('quote_key', 'This BO has reached the approval history limit; contact an administrator');
            changed = c.proposals.length ? shortlist(c.proposals, s) : snapshot(chosen, s);
            changed.id = randomUUID();
            changed.state = 'pending_accounts';
            changed.submitted_by_id = a.id;
            changed.submitted_at = now();
            changed.reason = c.reason;
            if (changed.proposals?.length) {
                for (const candidate of await assignmentIdentities(db, [])) {
                    if (candidate.id !== a.id && candidate.allows('accounts.quotation_approvals.write') && (await actions(db, candidate, p)).caps.review)
                        recipients.push(candidate.id);
                }
                if (!recipients.length)
                    invalid('finance', 'Assign another authorized Finance reviewer before submitting');
            }
            await repo.completeDraft(db, p, b, changed.group_id, a.id);
            r.attempts.push(changed);
        }
        else {
            const index = r.attempts.findIndex(v => v.id === c.attempt_id);
            if (index < 0)
                throw missing();
            const attempt = r.attempts[index];
            if (latest.get(attempt.group_id)?.id !== attempt.id)
                throw missing();
            const reason = proposalStaleReason(attempt, s);
            if (reason)
                invalid('attempt_id', reason);
            if (attempt.state !== 'pending_accounts')
                invalid('attempt_id', 'This quotation has already been decided');
            if (attempt.submitted_by_id === a.id)
                throw new PurchaseError(403, 'Another Accounts user must review this submission');
            changed = decide(selectProposal(attempt, c.selected_quote_key), c.decisions);
            if (attempt.proposals?.length) {
                if (changed.state === 'rejected' && !c.return_action)
                    invalid('return_action', 'Choose negotiate or change vendor for a rejected group');
                if (changed.state === 'approved' && c.return_action)
                    invalid('return_action', 'Approved groups cannot have a return instruction');
                if (c.decisions.some(d => d.decision !== c.decisions[0].decision))
                    invalid('decisions', 'Approve one complete proposal or reject the whole group');
            }
            if (changed.state === 'approved') {
                changed = financeBudget(changed, s);
                if (changed.budget_issue)
                    invalid('budget', changed.budget_issue);
            }
            if (c.return_action)
                changed.return_action = c.return_action;
            else
                delete changed.return_action;
            changed.review_comment = c.reason;
            const decision = await financialDecision(db, a, r, changed, c.reason);
            changed.reviewed_at = now();
            changed.reviewed_by_id = a.id;
            changed.decision_id = decision;
            r.attempts[index] = changed;
            if (changed.proposals?.length)
                recipients.push(changed.submitted_by_id);
        }
        r.version++;
        const saved = await repo.save(db, r, a.id), e: repo.Event = { id: randomUUID(), project_id: p, bo_id: b, action: action === 'submit' ? 'submit' : 'accounts_decision', reason: c.reason, before, after: saved, created_by_id: a.id, created_at: now() };
        await repo.event(db, e);
        if (changed.proposals?.length)
            await notify(db, e, changed, recipients, project.project_number + ' · ' + project.name);
        return workspaceBound(db, a, p, b);
    });
}
export async function saveDraft(token: string, p: string, b: string, raw: Record<string, unknown>) { return transaction(async (db) => { const a = await actor(db, token); id(p); id(b); const { caps } = await actions(db, a, p); if (!caps.read || !caps.submit)
    throw forbidden(); const s = await source(db, p, b); if (!s.available)
    invalid('project', 'Project purchasing is inactive, held or closed'); const r = await repo.get(db, p, b); if (r && !r.is_active)
    throw missing(); const input = object(raw, ['group_id', 'version', 'proposals', 'comment']); const d = validateDraft({ project_id: p, bo_id: b, group_id: id(input.group_id), version: version(input.version ?? 0), proposals: input.proposals as Draft['proposals'], comment: string(input.comment, 'comment', ''), is_active: true, updated_by: a.id, updated_at: now() }, s, r?.attempts ?? []); await repo.saveDraft(db, d, a.id); return workspaceBound(db, a, p, b); }); }
export async function approved(db: DB, p: string, b: string, all = true) { const s = await source(db, p, b), r = await repo.get(db, p, b); if (!r?.is_active)
    throw forbidden(); const { latest, ready } = evaluate(r.attempts, s); if (all && !ready)
    invalid('approval', "Accounts must approve every current group's quotation before the next step"); const out = s.groups.map(g => latest.get(g.id)).filter((a): a is Attempt => !!a && a.effective_state === 'approved'); if (!all && !out.length)
    invalid('approval', "Finance must approve this group's quotation before sourcing"); return redact(out); }
export async function commercial(db: DB, p: string, b: string, all = true) { const rows = await approved(db, p, b, all), s = await source(db, p, b); return rows.flatMap(a => s.quotes.filter(q => q.key === a.quote_key && q.vendor_id === a.vendor_id).flatMap(q => q.lines.filter(l => !all || a.lines.some(v => v.line_id === l.line_id && v.item.id === l.item.id)).map(l => ({ attempt_id: a.id, line_id: l.line_id, item_id: l.item.id, vendor_id: a.vendor_id, quote_key: a.quote_key, quantity: l.quantity, total: l.quoted, price: l.price })))); }
// Project-wide clearance is required before technical work, including all active BOs.
export async function projectReady(db: DB, p: string) { const bos = await import('../bo/repository'); let found = false; for (const bo of await bos.list(db, p)) {
    if (!bo.is_active)
        continue;
    found = true;
    const s = await source(db, p, bo.id), r = await repo.get(db, p, bo.id);
    if (!r || !evaluate(r.attempts, s).ready)
        return false;
} return found; }
