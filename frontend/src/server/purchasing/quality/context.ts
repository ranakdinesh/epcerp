import type { DB } from '../../db';
import { requireAnyPermission } from '../../auth/access';
import { AppError } from '../../errors';
import type { Actor } from '../core/access';
import { inspect } from '../workflow/reader';
import { stageAccess } from '../core/stage-access';
import { issuedFact } from '../inspection-reports/service';
import { invalid } from '../core/validation';
import type { Source } from './types';
export const actor=(db:DB,token:string)=>requireAnyPermission(db,token,['projects.purchase_quality.read'],true);
export async function access(db:DB,a:Actor,p:string,id:string){const c=await stageAccess(db,a,p,id,'projects.purchase_quality',14);return{...c,verify:c.read&&a.allows('projects.purchase_quality.verify')};}
export async function source(db:DB,p:string,id:string):Promise<Source>{const{record,current}=await inspect(db,p,id),out:Source={project_id:p,package_id:id,report_id:'',report_version:0,report_revision:0,ir_number:'',irn_number:'',issue_date:'',cleared_quantity:'',uom:'',order_id:'',order_version:0,number:'',supplier_name:'',description:'',inspection_id:'',inspection_version:0,available:current.project_available&&current.eligible,blockers:[...current.blockers]};for(const step of record.steps)if(step.number===14&&step.state==='blocked')out.blockers.push('Quality certificates have an unresolved blocker');let actual;try{actual=await issuedFact(db,p,id);}catch(e){if(e instanceof AppError&&[404,409,422].includes(e.status)){out.available=false;out.blockers.push(e.message);return out;}throw e;}if(actual.project_id!==p||actual.package_id!==id)invalid('source','Report belongs to another project/package');const s=actual.source_snapshot,r=actual.report;return{...out,report_id:actual.id,report_version:actual.version,report_revision:actual.revision,ir_number:r.ir_number,irn_number:r.irn_number,issue_date:r.issue_date,cleared_quantity:r.cleared_quantity,uom:r.uom,order_id:s.order_id,order_version:s.order_version,number:s.number,supplier_name:s.supplier_name,description:s.description,inspection_id:s.inspection_id,inspection_version:s.inspection_version};}
