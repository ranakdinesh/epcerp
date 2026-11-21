import 'server-only';
import { createHmac } from 'node:crypto';
import { AppError } from '../errors';
export async function processor(path: string, data: Buffer, signal?: AbortSignal, name?: string) {
    const url = new URL(process.env.EPCERP_TENDER_FORMAT_RENDER_URL ?? 'http://document-processor:8082');
    if (url.protocol !== 'http:' || !['document-processor', 'document-worker', 'localhost', '127.0.0.1'].includes(url.hostname) || url.username || url.password)
        throw new AppError(503, 'Invalid private processor');
    const key = process.env.EPCERP_DOCUMENT_READING_KEY ?? '';
    if (!/^[a-f\d]{64}$/i.test(key))
        throw new AppError(503, 'Private processor key unavailable');
    const at = String(Date.now()), label = path === '/verify' ? 'epcerp-package-verify-v1' : 'epcerp-office-pdf-v1', signature = createHmac('sha256', Buffer.from(key, 'hex')).update(label + '\n' + at + '\n').update(data).digest('hex'), headers: Record<string, string> = { 'Content-Type': 'application/octet-stream', 'X-Render-Time': at, 'X-Render-Signature': signature };
    if (name) {
        const encoded = Buffer.from(name).toString('base64url');
        headers['X-Document-Name'] = encoded;
        headers['X-Document-Signature'] = createHmac('sha256', Buffer.from(key, 'hex')).update(at + '\n' + encoded + '\n').update(data).digest('hex');
        if (path === '/office-pdf')
            headers['X-Office-Kind'] = name.toLowerCase().endsWith('.xlsx') ? 'xlsx' : 'docx';
    }
    const response = await fetch(new URL(path, url), {
        method: 'POST', body: new Uint8Array(data), headers, signal: AbortSignal.any([AbortSignal.timeout(15 * 60 * 1000), ...(signal ? [signal] : [])]), redirect: 'error'
    });
    if (!response.ok)
        throw new AppError(503, 'Private document processor unavailable');
    return response;
}
export async function officePDF(data: Buffer, name: string) {
    const r = await processor('/office-pdf', data, undefined, name), parts: Uint8Array[] = [];
    let size = 0;
    for await (const part of r.body! as unknown as AsyncIterable<Uint8Array>) {
        size += part.length;
        if (size > 32 * 1048576)
            throw new AppError(413, 'Converted document exceeds bounds');
        parts.push(part);
    }
    const pdf = Buffer.concat(parts);
    if (pdf.subarray(0, 5).toString() !== '%PDF-')
        throw new AppError(503, 'Invalid converted PDF');
    return pdf;
}
