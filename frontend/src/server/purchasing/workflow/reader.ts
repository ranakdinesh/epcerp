import type { DB } from '../../db';
import type { Actor } from '../core/access';
import { actions,source } from './context';
import { get } from './repository';
import { guard,missing,workGuard } from './policy';
import { invalid } from '../core/validation';
export async function inspect(db:DB,p:string,id:string,requireAvailable=false){const record=await get(db,p,id);if(!record?.is_active)throw missing();const current=await source(db,p,record.indent_id,record.demand_id);if(requireAvailable)guard(current);return{record,current};}
export async function stepAccess(db:DB,a:Actor,p:string,id:string,n:number){const{record,current}=await inspect(db,p,id),caps=await actions(db,a,p);if(n<3||n>20||record.steps.length!==20)invalid('step','Choose preparation step3–20');const s=record.steps[n-1];let work=false;try{guard(current);workGuard(a.id,caps,s,'start');work=true;}catch(e){if(!(e instanceof Error))throw e;}return{work,read:caps.execute&&s.doer_id===a.id||caps.resolve&&s.blocked_resolver_id===a.id,cancel:caps.execute&&(caps.recovery||s.doer_id===a.id)};}
