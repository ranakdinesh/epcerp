import 'server-only';
import type { DB } from '../db';
import { AppError } from '../errors';
import { purchaseTaskItem as item } from '../master/items/task-reader';
import { indiaDate } from '../personal-tasks/validation';
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
    const r=(await db.query(`SELECT w.*,p.name AS project_name,p.is_active AS project_active,p.purchase_enabled,p.lifecycle_status,p.client_id FROM purchase_work_packages w JOIN projects p ON p.id=w.project_id WHERE w.id=$1 AND w.is_active`,[packageID])).rows[0];
    if(!r)throw new AppError(404,'Purchase package not found');
    const indent=(await db.query('SELECT data,is_active FROM purchase_indents WHERE id=$1 AND project_id=$2',[r.indent_id,r.project_id])).rows[0];
    if(!indent)throw new AppError(404,'Source indent not found');
    const demand=(await db.query('SELECT * FROM project_material_requirements WHERE id=$1 AND project_id=$2',[r.demand_id,r.project_id])).rows[0];
    if(!demand)throw new AppError(404,'Project requirement not found');
    const client=(await db.query('SELECT is_active FROM clients WHERE id=$1',[r.client_id])).rows[0];
    const selected=await item(db,demand.item_id),config=selected.snapshot.configuration;
    let active=demand.is_active&&!!client?.is_active&&selected.approved_for_purchase;
    await db.query('SELECT pg_advisory_xact_lock(91827148)');
    const rules=(await db.query('SELECT data,is_active FROM client_purchase_rules WHERE client_id=$1 ORDER BY created_at,id',[r.client_id])).rows as Rule[];
    const today=indiaDate(new Date()),blockers=policyReasons(rules,r.project_id,config,selected.manufacturer_id,today);
    let eligible=blockers.length===0;
    for(const id of demand.alternatives??[]){const alt=await item(db,id);active=active&&alt.approved_for_purchase;if(policyReasons(rules,r.project_id,{...config,code:alt.snapshot.configuration.code},alt.manufacturer_id,today).length){eligible=false;blockers.push('An alternative manufacturer is not client-permitted');}}
    const holds=(await db.query('SELECT data,is_active FROM purchase_holds WHERE project_id=$1 ORDER BY created_at,id',[r.project_id])).rows;
    for(const h of holds){const d=h.data;if(h.is_active&&new Date(d.started_at).getTime()<=Date.now()&&(d.scope==='project'||d.scope==='category'&&d.category_code===config.category_code||d.scope==='items'&&d.demand_ids?.includes(r.demand_id)))blockers.push(d.reason);}
    if(!indent.is_active)blockers.push('Source indent is withdrawn');
    if(!active)blockers.push('Current requirement, item or client is inactive');
    const released=(indent.data.released??[]).find((line:{demand_id:string})=>line.demand_id===r.demand_id);
    if(!released)throw new AppError(422,'Only a released indent line can create a purchase package');
    if(demand.version!==released.source.demand_version)blockers.push('Project requirement changed since indent release; source review required');
    return {project_name:r.project_name,available:!!r.project_active&&!!r.purchase_enabled&&!['on_hold','closed'].includes(r.lifecycle_status)&&active&&eligible&&!!indent.is_active&&blockers.length===0,blockers};
}
