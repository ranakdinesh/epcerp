import type { DB } from '../../db';
import { indiaDate } from '../../personal-tasks/validation';
import { source, suppliers, verify } from './context';
import { get } from './repository';
import { currentReleased, selected, released as fact } from './policy';
export async function currentReleasedFact(db: DB, p: string, id: string) { const record = await get(db, p, id); if (!record?.is_active || record.state !== 'released')
    return null; const today = indiaDate(new Date()), offer = selected(record, today); return currentReleased(record, await source(db, p, id), (await suppliers(db, p, id)).find(s => s.id === offer.supplier_id), await verify(db, p, id, offer), today); }
export async function released(db: DB, p: string, id: string) { const record = await get(db, p, id); return record ? fact(record) : null; }
