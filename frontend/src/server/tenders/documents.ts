import 'server-only';
import type { DB } from '../db';
import { AppError } from '../errors';
import { requireAnyPermission, userCapabilities } from '../auth/access';
import { digest, decryptBytes } from './tsc/crypto';
import type { Source, Original, Snapshot } from './tsc/types';
export async function authorizeTender(db: DB, token: string, id: string, action: 'read' | 'write' | 'export' | 'originals.write' = 'read') {
    const required = ['tenders.register.view', 'tenders.register.read', 'tenders.processing.read', 'tenders.tsc.read', ...(action === 'write' || action === 'originals.write' ? ['tenders.register.write', 'tenders.processing.write', 'tenders.tsc.write'] : []), ...(action === 'read' ? [] : ['tenders.tsc.' + action])];
    const actor = await requireAnyPermission(db, token, [required[0]], true);
    if (required.some(p => !actor.allows(p)))
        throw new AppError(403, 'Tender checklist permission required');
    const d = (await db.query('SELECT * FROM tender.document_drafts WHERE id=$1 AND owner_id=$2 AND is_active FOR UPDATE', [id, actor.id])).rows[0];
    if (!d)
        throw new AppError(404, 'Tender draft not found');
    return { actor, draft: { id: d.id, name: d.name, version: Number(d.version) } };
}
export async function authorizeWorker(db: DB, actor: string, draft: string) { const u = await userCapabilities(db, actor); if (!u?.active || !u.ready || ['tenders.register.view', 'tenders.register.read', 'tenders.register.write', 'tenders.processing.read', 'tenders.processing.write', 'tenders.tsc.read', 'tenders.tsc.write', 'tenders.tsc.originals.write'].some(p => !u.allows(p)))
    throw new AppError(403, 'Tender checklist permission required'); const d = (await db.query('SELECT id,name,version FROM tender.document_drafts WHERE id=$1 AND owner_id=$2 AND is_active FOR UPDATE', [draft, actor])).rows[0]; if (!d)
    throw new AppError(404, 'Tender draft not found'); return d; }
export const referenceDocument = (path: string) => /tender selection|tsc format|completed tsc/.test(path.toLowerCase().replace(/[_-]/g, ' '));
export async function tenderSources(db: DB, draft: string, owner: string): Promise<Source[]> {
    const r = await db.query(`SELECT m.id,m.receipt_id,m.sha256,m.path,m.kind,m.byte_size,m.pages,r.claim_id FROM public.document_package_members m JOIN public.document_package_receipts r ON r.id=m.receipt_id
 WHERE r.draft_id=$1 AND r.owner_id=$2 AND r.state IN('ready','consumed') AND m.validation_claim_id=r.claim_id ORDER BY r.created_at,r.id,m.normalized_path,m.id`, [draft, owner]);
    return r.rows.map(s => ({ ...s, byte_size: Number(s.byte_size) }));
}
export function sourceFingerprint(sources: Source[]) { return digest(JSON.stringify(sources.map(s => ({ id: s.id, receipt_id: s.receipt_id, sha256: s.sha256, claim_id: s.claim_id })).sort((a, b) => a.id.localeCompare(b.id)))); }
export async function originalDocuments(db: DB, snapshot: Snapshot, actor: string): Promise<Original[]> {
    const all = await tenderSources(db, snapshot.draft_id, actor), found = new Map(all.map(s => [s.id, s])), result: Original[] = [];
    for (const s of snapshot.sources) {
        const current = found.get(s.id);
        if (!current || current.sha256 !== s.sha256 || current.claim_id !== s.claim_id)
            throw new AppError(409, 'Tender source documents changed');
        const row = (await db.query('SELECT reader_payload,reader_payload_sha256 FROM public.document_package_members WHERE id=$1', [s.id])).rows[0];
        if (!row || digest(row.reader_payload) !== row.reader_payload_sha256)
            throw Error('Retained source integrity failed');
        const data = decryptBytes(row.reader_payload, s.sha256);
        if (data.length !== s.byte_size)
            throw Error('Retained source integrity failed');
        if(s.kind!=='pdf'){const {officePDF}=await import('../platform/processor');const pdf=await officePDF(data,s.path);result.push({...s,data:pdf,kind:'pdf',byte_size:pdf.length});}else{if(data.subarray(0,5).toString()!=='%PDF-')throw Error('Invalid source PDF');result.push({ ...s, data });}
    }
    return result;
}
