import 'server-only';
import type { DB } from '../db';
import { AppError } from '../errors';
import { inspect } from './workflow/reader';
type Rule={is_active:boolean;data:{project_id:string|null;scope_code:string;mode:string;allowed:string[];excluded:string[];effective_from:string;effective_until:string|null}};
export function policyReasons(rules:Rule[],project:string,configuration:{code:string;category_code:string;family_code:string},manufacturer:string,today:string){
    const out:string[]=[];
    for(const r of rules){const d=r.data;if(!r.is_active||(d.project_id&&d.project_id!==project)||d.effective_from>today||(d.effective_until&&d.effective_until<today)||!['*',configuration.code,configuration.category_code,configuration.family_code].includes(d.scope_code))continue;
        if(d.excluded?.includes(manufacturer))out.push('Manufacturer excluded by '+d.scope_code+' policy');
        if(d.mode==='approved_only'&&!d.allowed?.includes(manufacturer))out.push('Manufacturer outside approved list for '+d.scope_code);
    }return out;
}
// Read-only source projection corresponding to Purchase's transaction-bound Inspect.
export async function source(db:DB,packageID:string){
    const row=(await db.query('SELECT project_id FROM purchase_work_packages WHERE id=$1 AND is_active',[packageID])).rows[0];
    if(!row)throw new AppError(404,'Purchase package not found');
    const {current}=await inspect(db,row.project_id,packageID);
    return{project_name:current.project_name,available:current.project_available&&current.eligible&&current.blockers.length===0,blockers:[...current.blockers]};
}
