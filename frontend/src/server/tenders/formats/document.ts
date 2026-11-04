import 'server-only';
import { zipSync, strToU8 } from 'fflate';
import template from './template-package.json';
import { createHmac } from 'node:crypto';
import { pool } from '../../db';
import { AppError } from '../../errors';
import type { Format } from './service';
const xml = (s: string) => s.replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&apos;' }[c]!)).replace(/[\x00-\x08\x0b\x0c\x0e-\x1f]/g, '');
const paragraph = (s: string) => `<w:p><w:r><w:rPr><w:sz w:val="20"/></w:rPr><w:t xml:space="preserve">${xml(s)}</w:t></w:r></w:p>`;
export function word(format: Format) {
    const cell = (s: string, width: string) => `<w:tc><w:tcPr><w:tcW w:w="${width}" w:type="dxa"/></w:tcPr>${s.split('\n').map(paragraph).join('')}</w:tc>`, rows = format.questions.map((q, i) => `<w:tr><w:trPr><w:cantSplit/></w:trPr>${cell(q.printed_number || String(i + 1), '600')}${cell(q.section.replaceAll('_', ' ') + '\n' + q.question, '4000')}${cell('', '4400')}</w:tr>`).join(''), document = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>${paragraph(format.name)}<w:tbl><w:tblPr><w:tblW w:w="9000" w:type="dxa"/><w:tblBorders>${['top', 'left', 'bottom', 'right', 'insideH', 'insideV'].map(side => `<w:${side} w:val="single" w:sz="4"/>`).join('')}</w:tblBorders></w:tblPr><w:tblGrid><w:gridCol w:w="600"/><w:gridCol w:w="4000"/><w:gridCol w:w="4400"/></w:tblGrid><w:tr><w:trPr><w:tblHeader/></w:trPr>${cell('No.', '600')}${cell('Question', '4000')}${cell('Answer', '4400')}</w:tr>${rows}</w:tbl><w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="900" w:right="900" w:bottom="900" w:left="900"/></w:sectPr></w:body></w:document>`;
    return Buffer.from(zipSync({ ...Object.fromEntries(Object.entries(template.parts).map(([name, bytes]) => [name, Buffer.from(bytes, 'base64')])), 'word/document.xml': strToU8(document) }, { level: 6 }));
}
export async function pdf(bytes: Buffer, signal?: AbortSignal) { const endpoint = new URL(process.env.EPCERP_TENDER_FORMAT_RENDER_URL ?? 'http://document-worker:8082'); if (!['http:', 'https:'].includes(endpoint.protocol) || !['document-worker', 'epcerp-tender-qa-worker', 'localhost', '127.0.0.1'].includes(endpoint.hostname) || endpoint.username || endpoint.password || endpoint.pathname !== '/')
    throw new AppError(503, 'Invalid private format renderer'); const key = process.env.EPCERP_DOCUMENT_READING_KEY ?? ''; if (!/^[a-f\d]{64}$/i.test(key))
    throw new AppError(503, 'Format renderer is not configured'); const db = await pool().connect(); let acquired = false; try {
    acquired = (await db.query('SELECT pg_try_advisory_lock(735841920001)AS acquired')).rows[0].acquired;
    if (!acquired)
        throw new AppError(503, 'Document processor is busy; try the preview again shortly');
    const at = String(Date.now()), signature = createHmac('sha256', Buffer.from(key, 'hex')).update('epcerp-format-preview-v1\n' + at + '\n').update(bytes).digest('hex'), response = await fetch(endpoint, { method: 'POST', headers: { 'Content-Type': 'application/vnd.openxmlformats-officedocument.wordprocessingml.document', 'X-Render-Time': at, 'X-Render-Signature': signature }, body: new Uint8Array(bytes), redirect: 'error', signal: signal ? AbortSignal.any([signal, AbortSignal.timeout(50000)]) : AbortSignal.timeout(50000) });
    if (!response.ok)
        throw new AppError(503, 'Format preview is unavailable; download the Word file or retry');
    const reader = response.body!.getReader(), parts: Uint8Array[] = [];
    let size = 0;
    try {
        while (true) {
            const { value, done } = await reader.read();
            if (done)
                break;
            size += value.length;
            if (size > 16 * 1048576) {
                await reader.cancel();
                throw new AppError(503, 'Format preview exceeds the size limit');
            }
            parts.push(value);
        }
    }
    finally {
        reader.releaseLock();
    }
    const data = Buffer.concat(parts);
    if (data.subarray(0, 5).toString() !== '%PDF-')
        throw new AppError(503, 'Invalid format preview');
    return data;
}
finally {
    let destroy = false;
    if (acquired)
        try {
            await db.query('SELECT pg_advisory_unlock(735841920001)');
        }
        catch {
            destroy = true;
        }
    db.release(destroy);
} }
