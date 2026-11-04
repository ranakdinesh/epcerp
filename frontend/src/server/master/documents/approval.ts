import "server-only";
import { randomUUID } from "node:crypto";
import type { DB } from "../../db";
import { userCapabilities, documentReviewCandidates } from "../../auth/access";
import { recordMasterEvent } from "../../events/journal";
import { query } from "../catalogue/database";
import { fieldError } from "../catalogue/validation";
import { AppError } from "../../errors";
import type { Document } from "./domain";
async function save(db: DB, d: Document, actor: string, state: string, comment: string, action: string) {
    const saved = (await query(db, "DocumentApprovalSave", { document_id: d.id, supplier_id: d.supplier_id, document_version: d.version, sha256: d.sha256, state, comment, actor })).rows[0];
    const names: Record<string, string> = { document_id: "DocumentID", supplier_id: "SupplierID", document_version: "DocumentVersion", sha256: "Sha256", state: "State", comment: "Comment", version: "Version", is_active: "IsActive", created_at: "CreatedAt", updated_at: "UpdatedAt", created_by_id: "CreatedByID", updated_by_id: "UpdatedByID" };
    const snapshot = Object.fromEntries(Object.entries(names).map(([key, name]) => [name, saved[key]])), id = randomUUID();
    await query(db, "DocumentApprovalJournal", { id, document_id: d.id, version: saved.version, action, snapshot, actor });
    await recordMasterEvent(db, "supplier_document_approval", "supplier_document_approval", d.id, saved.version, action, actor, id, saved.updated_at);
}
export async function submitted(db: DB, d: Document, actor: string) {
    const uploader = await userCapabilities(db, actor);
    if (!uploader?.active || !uploader.allows("settings.supplier_documents.write"))
        throw new AppError(403, "Insufficient permission");
    await submitFor(db, d, uploader, actor, 'uploaded');
}
async function submitFor(db: DB, d: Document, uploader: NonNullable<Awaited<ReturnType<typeof userCapabilities>>>, actor: string, action: string) {
    const automatic = uploader.allows("accounts.vendor_verification.auto_approve");
    await save(db, d, actor, automatic ? "accept" : "pending", automatic ? "Automatically approved by upload permission" : "Awaiting document review", action);
    if (automatic)
        return;
    let reviewer = (await db.query<{
        user_id: string;
    }>("SELECT user_id FROM supplier_review_assignments WHERE supplier_id=$1 AND is_active", [d.supplier_id])).rows[0]?.user_id;
    if (!reviewer) {
        const candidates = await documentReviewCandidates(db);
        if (candidates.length > 1)
            fieldError("file", "Choose the supplier's Finance reviewer before uploading");
        reviewer = candidates[0];
    }
    const user = reviewer ? await userCapabilities(db, reviewer) : null;
    if (!user?.active || !user.allows("accounts.vendor_verification.read") || !user.allows("accounts.vendor_verification.write"))
        fieldError("file", "Assign an active Finance reviewer before uploading for approval");
    await query(db, "DocumentTaskCreate", { id: randomUUID(), document_id: d.id, supplier_id: d.supplier_id, user_id: reviewer, kind: "review", comment: "Review uploaded document", actor });
}
export async function retired(db: DB, d: Document, actor: string, outcome: "archived" | "replaced") {
    for (const kind of ["review", "correction"])
        await query(db, "DocumentTasksComplete", { document_id: d.id, kind, outcome: kind === "correction" && outcome === "replaced" ? "resubmitted" : outcome, comment: "Document " + outcome, actor });
    await save(db, d, actor, "pending", "Document " + outcome, outcome);
}
export async function decided(db: DB, d: Document, actor: string, state: string, comment: string) {
    const previous = (await query(db, 'DocumentApprovalGet', { id: d.id })).rows[0];
    if (previous && previous.state === state && previous.document_version === d.version && previous.sha256 === d.sha256)
        return;
    await save(db, d, actor, state, comment, 'reviewed');
    await query(db, 'DocumentTasksComplete', { document_id: d.id, kind: 'review', outcome: state === 'reject' ? 'rejected' : 'approved', comment, actor });
    if (state === 'reject' && d.created_by_id)
        await query(db, 'DocumentTaskCreate', { id: randomUUID(), document_id: d.id, supplier_id: d.supplier_id, user_id: d.created_by_id, kind: 'correction', comment, actor });
    else
        await query(db, 'DocumentTasksComplete', { document_id: d.id, kind: 'correction', outcome: 'approved', comment, actor });
}
export async function reconciled(db: DB, d: Document, actor: string) {
    if ((await query(db, 'DocumentApprovalGet', { id: d.id })).rows[0])
        return;
    const uploader = d.created_by_id ? await userCapabilities(db, d.created_by_id) : null;
    if (!uploader?.active || !uploader.allows('settings.supplier_documents.write'))
        throw new AppError(403, 'Insufficient permission');
    // Reuse submit policy with the original uploader and the operator as audit actor.
    await submitFor(db, d, uploader, actor, 'reconciled');
}
