import 'server-only';
import { randomUUID } from 'node:crypto';
import { transaction } from '../db';
import { requireAnyPermission } from '../auth/access';
import { AppError } from '../errors';
import { body, json } from '../purchasing/http-input';
import { uuid, isoDate, trim } from '../purchasing/core/validation';
import { calculate } from '../projects/schedule';
import * as projects from '../projects/repository';
import { journal } from '../purchasing/core/journal';
import { now } from '../purchasing/core/time';
const invalid = () => new AppError(422, 'Invalid holiday name, date, reason or current version');
const conflict = () => new AppError(409, 'Holiday changed or date already exists; refresh before saving');
export async function list(token: string, year: number) {
    return transaction(async (db) => {
        await requireAnyPermission(db, token, ['settings.holidays.read']);
        if (!Number.isInteger(year) || year < 1 || year > 9999)
            throw invalid();
        return (await db.query('SELECT id,name,holiday_date::text,is_active,version FROM work_calendar_holidays WHERE extract(year FROM holiday_date)=$1 ORDER BY holiday_date', [year])).rows;
    });
}
export async function save(token: string, target: string | undefined, b: Record<string, unknown>) {
    return transaction(async (db) => {
        const a = await requireAnyPermission(db, token, ['settings.holidays.write'], true);
        if (typeof b.name !== 'string' || typeof b.reason !== 'string' || typeof b.holiday_date !== 'string' || typeof b.is_active !== 'boolean' || !Number.isInteger(b.expected_version) || Number(b.expected_version) < 0 || Number(b.expected_version) > 2147483647)
            throw invalid();
        const name = trim(b.name), why = trim(b.reason), date = b.holiday_date, v = Number(b.expected_version);
        if ([...name].length < 2 || [...name].length > 128 || !why || [...why].length > 2000 || !isoDate(date) || target && v < 1 || !target && v !== 0)
            throw invalid();
        await db.query('SELECT pg_advisory_xact_lock(70623124)');
        let row;
        try {
            row = target ? (await db.query('UPDATE work_calendar_holidays SET name=$2,holiday_date=$3,is_active=$4,version=version+1,updated_by_id=$5,updated_at=clock_timestamp()WHERE id=$1 AND version=$6 RETURNING id,name,holiday_date::text,is_active,version', [uuid(target), name, date, b.is_active, a.id, v])).rows[0] : (await db.query('INSERT INTO work_calendar_holidays(id,name,holiday_date,is_active,created_by_id,updated_by_id)VALUES($1,$2,$3,$4,$5,$5)RETURNING id,name,holiday_date::text,is_active,version', [randomUUID(), name, date, b.is_active, a.id])).rows[0];
        }
        catch (e) {
            if ((e as {
                code?: string;
            }).code === '23505')
                throw conflict();
            throw e;
        }
        if (!row)
            throw conflict();
        const id = randomUUID();
        await db.query('INSERT INTO work_calendar_changes(id,holiday_id,version,reason,snapshot,created_by_id)VALUES($1,$2,$3,$4,$5,$6)', [id, row.id, row.version, why, {
                ID: row.id, Name: row.name, Date: row.holiday_date, Actor: '', Active: row.is_active, Version: row.version
            }, a.id]);
        await journal(db, {
            id, module: 'work_calendar', type: 'holiday', entity: row.id, version: row.version, action: 'saved', actor: a.id, project: null, at: now()
        });
        const candidates = (await db.query('SELECT id FROM projects WHERE actual_start_date IS NOT NULL AND planned_completion_date IS NOT NULL ORDER BY id FOR UPDATE')).rows;
        for (const c of candidates) {
            const p = await projects.get(db, c.id), calculated = await calculate(db, p), event = randomUUID();
            await db.query('INSERT INTO project_schedule_changes(id,project_id,project_version,before_calculation,after_calculation,created_by_id)VALUES($1,$2,$3,$4,$5,$6)', [event, p.id, p.version + 1, p.schedule_calculation, calculated.schedule_calculation, a.id]);
            await db.query('UPDATE projects SET schedule_calculation=$2,planned_duration_days=$3,safety_buffer_days=$4,version=version+1,updated_by_id=$5,updated_at=clock_timestamp()WHERE id=$1', [p.id, calculated.schedule_calculation, Number(calculated.planned_duration_days || 0), Number(calculated.safety_buffer_days || 0), a.id]);
            await journal(db, {
                id: event, module: 'projects', type: 'project', entity: p.id, version: p.version + 1, action: 'calendar_recalculated', actor: a.id, project: p.id, at: now()
            });
        }
        return row;
    });
}
export async function handleCalendar(r: Request) {
    try {
        const url = new URL(r.url), token = r.headers.get('authorization')?.replace(/^Bearer /i, '') ?? '', p = url.pathname.replace(/^\/api\/v1/, '').split('/').filter(Boolean);
        if (r.method === 'GET' && p.length === 2) {
            if ([...url.searchParams.keys()].some(k => k !== 'year') || url.searchParams.getAll('year').length > 1)
                throw new AppError(422, 'Select a calendar year');
            return json(await list(token, Number(url.searchParams.get('year') || new Date().getUTCFullYear())));
        }
        if ((r.method === 'POST' && p.length === 2) || (r.method === 'PATCH' && p.length === 3))
            return json(await save(token, p[2], await body(r, ['name', 'holiday_date', 'is_active', 'reason', 'expected_version'], 16384)));
        return json({ detail: 'Not found' }, 404);
    }
    catch (e) {
        if (e instanceof AppError)
            return json({ detail: e.message }, e.status);
        console.error('Calendar operation unavailable');
        return json({ detail: 'Calendar operation failed' }, 500);
    }
}
