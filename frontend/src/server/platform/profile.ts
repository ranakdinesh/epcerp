import 'server-only';
import sharp from 'sharp';
import { randomUUID, createCipheriv, createDecipheriv, randomBytes } from 'node:crypto';
import { transaction } from '../db';
import { requireAnyPermission } from '../auth/access';
import { AppError } from '../errors';
import { body, bytes, json } from '../purchasing/http-input';
import { uuid, object } from '../purchasing/core/validation';
import { putObject, readStoredObject, documentBucket, hash } from '../master/documents/files';
import { contact, orgEvent } from './organisation';
import { journal } from '../purchasing/core/journal';
import { timeSQL, timestamp } from '../inbox/repository';
const invalid = () => new AppError(422, 'Use a display name of at most 100 characters or a PNG/JPEG up to 512KiB and 2048×2048 pixels');
function key() {
    const k = process.env.EPCERP_IMAGE_ENCRYPTION_KEY ?? '';
    if (!/^[a-f\d]{64}$/i.test(k))
        throw Error('Image encryption unavailable');
    return Buffer.from(k, 'hex');
}
export async function handleProfile(r: Request) {
    try {
        const p = new URL(r.url).pathname.replace(/^\/api\/v1\/auth\/user_profile\/?/, '').split('/').filter(Boolean), token = r.headers.get('authorization')?.replace(/^Bearer /i, '') ?? '', write = r.method !== 'GET';
        return await transaction(async (db) => {
            const a = await requireAnyPermission(db, token, [write ? 'account.profile.write' : 'account.profile.read'], write ? true : 'business'), target = p[0] === 'me' ? a.id : uuid(p[0]);
            if (write && p[0] !== 'me')
                throw new AppError(403, 'Permission denied');
            let u = (await db.query('SELECT id,username,first_name,last_name,mobile FROM auth.users WHERE id=$1 AND is_active', [target])).rows[0];
            if (!u)
                throw new AppError(404, 'User not found');
            let row = (await db.query(`SELECT *,${timeSQL('updated_at')}AS stamp FROM auth.user_profile WHERE user_id=$1 AND is_active`, [target])).rows[0];
            let name: string | null = null, replace = false, photo: Buffer | null = null, objectKey: string | null = null, photoHash: string | null = null, action = 'details_updated';
            const event = randomUUID();
            if (r.method === 'PATCH' && p.length === 1) {
                const b = await body(r, ['display_name', 'contact', 'expected_contact', 'version'], 8192);
                if (typeof b.display_name !== 'string' || [...b.display_name.trim()].length > 100 || /[\r\n\x00]/.test(b.display_name))
                    throw invalid();
                name = b.display_name.trim();
                if (b.contact !== undefined) {
                    const c = contact(object(b.contact, ['first_name', 'last_name', 'mobile'])), expected = object(b.expected_contact, ['first_name', 'last_name', 'mobile']);
                    if (b.version !== Number(row?.version ?? 0) || ['first_name', 'last_name', 'mobile'].some(k => u[k] !== expected[k]))
                        throw new AppError(409, 'Your profile changed. Close and reopen it before saving.');
                    const before = { first_name: u.first_name, last_name: u.last_name, mobile: u.mobile };
                    await db.query('UPDATE auth.users SET first_name=$2,last_name=$3,mobile=$4,updated_at=clock_timestamp(),updated_by_id=$1 WHERE id=$1', [target, c.first_name, c.last_name, c.mobile]);
                    await db.query('UPDATE auth.auth_reporting_lines SET version=version+1,updated_at=clock_timestamp(),updated_by_id=$1 WHERE user_id=$1', [target]);
                    await orgEvent(db, target, 'employee', target, 'Account holder updated personal profile.', randomUUID());
                    await db.query('INSERT INTO auth.auth_personal_contact_changes(id,user_id,version,reason,before_snapshot,after_snapshot,actor_id)VALUES($1,$2,$3,$4,$5,$6,$2)', [event, target, Number(row?.version ?? 0) + 1, 'Account holder updated personal profile.', before, c]);
                    u = { ...u, ...c };
                    action = 'personal_details_updated';
                }
                else if (b.version !== undefined || b.expected_contact !== undefined)
                    throw invalid();
            }
            else if ((r.method === 'PUT' || r.method === 'DELETE') && p[1] === 'photo' && p.length === 2) {
                replace = true;
                action = r.method === 'PUT' ? 'photo_updated' : 'photo_removed';
                if (r.method === 'PUT') {
                    if (!['image/png', 'image/jpeg'].includes(r.headers.get('content-type') ?? ''))
                        throw invalid();
                    const data = await bytes(r, 512 * 1024);
                    if (!data.length)
                        throw invalid();
                    const image = sharp(data, { limitInputPixels: 2048 * 2048, failOn: 'warning' }), meta = await image.metadata().catch(() => {
                        throw invalid();
                    });
                    if (!meta.width || !meta.height || meta.width > 2048 || meta.height > 2048 || !['png', 'jpeg'].includes(meta.format ?? ''))
                        throw invalid();
                    photo = await image.flatten({ background: '#ffffff' }).resize(256, 256, { fit: 'cover', position: 'centre', kernel: 'nearest' }).jpeg({ quality: 85 }).toBuffer();
                    objectKey = 'images/auth/profiles/' + target + '/' + randomUUID() + '.enc';
                    photoHash = hash(photo);
                    const nonce = randomBytes(12), c = createCipheriv('aes-256-gcm', key(), nonce);
                    await putObject(documentBucket(), objectKey, Buffer.concat([nonce, c.update(photo), c.final(), c.getAuthTag()]));
                }
            }
            else if (r.method !== 'GET')
                return json({ detail: 'Not found' }, 404);
            if (write) {
                row = (await db.query(`INSERT INTO auth.user_profile(user_id,display_name,photo_object_key,photo_sha256,created_by_id,updated_by_id)VALUES($1,coalesce($2,''),$3,$4,$1,$1)ON CONFLICT(user_id)DO UPDATE SET display_name=coalesce($2,user_profile.display_name),photo=CASE WHEN $5 THEN NULL ELSE user_profile.photo END,photo_object_key=CASE WHEN $5 THEN $3 ELSE user_profile.photo_object_key END,photo_sha256=CASE WHEN $5 THEN $4 ELSE user_profile.photo_sha256 END,version=user_profile.version+1,updated_at=now(),updated_by_id=$1 RETURNING *,${timeSQL('updated_at')}AS stamp`, [target, name, objectKey, photoHash, replace])).rows[0];
                await journal(db, {
                    id: event, module: 'auth', type: 'user_profile', entity: target, version: Number(row.version), action, actor: target, project: null, at: timestamp(row.stamp)!
                });
            }
            if (r.method === 'GET' && p[1] === 'photo') {
                let data = row?.photo as Buffer | null;
                if (row?.photo_object_key) {
                    if (!String(row.photo_object_key).startsWith('images/'))
                        throw Error('Invalid photo reference');
                    const encrypted = await readStoredObject(documentBucket(), row.photo_object_key);
                    if (encrypted.length > 512 * 1024 + 28)
                        throw invalid();
                    const c = createDecipheriv('aes-256-gcm', key(), encrypted.subarray(0, 12));
                    c.setAuthTag(encrypted.subarray(-16));
                    data = Buffer.concat([c.update(encrypted.subarray(12, -16)), c.final()]);
                    if (hash(data) !== row.photo_sha256)
                        throw Error('Photo integrity failed');
                }
                return new Response(data?.length ? new Uint8Array(data) : null, { status: data?.length ? 200 : 204, headers: { 'Content-Type': 'image/jpeg', 'Cache-Control': 'private, no-store', 'X-Content-Type-Options': 'nosniff' } });
            }
            return json({
                user_id: target, display_name: row?.display_name || u.username, has_photo: !!(row?.photo?.length || row?.photo_object_key), version: Number(row?.version ?? 0), ...(target === a.id ? { contact: { first_name: u.first_name, last_name: u.last_name, mobile: u.mobile } } : {})
            });
        });
    }
    catch (e) {
        if (e instanceof AppError)
            return json({ detail: e.message }, e.status);
        console.error('Profile operation unavailable');
        return json({ detail: 'Profile service unavailable' }, 500);
    }
}
