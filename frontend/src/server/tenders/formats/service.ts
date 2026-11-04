import 'server-only';
import { randomUUID } from 'node:crypto';
import { transaction, type DB } from '../../db';
import { AppError } from '../../errors';
import { access, admit } from '../intake/access';
import { id, integer, object, flag } from '../tsc/validation';
import { lower } from '../intake/receipts';
export const defaultFormat = '00000000-0000-4000-8000-000000000097';
export type FormatQuestion = {
    id: string;
    section: string;
    printed_number: string;
    question: string;
    manual_review: boolean;
    row: number;
};
export type Format = {
    id: string;
    name: string;
    purpose: string;
    description: string;
    is_active: boolean;
    version: number;
    question_count: number;
    questions: FormatQuestion[];
    created_at: string;
    updated_at: string;
};
export const questionKey = (value: string) => lower(value).replace(/[\p{White_Space}\p{P}]/gu, '');
function text(value: unknown, name: string, min: number, max: number, fallback?: string) { if (value == null && fallback !== undefined)
    value = fallback; if (typeof value !== 'string' || /[\uD800-\uDFFF]/u.test(value))
    throw new AppError(422, 'Invalid ' + name); const s = value.trim(); if ([...s].length < min || [...s].length > max)
    throw new AppError(422, `${name} must contain ${min}–${max} characters`); return s; }
async function loaded(db: DB, key: string): Promise<Format> { const r = (await db.query('SELECT * FROM tender.tsc_formats WHERE id=$1 FOR UPDATE', [id(key)])).rows[0]; if (!r)
    throw new AppError(404, 'TSC format not found'); const questions = (await db.query('SELECT id,section,printed_number,question,manual_review,legacy_row AS row FROM tender.tsc_format_questions WHERE format_id=$1 ORDER BY position', [key])).rows as FormatQuestion[]; return { id: r.id, name: r.name, purpose: r.purpose, description: r.description, is_active: r.is_active, version: Number(r.version), question_count: questions.length, questions, created_at: r.created_at, updated_at: r.updated_at }; }
export async function list(token: string) { await admit(token, 'format', 'read'); return transaction(async (db) => { await access(db, token, 'format'); return { items: (await db.query('SELECT f.id,f.name,f.purpose,f.description,f.is_active,f.version,f.created_at,f.updated_at,(SELECT count(*)FROM tender.tsc_format_questions q WHERE q.format_id=f.id)::int AS question_count FROM tender.tsc_formats f ORDER BY f.name_key LIMIT 500')).rows.map(f => ({ ...f, version: Number(f.version) })) }; }); }
export async function get(token: string, key: string, exporting = false) { await admit(token, 'format', exporting ? 'preview' : 'read'); return transaction(async (db) => { await access(db, token, 'format'); return loaded(db, key); }); }
export async function save(token: string, key: string | undefined, value: unknown) {
    await admit(token, 'format', 'write', true);
    if (key)
        id(key);
    const r = object(value, ['name', 'purpose', 'description', 'is_active', 'expected_version', 'questions']), version = integer(r.expected_version ?? 0, key ? 1 : 0);
    if (!key && version !== 0)
        throw new AppError(422, 'New formats must use version zero');
    const name = text(r.name, 'Format name', 3, 120), description = text(r.description, 'Description', 0, 1000, ''), purpose = text(r.purpose, 'Purpose', 0, 80, '') || 'Generic', active = r.is_active == null ? false : flag(r.is_active);
    if (!questionKey(name))
        throw new AppError(422, 'Provide a format name');
    if (!Array.isArray(r.questions) || r.questions.length < 1 || r.questions.length > 200)
        throw new AppError(422, 'A format must contain 1–200 questions');
    const supplied = r.questions;
    return transaction(async (db) => {
        const actor = await access(db, token, 'format', true), old = await loaded(db, key ?? defaultFormat);
        if (key && old.version !== version)
            throw new AppError(409, 'This format changed; reload it before saving');
        const seen = new Map<string, number>(), ids = new Set<string>(), questions = supplied.map((value, index) => { const q = object(value, ['id', 'section', 'printed_number', 'question', 'manual_review', 'row']), rawID = typeof q.id === 'string' ? q.id : '', section = text(q.section, 'Question section', 1, 80), question = text(q.question, 'Question', 3, 2000), printed_number = text(q.printed_number, 'Question number', 0, 30, ''), manual_review = q.manual_review == null ? false : flag(q.manual_review), previous = old.questions.find(p => p.id === rawID && p.section === q.section && p.question === q.question && p.manual_review === manual_review), uuid = /^[a-f\d]{8}-[a-f\d]{4}-[a-f\d]{4}-[a-f\d]{4}-[a-f\d]{12}$/i.test(rawID), identity = previous || uuid ? rawID : randomUUID(), row = previous ? integer(q.row ?? 0, 0, 2147483647) : 0; if (!/^[a-zA-Z0-9_.-]{1,80}$/.test(identity) || ids.has(identity))
            throw new AppError(422, 'Question identities must be valid and unique'); const normalized = questionKey(question); if (!normalized)
            throw new AppError(422, 'Provide question wording'); if (seen.has(normalized))
            throw new AppError(422, `Question ${index + 1} duplicates question ${seen.get(normalized)}`); seen.set(normalized, index + 1); ids.add(identity); return { id: identity, section, printed_number, question, manual_review, row }; }), target = key ?? randomUUID();
        if (key) {
            await db.query('UPDATE tender.tsc_formats SET name=$2,purpose=$3,description=$4,is_active=$5,version=version+1,updated_by_id=$6,updated_at=clock_timestamp()WHERE id=$1 AND version=$7', [target, name, purpose, description, active, actor.id, version]);
            await db.query('DELETE FROM tender.tsc_format_questions WHERE format_id=$1', [target]);
        }
        else
            await db.query('INSERT INTO tender.tsc_formats(id,name,purpose,description,is_active,created_by_id,updated_by_id)VALUES($1,$2,$3,$4,$5,$6,$6)', [target, name, purpose, description, active, actor.id]);
        for (const [i, q] of questions.entries())
            await db.query('INSERT INTO tender.tsc_format_questions(format_id,id,position,section,printed_number,question,manual_review,legacy_row)VALUES($1,$2,$3,$4,$5,$6,$7,$8)', [target, q.id, i + 1, q.section, q.printed_number, q.question, q.manual_review, q.row]);
        const saved = await loaded(db, target), event = randomUUID();
        await db.query('INSERT INTO tender.tsc_format_events(id,format_id,version,actor_id,before_snapshot,after_snapshot)VALUES($1,$2,$3,$4,$5,$6)', [event, target, saved.version, actor.id, key ? JSON.stringify(old) : null, JSON.stringify(saved)]);
        await db.query("INSERT INTO business_events(id,source_module,entity_type,entity_id,entity_version,delivery_count,action,actor_id,actor_kind,correlation_id,scope_kind,visibility,occurred_at)VALUES($1,'tender_fms','tsc_format',$2,$3,0,'tsc_format_saved',$4,'user',$1,'platform','restricted',clock_timestamp())", [event, target, saved.version, actor.id]);
        return saved;
    });
}
