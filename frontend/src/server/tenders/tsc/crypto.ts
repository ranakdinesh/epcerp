import 'server-only';
import { createCipheriv, createDecipheriv, createHash, randomBytes, timingSafeEqual } from 'node:crypto';
export const digest = (value: Buffer | string) => createHash('sha256').update(value).digest('hex');
function key() { const value = process.env.EPCERP_DOCUMENT_READING_KEY ?? ''; if (!/^[a-f0-9]{64}$/i.test(value))
    throw Error('Document encryption key is not configured'); return Buffer.from(value, 'hex'); }
export function encrypt(value: unknown) { const bytes = Buffer.from(JSON.stringify(value)), nonce = randomBytes(12), cipher = createCipheriv('aes-256-gcm', key(), nonce); return { cipher: Buffer.concat([nonce, cipher.update(bytes), cipher.final(), cipher.getAuthTag()]), hash: digest(bytes) }; }
export function decryptBytes(bytes: Buffer, hash: string) { if (bytes.length < 28 || !/^[a-f0-9]{64}$/.test(hash))
    throw Error('Retained document integrity failed'); const cipher = createDecipheriv('aes-256-gcm', key(), bytes.subarray(0, 12)); cipher.setAuthTag(bytes.subarray(-16)); const plain = Buffer.concat([cipher.update(bytes.subarray(12, -16)), cipher.final()]); if (!timingSafeEqual(Buffer.from(digest(plain), 'hex'), Buffer.from(hash, 'hex')))
    throw Error('Retained document integrity failed'); return plain; }
export const decrypt = <T>(bytes: Buffer, hash: string): T => JSON.parse(decryptBytes(bytes, hash).toString('utf8'));
