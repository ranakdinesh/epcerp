import 'server-only';
import { transaction } from '../../db';
import { seal, open } from '../../master/documents/files';
import { inspectQuotation } from '../core/evidence-inspector';
import { invalid, uuid, PurchaseError } from '../core/validation';
import { access } from './service';
import { source } from './source';
import * as repo from './repository';
export async function upload(token: string, target: string, bo: string, document: string, name: string, mime: string, data: Buffer) {
    if (!data.length || data.length > 10 * 1024 * 1024)
        invalid('file', 'Upload nonempty quotation evidence of at most 10 MiB');
    await inspectQuotation(name, mime, data);
    return transaction(async (db) => { const { a, p } = await access(db, token, target, ['projects.group_quotations.write', 'projects.group_quotations.read'], true); bo = uuid(bo); document = uuid(document); const s = await source(db, p.id, bo); if (!s.available || !s.documents.some(d => d.id === document && d.data.sent_date !== null && d.data.is_current && d.vendor_available))
        invalid('document_id', 'Upload only for a current dispatched indent while purchasing is available'); const encrypted = seal(data), saved = await repo.insertAttachment(db, document, name, mime, data.length, encrypted.sha256, encrypted.encrypted, a.id); await repo.attachmentEvent(db, saved, 'uploaded', a.id); return saved; });
}
export async function download(token: string, target: string, bo: string, document: string, id: string) { return transaction(async (db) => { const { a, p } = await access(db, token, target, ['projects.group_quotations.read'], false); bo = uuid(bo); document = uuid(document); id = uuid(id); const s = await source(db, p.id, bo); if (!s.documents.some(d => d.id === document))
    throw new PurchaseError(404, 'Quotation or evidence not found in this project BO'); const file = await repo.attachment(db, document, id); if (!file)
    throw new PurchaseError(404, 'Quotation or evidence not found in this project BO'); const data = open(file.encrypted, file.metadata.sha256); await repo.attachmentEvent(db, file.metadata, 'downloaded', a.id); return { file: file.metadata, data }; }); }
