import "server-only";
import { transaction } from "../../db";
import { AppError } from "../../errors";
import { requireAnyPermission, userCapabilities, financeReviewers } from "../../auth/access";
import { fieldError, goObject } from "../catalogue/validation";
import { DocumentRepository } from "../documents/repository";
import { open, readStoredObject, inspect } from "../documents/files";
import { retired, reconciled, decided } from "../documents/approval";
import { evaluate, fingerprint, parseReview, validateReview, parseAssignment, validateAssignment, staleReview } from "./domain";
import { VerificationRepository, assignmentView } from "./repository";
const finance = 'accounts.vendor_verification.';
async function authority(db: Parameters<typeof requireAnyPermission>[0], token: string, permissions: string[], write = false) { const actor = await requireAnyPermission(db, token, [permissions[0]], write); if (permissions.some(p => !actor.allows(p)))
    throw new AppError(403, 'Insufficient permission'); return actor; }
export function status(token: string, supplier: string) { return transaction(async (db) => { const actor = await requireAnyPermission(db, token, [finance + 'read', 'settings.suppliers.read']), repo = new VerificationRepository(db), snapshot = await repo.snapshot(supplier), result = evaluate(snapshot, await repo.review(supplier)); if (!actor.allows(finance + 'read'))
    delete result.review; return result; }); }
export function review(token: string, supplier: string, value: unknown) {
    return transaction(async (db) => {
        const actor = await authority(db, token, [finance + 'read', finance + 'write'], true), input = parseReview(value), repo = new VerificationRepository(db), snapshot = await repo.snapshot(supplier), previous = await repo.review(supplier), hash = fingerprint(snapshot);
        if (input.version !== (previous?.version ?? 0) || input.fingerprint !== hash)
            throw staleReview();
        validateReview(snapshot, input);
        const saved = await repo.saveReview(supplier, input.version + 1, hash, input.decisions, input.reason, actor.id), documents = new DocumentRepository(db);
        for (const decision of input.decisions) {
            const d = await documents.get(supplier, decision.document_id);
            if (!d?.is_active)
                throw new AppError(404, 'Supplier document not found');
            await decided(db, d, actor.id, decision.decision, decision.comment);
        }
        return evaluate(await repo.snapshot(supplier), saved, hash);
    });
}
export function getAssignment(token: string, supplier: string) { return transaction(async (db) => { await authority(db, token, [finance + 'read']); const repo = new VerificationRepository(db); await repo.lockSupplier(supplier); return { assignment: assignmentView(await repo.assignment(supplier)) }; }); }
export function assignmentOptions(token: string) { return transaction(async (db) => { await authority(db, token, [finance + 'read', finance + 'assign']); return financeReviewers(db); }); }
export function saveAssignment(token: string, supplier: string, value: unknown) {
    return transaction(async (db) => {
        const actor = await authority(db, token, [finance + 'read', finance + 'assign'], true), input = parseAssignment(value);
        validateAssignment(input);
        const repo = new VerificationRepository(db);
        await repo.lockSupplier(supplier);
        const current = await repo.assignment(supplier);
        if (input.version !== (current?.version ?? 0))
            throw staleReview();
        if (!input.is_active && (!current || current.user_id !== input.user_id))
            fieldError('user_id', 'Revoke the current reviewer assignment');
        if (input.is_active) {
            const user = await userCapabilities(db, input.user_id);
            if (!user || !['view', 'read', 'write'].every(p => user.allows(finance + p)))
                fieldError('user_id', 'Choose an active Finance reviewer');
        }
        return repo.saveAssignment(supplier, input, current, actor.id);
    });
}
export function reconcileApprovals(token: string, supplier: string, value: unknown) {
    if (Object.keys(goObject(value, [])).length)
        throw new AppError(422, 'Send an empty JSON object');
    return transaction(async (db) => {
        const actor = await authority(db, token, ['settings.supplier_documents.write', finance + 'auto_approve', 'settings.supplier_documents.import'], true), repo = new DocumentRepository(db);
        await repo.supplier(supplier);
        const docs = await repo.list(supplier), latest = new Map<string, string>(), result = { processed: 0, unreadable: 0, replaced: 0 };
        for (const d of docs)
            if (d.is_active && d.doc_type !== 'other')
                latest.set(d.doc_type, d.id);
        for (const d of docs) {
            if (!d.is_active)
                continue;
            if (d.doc_type !== 'other' && latest.get(d.doc_type) !== d.id) {
                const saved = await repo.archive(supplier, d.id, d.version, actor.id);
                await repo.changed(saved, actor.id, 'archived', 'Replaced by the latest uploaded document');
                await retired(db, saved, actor.id, 'replaced');
                result.replaced++;
                continue;
            }
            const stored = (await repo.get(supplier, d.id))!, encrypted = stored.object_key ? await readStoredObject(stored.object_bucket!, stored.object_key) : stored.encrypted_content, plain = open(encrypted ?? Buffer.alloc(0), stored.sha256);
            try {
                await inspect(plain);
            }
            catch {
                result.unreadable++;
                continue;
            }
            await reconciled(db, stored, actor.id);
            result.processed++;
        }
        return result;
    });
}
