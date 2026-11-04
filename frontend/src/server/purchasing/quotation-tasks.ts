import 'server-only';
import type { DB } from '../db';
import type { requireAnyPermission } from '../auth/access';
import { AppError } from '../errors';
type Actor=Awaited<ReturnType<typeof requireAnyPermission>>;
async function scoped(db:DB,actor:Actor,project:{id:string;project_manager_id:string},permission:string){
    if(actor.superuser||actor.allows('projects.portfolio.read')||actor.allows('projects.portfolio.write')||project.project_manager_id===actor.id)return true;
    return (await db.query(`SELECT EXISTS(SELECT 1 FROM project_responsibility_profiles p JOIN auth.roles r ON r.id=p.bundle_id AND r.is_active JOIN auth.user_roles ur ON ur.role_id=r.id AND ur.user_id=p.user_id JOIN auth.role_permissions rp ON rp.role_id=r.id JOIN auth.permissions a ON a.id=rp.permission_id WHERE p.project_id=$1 AND p.user_id=$2 AND p.is_active AND p.permission_codes ? $3 AND a.code=$3) AS allowed`,[project.id,actor.id,permission])).rows[0].allowed as boolean;
}
export async function quotationTasks(db:DB,actor:Actor){
    const rows=(await db.query("SELECT * FROM purchase_quotation_work_tasks WHERE recipient_id=$1 AND state<>'completed' ORDER BY created_at DESC,id LIMIT 100",[actor.id])).rows,out=[];
    for(const r of rows){
        const finance=r.kind==='finance',permission=finance?'accounts.quotation_approvals.read':'projects.group_comparisons.read';
        if(!actor.allows(permission))continue;
        const p=(await db.query('SELECT * FROM projects WHERE id=$1',[r.project_id])).rows[0];if(!p)throw new AppError(404,'Quotation project not found');
        let allowed=false;
        if(finance)allowed=await scoped(db,actor,p,permission);
        else if(actor.allows('projects.purchase_workflow.view')&&actor.allows('projects.purchase_workflow.read')){
            const w=(await db.query('SELECT purchase_manager_id,is_active FROM purchase_fms_workspaces WHERE project_id=$1',[p.id])).rows[0];
            allowed=actor.allows('projects.purchase_portfolio.read')||actor.allows('projects.purchase_portfolio.write')||p.project_manager_id===actor.id||!!w?.is_active&&w.purchase_manager_id===actor.id;
        }
        if(allowed)out.push({id:r.id,project_id:p.id,project_name:p.name,title:r.title,link:r.link,state:r.state});
    }return out;
}
