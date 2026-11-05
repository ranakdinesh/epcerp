import 'server-only';
import { randomUUID } from 'node:crypto';
import type { DB } from '../../db';
import { PurchaseError,invalid,uuid,reason,isoDate,trim,lower } from '../core/validation';
import { quantity } from '../core/decimal';
import { catalogue,permitted } from '../core/catalogue';
import { journal } from '../core/journal';
import * as repo from './repository';
export type Save = { item_id:string;quantity:string;required_date:string;allocations:repo.Allocation[];alternatives:string[];is_active:boolean;version:number;reason:string };
const stale = () => new PurchaseError(409,'Project requirement changed; reload before saving');
function validate(input:Save) {
    const b=structuredClone(input);b.item_id=uuid(b.item_id,'item_id','Select a catalogue item');const q=quantity(b.quantity);
    if(!isoDate(b.required_date))invalid('required_date','Use a valid required-at-site date');
    if(!Number.isInteger(b.version)||b.version<1)invalid('version','Use the current material requirement version');
    b.reason=reason(b.reason,'Provide a reason within 2000 characters');b.allocations??=[];b.alternatives??=[];
    if(b.allocations.length>100||b.alternatives.length>100)invalid('allocations','Use at most 100 allocations and alternatives');
    const locations=new Set<string>();let total=BigInt(0);
    for(const allocation of b.allocations){allocation.location=trim(allocation.location);const key=lower(allocation.location);if(!key||[...allocation.location].length>255)invalid('allocations','Enter a location within 255 characters');if(locations.has(key))invalid('allocations','Each location must occur once');locations.add(key);total+=quantity(allocation.quantity);}
    if(b.allocations.length&&total!==q)invalid('allocations','Location allocations must total the project item quantity');
    const seen=new Set([b.item_id]);b.alternatives=b.alternatives.map(id=>{const key=uuid(id,'alternatives','Select valid alternative items');if(seen.has(key))invalid('alternatives','Duplicate or primary item in alternatives');seen.add(key);return key;});
    return b;
}
// Published writer: caller owns purchasing/material-write authorization and project lock.
export async function publish(db:DB,project:string,id:string|undefined,raw:Save,actor:string) {
    const b=validate(raw),before=id?await repo.demand(db,project,id):null;
    if(id&&!before)throw new PurchaseError(404,'Project material requirement not found');
    if(before&&before.version!==b.version)throw stale();
    if(before&&before.item_id!==b.item_id)invalid('item_id',"A project item's SKU cannot be replaced");
    if(before){const committed=(await db.query('SELECT coalesce(sum(r.quantity),0)::text AS quantity FROM purchase_indent_releases r JOIN purchase_indents i ON i.id=r.indent_id WHERE i.project_id=$1 AND r.demand_id=$2',[project,id])).rows[0].quantity;if(BigInt(committed.replace('.',''))!==BigInt(0)&&(!b.is_active||quantity(b.quantity)<quantity(committed)))invalid('quantity','Requirement cannot be reduced below released purchasing quantity or deactivated');}
    let snapshot=before?.item_snapshot;
    if(!before||b.is_active&&(!before.is_active||quantity(b.quantity)>quantity(before.quantity)))snapshot=await catalogue(db,b.item_id);
    if(!snapshot)throw new Error('Material snapshot missing');
    if(b.is_active)await permitted(db,project,snapshot);
    if(['nos','no','pcs','pieces','each'].includes(lower(trim(snapshot.uom))))for(const amount of [b.quantity,...b.allocations.map(a=>a.quantity)])if(quantity(amount)%BigInt(1000)!==BigInt(0))invalid('quantity',snapshot.uom+' quantities must be whole numbers');
    for(const d of await repo.demands(db,project)){if(d.id===id)continue;if(d.item_id===b.item_id)invalid('item_id','This SKU already exists in the project; update its existing requirement');if(d.engineering_key===snapshot.engineering_key)invalid('item_id','This engineering configuration already exists; add the make as an alternative');}
    for(const alt of b.alternatives){const other=await permitted(db,project,await catalogue(db,alt));if(other.engineering_key!==snapshot.engineering_key)invalid('alternatives','An alternative must have the same engineering configuration');}
    if(!before&&b.version!==1)throw stale();
    const record={...b,id:id??randomUUID(),project_id:project,engineering_key:snapshot.engineering_key,item_snapshot:snapshot,version:before?before.version+1:1};
    let saved;
    try{saved=await repo.save(db,record,actor);}catch(error){if((error as {code?:string}).code==='23505')invalid('item_id','This SKU or engineering configuration already exists in the project');throw error;}
    if(!saved)throw stale();
    const event=randomUUID(),action=before?'revise':'create';
    await db.query('INSERT INTO project_material_events(id,subject_id,action,reason,before_snapshot,after_snapshot,created_by_id)VALUES($1,$2,$3,$4,$5,$6,$7)',[event,saved.id,action,b.reason,JSON.stringify(before),JSON.stringify(saved),actor]);
    await journal(db,{id:event,module:'project_items',type:'project_material_requirement',entity:saved.id,version:saved.version,action,actor,project,at:saved.updated_at});
    return saved;
}
