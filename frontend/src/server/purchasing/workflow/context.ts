import type { DB } from '../../db';
import { requireAnyPermission,assignmentIdentities } from '../../auth/access';
import { AppError } from '../../errors';
import { project,scope,type Actor } from '../core/access';
import { projectAccess } from '../core/project-access';
import { workspace } from '../workspace/repository';
import * as indents from '../indents/repository';
import { lineSource } from '../indents/source';
import { invalid } from '../core/validation';
import type { Capabilities,Source,Member } from './types';
import { missing } from './policy';
export const actor=(db:DB,token:string)=>requireAnyPermission(db,token,['projects.purchase_workflow.view','projects.purchase_workflow.execute','projects.purchase_workflow.resolve'],true);
export async function actions(db:DB,a:Actor,p:string):Promise<Capabilities>{
    const parent=await project(db,p,true),w=await workspace(db,p);const check=async(code:string,write:boolean)=>{if(!a.allows(code))return false;try{const v=await scope(db,a,parent,w,code,write);return!write||v.can_write;}catch(e){if(!(e instanceof AppError)||e.status!==403)throw e;try{const v=await projectAccess(db,a,parent,code,write);return!write||v.can_write;}catch(e){if(e instanceof AppError&&e.status===403)return false;throw e;}}};
    const view=await check('projects.purchase_workflow.view',false),read=await check('projects.purchase_workflow.read',false);let write=false;try{write=await check('projects.purchase_workflow.write',true);}catch(e){if(!(e instanceof AppError)||e.status!==422)throw e;}return{read:view&&read,write:view&&read&&write,execute:a.allows('projects.purchase_workflow.execute'),resolve:a.allows('projects.purchase_workflow.resolve'),recovery:a.superuser};
}
export async function members(db:DB):Promise<Member[]>{return(await assignmentIdentities(db,[])).filter(u=>u.allows('projects.purchase_workflow.execute')||u.allows('projects.purchase_workflow.resolve')).map(u=>({id:u.id,label:u.name,can_execute:u.allows('projects.purchase_workflow.execute'),can_resolve:u.allows('projects.purchase_workflow.resolve')}));}
export async function candidates(db:DB,p:string){return(await indents.store.list(db,p)).filter(r=>r.is_active).flatMap(r=>r.data.released.map(l=>({indent_id:r.id,indent_number:r.data.number,demand_id:l.demand_id,description:l.source.item.description,quantity:l.quantity})));}
export async function source(db:DB,p:string,indent:string,demand:string):Promise<Source>{const parent=await project(db,p,true),r=await indents.store.get(db,p,indent);if(!r)throw missing();const current=await lineSource(db,p,demand),site=(await db.query('SELECT site_name FROM projects WHERE id=$1',[p])).rows[0],blockers=[...current.policy.reasons,...current.holds.holds.map(h=>h.reason)];if(!r.is_active)blockers.push('Source indent is withdrawn');if(!current.active)blockers.push('Current requirement, item or client is inactive');const line=r.data.released.find(l=>l.demand_id===demand);if(!line)invalid('demand_id','Only a released indent line can create a purchase package');if(current.demand_version!==line.source.demand_version)blockers.push('Project requirement changed since indent release; source review required');return{project_name:parent.name,site:site.site_name,indent_number:r.data.number,demand_id:demand,project_available:parent.is_active&&parent.purchase_enabled&&!['on_hold','closed'].includes(parent.lifecycle_status),eligible:current.active&&current.policy.status==='eligible'&&r.is_active,blockers,description:line.source.item.description,quantity:line.quantity,uom:line.source.item.uom,required_date:line.source.required_date,full_kit:line.full_kit,release_at:line.at};}
