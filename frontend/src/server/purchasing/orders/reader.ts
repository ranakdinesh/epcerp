import type { DB } from '../../db';
import { isoDate,invalid } from '../core/validation';
import { id } from '../sourcing/values';
import { guard,missing } from './policy';
import { get } from './repository';
export async function released(db:DB,p:string,pkg:string){const r=await get(db,p,pkg);if(!r?.is_active||r.project_id!==p||r.package_id!==pkg||r.source_snapshot.project_id!==p||r.source_snapshot.package_id!==pkg)throw missing();if(r.state!=='released'||!r.number||r.version<2||!r.released_at||r.released_at.startsWith('0001-01-01T00:00:00')||!r.released_by_id||!isoDate(r.details.date))invalid('purchase_order','Release the actual purchase order before recording vendor acceptance');for(const v of [r.id,r.project_id,r.package_id,r.released_by_id])id(v);guard(r.source_snapshot);return r;}
