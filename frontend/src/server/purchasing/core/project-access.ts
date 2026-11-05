import type { DB } from '../../db';
import type { Actor,Project } from './access';
import { assignmentBundles,assignedBundleIDs } from '../../auth/access';
import { scoped } from '../../projects/team/validation';
import { PurchaseError,invalid } from './validation';
const financial=new Set(['accounts.purchase_acceptance.write','accounts.purchase_advances.write','accounts.purchase_advances.disburse','projects.budgets.approve','accounts.quotation_approvals.write','projects.group_awards.approve','projects.purchase_sourcing.approve']);
// Published project-scoped authority, including financial decision separation.
export async function projectAccess(db:DB,a:Actor,p:Project,permission:string,write:boolean){
    const denied=()=>new PurchaseError(403,'Project responsibility access denied');
    if(!scoped(permission)||!a.allows(permission))throw denied();
    const portfolioRead=a.superuser||a.allows('projects.portfolio.read')||a.allows('projects.portfolio.write'),portfolioWrite=a.superuser||a.allows('projects.portfolio.write');
    let allowed=(write?portfolioWrite:portfolioRead)||p.project_manager_id===a.id&&!financial.has(permission);
    if(!allowed){const profiles=(await db.query('SELECT user_id,bundle_id,permission_codes,is_active FROM project_responsibility_profiles WHERE project_id=$1 ORDER BY bundle_id,id LIMIT 1001',[p.id])).rows;if(profiles.length>1000)invalid('profiles','Project responsibility limit reached');const membership=await assignedBundleIDs(db,a.id);
        for(const profile of profiles){if(!profile.is_active||profile.user_id!==a.id||!profile.permission_codes.includes(permission)||!membership.includes(profile.bundle_id))continue;const [bundle]=await assignmentBundles(db,profile.bundle_id);if(bundle?.is_active&&bundle.permissions.includes(permission)){allowed=true;break;}}
    }
    if(!allowed)throw denied();const open=p.is_active&&!['closed','on_hold'].includes(p.lifecycle_status);if(write&&!open)invalid('project','Project is inactive, held or closed');return{can_write:open&&!(!write&&portfolioRead&&!portfolioWrite&&p.project_manager_id!==a.id)};
}
