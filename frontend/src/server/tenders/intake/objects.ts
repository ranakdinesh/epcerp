import 'server-only';
import { constants } from 'node:fs';
import { lstat, open, statfs, unlink } from 'node:fs/promises';
import { createHash, createCipheriv, randomBytes } from 'node:crypto';
import { join, resolve } from 'node:path';
import { AppError } from '../../errors';
import { id } from '../tsc/validation';
const chunkSize = 1 << 20, magic = Buffer.from('EPCPKG01');
async function path(key: string) { id(key); const root = resolve(process.env.EPCERP_DOCUMENT_PACKAGE_ROOT ?? ''); if (!process.env.EPCERP_DOCUMENT_PACKAGE_ROOT)
    throw new AppError(503, 'Upload storage is not configured'); const s = await lstat(root); if (!s.isDirectory() || s.isSymbolicLink() || (s.mode & 0o077) !== 0)
    throw new AppError(503, 'Upload storage must be a private directory'); return { root, file: join(root, key + '.blob') }; }
function seal(bytes: Buffer) { const key = process.env.EPCERP_DOCUMENT_READING_KEY ?? ''; if (!/^[a-f\d]{64}$/i.test(key))
    throw new AppError(503, 'Upload encryption is not configured'); const nonce = randomBytes(12), cipher = createCipheriv('aes-256-gcm', Buffer.from(key, 'hex'), nonce); return Buffer.concat([nonce, cipher.update(bytes), cipher.final(), cipher.getAuthTag()]); }
export async function writeObject(key: string, request: Request, size: number, hash: string) {
    const p = await path(key), capacity = await statfs(p.root);
    if (capacity.bavail * capacity.bsize < size + (256 << 20))
        throw new AppError(507, 'Insufficient document storage capacity');
    const out = await open(p.file, constants.O_CREAT | constants.O_EXCL | constants.O_WRONLY | constants.O_NOFOLLOW, 0o600);
    let success = false;
    const source = request.body?.getReader(), digest = createHash('sha256');
    let received = 0, pending = Buffer.alloc(0);
    try {
        await out.writeFile(magic);
        if (!source)
            throw new AppError(422, 'Upload body required');
        while (true) {
            request.signal.throwIfAborted();
            const { value, done } = await source.read();
            if (done)
                break;
            received += value.length;
            if (received > size)
                throw new AppError(422, 'Uploaded size does not match declaration');
            digest.update(value);
            pending = Buffer.concat([pending, Buffer.from(value)]);
            while (pending.length >= chunkSize) {
                const frame = seal(pending.subarray(0, chunkSize)), length = Buffer.alloc(4);
                length.writeUInt32BE(frame.length);
                await out.writeFile(Buffer.concat([length, frame]));
                pending = pending.subarray(chunkSize);
            }
        }
        if (received !== size || digest.digest('hex') !== hash)
            throw new AppError(422, 'Uploaded checksum or size does not match declaration');
        if (pending.length) {
            const frame = seal(pending), length = Buffer.alloc(4);
            length.writeUInt32BE(frame.length);
            await out.writeFile(Buffer.concat([length, frame]));
        }
        await out.writeFile(Buffer.alloc(4));
        await out.sync();
        success = true;
    }
    finally {
        if (source) {
            if (!success)
                await source.cancel().catch(() => { });
            source.releaseLock();
        }
        await out.close();
        if (!success)
            await unlink(p.file).catch(() => { });
    }
}
export async function readObject(key: string, size: number, hash: string) { const p = await path(key), input = await open(p.file, constants.O_RDONLY | constants.O_NOFOLLOW); try {
    const s = await input.stat();
    if (!s.isFile() || s.size > size + ((size / chunkSize + 2) * 32) + 12)
        throw Error('Retained upload integrity failed');
    const data = await input.readFile();
    if (!data.subarray(0, 8).equals(magic))
        throw Error('Retained upload integrity failed');
    const parts: Buffer[] = [];
    let at = 8, total = 0;
    const { createDecipheriv } = await import('node:crypto');
    while (true) {
        if (at + 4 > data.length)
            throw Error('Retained upload integrity failed');
        const n = data.readUInt32BE(at);
        at += 4;
        if (n === 0)
            break;
        if (n < 29 || n > chunkSize + 28 || at + n > data.length)
            throw Error('Retained upload integrity failed');
        const f = data.subarray(at, at + n);
        at += n;
        const decipher = createDecipheriv('aes-256-gcm', Buffer.from(process.env.EPCERP_DOCUMENT_READING_KEY!, 'hex'), f.subarray(0, 12));
        decipher.setAuthTag(f.subarray(-16));
        const bytes = Buffer.concat([decipher.update(f.subarray(12, -16)), decipher.final()]);
        total += bytes.length;
        if (total > size)
            throw Error('Retained upload integrity failed');
        parts.push(bytes);
    }
    const result = Buffer.concat(parts);
    if (at !== data.length || total !== size || createHash('sha256').update(result).digest('hex') !== hash)
        throw Error('Retained upload integrity failed');
    return result;
}
finally {
    await input.close();
} }

export async function removeObject(key:string){const p=await path(key);try{await unlink(p.file);}catch(e){if((e as NodeJS.ErrnoException).code!=='ENOENT')throw e;}}
