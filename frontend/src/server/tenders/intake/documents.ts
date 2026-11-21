import 'server-only';
import { randomUUID } from 'node:crypto';
import { transaction, type DB } from '../../db';
import { AppError } from '../../errors';
import { digest, decryptBytes } from '../tsc/crypto';
import { id, object, integer } from '../tsc/validation';
import { access, admit } from './access';
import { ownedDraft } from './drafts';

async function member(db: DB, draft: string, document: string) {
    const row = (await db.query(`SELECT m.* FROM document_package_members m JOIN document_package_receipts r ON r.id=m.receipt_id
      WHERE m.id=$2 AND r.draft_id=$1 AND r.state IN('ready','consumed') AND m.validation_claim_id=r.claim_id`, [draft, id(document)])).rows[0];
    if (!row) throw new AppError(404, 'Verified document not found');
    return row;
}

export async function download(token: string, draft: string, document: string) {
    await admit(token, 'draft', 'read');
    return transaction(async db => {
        const actor = await access(db, token, 'draft');
        await ownedDraft(db, draft, actor.id);
        const row = await member(db, draft, document);
        if ((await db.query('SELECT 1 FROM tender.draft_document_removals WHERE draft_id=$1 AND member_id=$2', [draft, document])).rowCount)
            throw new AppError(410, 'This document was deleted from the tender');
        if (digest(row.reader_payload) !== row.reader_payload_sha256) throw Error('Retained document integrity failed');
        const bytes = decryptBytes(row.reader_payload, row.sha256);
        if (bytes.length !== Number(row.byte_size)) throw Error('Retained document integrity failed');
        return { bytes, filename: row.filename as string };
    });
}

export async function remove(token: string, draft: string, document: string, value: unknown) {
    await admit(token, 'draft', 'remove_document', true);
    const version = integer(object(value, ['expected_version']).expected_version, 1);
    await transaction(async db => {
        const actor = await access(db, token, 'draft', true), d = await ownedDraft(db, draft, actor.id);
        const row = await member(db, draft, document);
        // Replay is safe even after the draft version has advanced.
        if ((await db.query('SELECT 1 FROM tender.draft_document_removals WHERE draft_id=$1 AND member_id=$2', [draft, document])).rowCount) return;
        if (Number(d.version) !== version) throw new AppError(409, 'Tender documents changed; refresh before deleting');
        if ((await db.query("SELECT 1 FROM tender.tsc_workspace_tasks WHERE draft_id=$1 AND state IN('queued','processing') LIMIT 1", [draft])).rowCount)
            throw new AppError(409, 'Wait for the current TSC request to finish before deleting documents');
        await db.query('INSERT INTO tender.draft_document_removals(draft_id,member_id,created_by_id)VALUES($1,$2,$3)', [draft, document, actor.id]);
        await db.query('UPDATE tender.document_drafts SET version=version+1,updated_at=clock_timestamp(),updated_by_id=$2 WHERE id=$1', [draft, actor.id]);
        await db.query("INSERT INTO tender.draft_events(id,draft_id,action,receipt_id,created_by_id)VALUES($1,$2,'source_document_removed',$3,$4)", [randomUUID(), draft, row.receipt_id, actor.id]);
    });
}
