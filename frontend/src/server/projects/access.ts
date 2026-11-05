import 'server-only';
import type { DB } from '../db';
import { requireAnyPermission } from '../auth/access';
import { ProjectError } from './validation';
import type { Project } from './repository';
export type Actor=Awaited<ReturnType<typeof access>>;
export async function access(db:DB,token:string,extra:string[]=[]){const a=await requireAnyPermission(db,token,['projects.master.read'],true);if(extra.some(p=>!a.allows(p)))throw new ProjectError(403,'project access denied');return a;}
export const portfolio=(a:Actor)=>['projects.portfolio.read','projects.portfolio.write','projects.manager_assignments.write'].some(p=>a.allows(p));
export const correct=(a:Actor,p:Project)=>a.allows('projects.master.write')&&(a.id===p.project_manager_id||a.allows('projects.portfolio.write'));
export async function assigned(db:DB,a:Actor){
    const candidates=(await db.query('SELECT DISTINCT project_id FROM project_responsibility_profiles WHERE user_id=$1 AND is_active ORDER BY project_id LIMIT 1001',[a.id])).rows;
    if(candidates.length>1000)throw new Error('Assigned project reporting limit exceeded');
    if(!a.allows('projects.dashboard.read'))return [];
    const out:string[]=[];
    for(const {project_id:id}of candidates){
        const p=(await db.query('SELECT project_manager_id FROM projects WHERE id=$1',[id])).rows[0];if(!p)continue;
        if(a.superuser||a.allows('projects.portfolio.read')||a.allows('projects.portfolio.write')||p.project_manager_id===a.id){out.push(id);continue;}
        const count=Number((await db.query('SELECT count(*) FROM project_responsibility_profiles WHERE project_id=$1',[id])).rows[0].count);if(count>1000)throw new ProjectError(422,'Project responsibility limit reached','profiles');
        const r=(await db.query(`SELECT EXISTS(SELECT 1 FROM project_responsibility_profiles p JOIN auth.roles r ON r.id=p.bundle_id AND r.is_active JOIN auth.user_roles ur ON ur.role_id=r.id AND ur.user_id=p.user_id JOIN auth.role_permissions rp ON rp.role_id=r.id JOIN auth.permissions permission ON permission.id=rp.permission_id WHERE p.project_id=$1 AND p.user_id=$2 AND p.is_active AND p.permission_codes ? 'projects.dashboard.read' AND permission.code='projects.dashboard.read') AS allowed`,[id,a.id])).rows[0];
        if(r.allowed)out.push(id);
    }return out;
}
export async function requireRead(db:DB,a:Actor,p:Project){if(portfolio(a)||p.project_manager_id===a.id)return;if(!(await assigned(db,a)).includes(p.id))throw new ProjectError(403,'project access denied');}
