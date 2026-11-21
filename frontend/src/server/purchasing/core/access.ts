import 'server-only';
import type { DB } from '../../db';
import { requireAnyPermission } from '../../auth/access';
import { PurchaseError, invalid, uuid } from './validation';
export type Actor = Awaited<ReturnType<typeof actor>>;
export async function actor(db: DB, token: string, permissions: string[] = []) {
    const user = await requireAnyPermission(db, token, ['projects.purchase_workflow.view'], 'business');
    if (!user.allows('projects.purchase_workflow.read') || permissions.some(code => !user.allows(code)))
        throw forbidden();
    return user;
}
export const forbidden = () => new PurchaseError(403, 'purchase project access denied');
export const missing = () => new PurchaseError(404, 'purchase project not found');
export const portfolio = (user: Actor) => user.allows('projects.purchase_portfolio.read') || user.allows('projects.purchase_portfolio.write');
export type Project = {
    id: string;
    name: string;
    project_number: string;
    project_manager_id: string;
    client_id: string;
    is_active: boolean;
    purchase_enabled: boolean;
    lifecycle_status: string;
    version: number;
};
export type Workspace = {
    project_id: string;
    purchase_manager_id: string | null;
    version: number;
    is_active: boolean;
    updated_at: string;
    updated_by_id: string | null;
};
export const open = (project: Project) => project.is_active && !['closed', 'on_hold'].includes(project.lifecycle_status);
export const readable = (user: Actor, project: Project, workspace: Workspace) => portfolio(user) || project.project_manager_id === user.id || workspace.is_active && workspace.purchase_manager_id === user.id;
export const executable = (user: Actor, project: Project, workspace: Workspace) => user.allows('projects.purchase_portfolio.write') || project.project_manager_id === user.id || workspace.is_active && workspace.purchase_manager_id === user.id;
export const assignable = (user: Actor, project: Project) => user.allows('projects.purchase_assignments.write') && (user.allows('projects.purchase_portfolio.write') || project.project_manager_id === user.id) && open(project);
export async function project(db: DB, target: string, lock = false): Promise<Project> {
    const row = (await db.query(`SELECT id,name,project_number,project_manager_id,client_id,is_active,purchase_enabled,lifecycle_status,version FROM projects WHERE id=$1${lock ? ' FOR UPDATE' : ''}`, [uuid(target)])).rows[0];
    if (!row)
        throw missing();
    return row as Project;
}
export async function scope(db: DB, user: Actor, project: Project, workspace: Workspace, permission: string, write: boolean, hold = false) {
    if (!user.allows('projects.purchase_workflow.view') || !user.allows('projects.purchase_workflow.read') || !user.allows(permission) || !readable(user, project, workspace))
        throw forbidden();
    const canWrite = executable(user, project, workspace) && (hold ? project.is_active && project.lifecycle_status !== 'closed' : open(project));
    if (write) {
        if (!hold && !project.purchase_enabled)
            invalid('purchase_enabled', 'Purchase FMS is not enabled for this project');
        if (!canWrite)
            throw forbidden();
        const client = (await db.query('SELECT is_active FROM clients WHERE id=$1', [project.client_id])).rows[0];
        if (!client?.is_active)
            invalid('client_id', 'Project client is inactive');
    }
    return { can_write: canWrite, project };
}
