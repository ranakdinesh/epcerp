import 'server-only';
import { randomUUID } from 'node:crypto';
import { transaction } from '../db';
import { assignmentIdentities, assignmentIdentity } from '../auth/access';
import { lower } from '../tenders/intake/receipts';
import { initializeProjectPurchase } from '../purchasing/project-registration';
import { access,portfolio,assigned,requireRead,correct } from './access';
import { id,projectName,text,version,capture,filter,ProjectError,invalid } from './validation';
import { validate,calculate } from './schedule';
import * as repo from './repository';
const canCreate=['projects.master.write','projects.portfolio.write','projects.purchase_assignments.write'];
export async function list(token:string,params:URLSearchParams){return transaction(async db=>{
    const a=await access(db,token),q=filter(params),ids=portfolio(a)?[]:await assigned(db,a),search=q.search?'%'+lower(q.search).replace(/[\\%_]/g,c=>'\\'+c)+'%':'';
    const values=[portfolio(a),a.id,ids,search],where=`($1::boolean OR project_manager_id=$2 OR id=ANY($3::uuid[])) AND ($4::text='' OR lower(name) LIKE $4 OR lower(project_number) LIKE $4)`;
    const rows=(await db.query(`SELECT p.*,${repo.exact} FROM projects p WHERE ${where} ORDER BY p.created_at DESC,p.id LIMIT $5 OFFSET $6`,[...values,q.size,(q.page-1)*q.size])).rows;
    const total=Number((await db.query(`SELECT count(*) FROM projects WHERE ${where}`,values)).rows[0].count),items=[];
    for(const r of rows)items.push(await repo.owner(db,repo.view(r)));
    return {items,total,page:q.page,page_size:q.size};
});}
export async function get(token:string,target:string){return transaction(async db=>{const a=await access(db,token),p=await repo.get(db,id(target));await requireRead(db,a,p);return repo.owner(db,p);});}
export async function options(token:string){return transaction(async db=>{
    const a=await access(db,token);
    if(!(a.allows('projects.master.write')&&a.allows('projects.portfolio.write'))&&!a.allows('projects.manager_assignments.write'))throw new ProjectError(403,'project access denied');
    const users=await assignmentIdentities(db,[]),managers=users.filter(u=>u.allows('projects.master.read')&&u.allows('projects.master.write')).map(u=>({id:u.id,name:u.name})),procurement_executives=users.filter(u=>u.allows('projects.purchase_workflow.view')&&u.allows('projects.purchase_workflow.read')).map(u=>({id:u.id,name:u.name}));
    const clients=a.allows('projects.master.write')&&a.allows('projects.portfolio.write')?(await db.query('SELECT id,display_name AS name,client_code AS code FROM clients WHERE is_active ORDER BY display_name')).rows:[];
    return {managers,clients,procurement_executives};
});}
export async function create(token:string,b:Record<string,unknown>){return transaction(async db=>{
    const a=await access(db,token,canCreate),name=projectName(b.name),client=id(b.client_id,'client_id'),manager=id(b.project_manager_id,'project_manager_id'),procurement=typeof b.procurement_executive_id==='string'&&b.procurement_executive_id.trim()?id(b.procurement_executive_id,'procurement_executive_id'):null;
    let c=validate(capture(b),'create',name);
    const row=(await db.query('SELECT is_active FROM clients WHERE id=$1 FOR SHARE',[client])).rows[0];if(!row?.is_active)invalid('client_id','Select an active client');
    const user=await assignmentIdentity(db,manager);if(!user?.active||!user.allows('projects.master.read')||!user.allows('projects.master.write'))invalid('project_manager_id','Select an active manager with project read/write permissions');
    c=await calculate(db,c);
    const project=randomUUID(),columns=repo.captureColumns(c),values=[project,name,lower(name),client,manager,a.id,...columns.map(c=>c.value)];
    const r=(await db.query(`WITH number AS(SELECT nextval('project_number_sequence') AS value) INSERT INTO projects(id,project_number,name,normalized_name,client_id,project_manager_id,created_by_id,updated_by_id,${columns.map(c=>c.key).join(',')}) SELECT $1,'PRJ-'||CASE WHEN value<1000000 THEN lpad(value::text,6,'0') ELSE value::text END,$2,$3,$4,$5,$6,$6,${columns.map((c,i)=>'$'+(i+7)+'::'+c.cast).join(',')} FROM number RETURNING id`,values)).rows[0];
    const saved=await repo.get(db,r.id);await repo.assignment(db,saved,null,'Initial project manager assignment',a.id);await initializeProjectPurchase(db,saved.id,procurement,a.id);return repo.owner(db,saved);
});}
export async function transfer(token:string,target:string,b:Record<string,unknown>){return transaction(async db=>{
    const a=await access(db,token,['projects.manager_assignments.write']),project=id(target),manager=id(b.project_manager_id,'project_manager_id'),reason=text(b.reason,'reason',2000,true),expected=version(b.expected_version),before=await repo.get(db,project,true);
    if(before.version!==expected)throw new ProjectError(409,'project changed; refresh before saving');
    if(before.project_manager_id===manager)invalid('project_manager_id','Choose a different project manager');
    const user=await assignmentIdentity(db,manager);if(!user?.active||!user.allows('projects.master.read')||!user.allows('projects.master.write'))invalid('project_manager_id','Select an active manager with project read/write permissions');
    await db.query('UPDATE projects SET project_manager_id=$2,version=version+1,updated_at=clock_timestamp(),updated_by_id=$3 WHERE id=$1',[project,manager,a.id]);const saved=await repo.get(db,project);await repo.assignment(db,saved,before.project_manager_id,reason,a.id);return repo.owner(db,saved);
});}
export async function managerHistory(token:string,target:string){return transaction(async db=>{
    const a=await access(db,token),p=await repo.get(db,id(target));await requireRead(db,a,p);
    return (await db.query(`SELECT id,project_id,previous_manager_id,project_manager_id,project_version,reason,created_by_id,${repoTime('created_at')} AS created_at FROM project_manager_history WHERE project_id=$1 ORDER BY project_version DESC`,[p.id])).rows.map(r=>({...r,created_at:timestamp(r.created_at)}));
});}
import { timeSQL as repoTime,timestamp } from '../inbox/repository';
export async function update(token:string,target:string,b:Record<string,unknown>){return transaction(async db=>{
    const a=await access(db,token,['projects.master.write']),project=id(target),name=projectName(b.name),client=id(b.client_id,'client_id'),reason=text(b.reason,'reason',2000,true),expected=version(b.expected_version);
    let c=validate(capture(b),'draft');const before=await repo.get(db,project,true);
    if(!correct(a,before))throw new ProjectError(403,'project access denied');if(before.version!==expected)throw new ProjectError(409,'project changed; refresh before saving');
    if(!before.is_active||before.lifecycle_status!=='draft')invalid('','Only active draft project masters can be corrected');
    const row=(await db.query('SELECT is_active FROM clients WHERE id=$1 FOR SHARE',[client])).rows[0];if(!row||!row.is_active&&client!==before.client_id)invalid('client_id','Select an active client');
    c=await calculate(db,c);c.operations_manager_id=before.operations_manager_id;
    const columns=repo.captureColumns(c),values=[project,name,lower(name),client,a.id,...columns.map(c=>c.value)];
    try { await db.query(`UPDATE projects SET name=$2,normalized_name=$3,client_id=$4,updated_by_id=$5,updated_at=clock_timestamp(),version=version+1,${columns.map((c,i)=>c.key+'=$'+(i+6)+'::'+c.cast).join(',')} WHERE id=$1`,values); } catch(e) { if((e as {code?:string}).code==='23505')invalid('name','A project with this name already exists');throw e; }
    const saved=await repo.get(db,project),event=randomUUID();
    await db.query('INSERT INTO project_metadata_changes(id,project_id,project_version,reason,before_snapshot,after_snapshot,created_by_id)VALUES($1,$2,$3,$4,$5,$6,$7)',[event,project,saved.version,reason,repo.snapshot(before,lower(before.name)),repo.snapshot(saved,lower(saved.name)),a.id]);
    await repo.journal(db,event,saved,'metadata_updated',a.id);return repo.owner(db,saved);
});}
export async function metadataHistory(token:string,target:string){return transaction(async db=>{const a=await access(db,token),p=await repo.get(db,id(target));await requireRead(db,a,p);return (await db.query(`SELECT *,${repoTime('created_at')} AS created_at FROM project_metadata_changes WHERE project_id=$1 ORDER BY project_version DESC`,[p.id])).rows.map(r=>({id:r.id,project_version:r.project_version,reason:r.reason,created_by_id:r.created_by_id??'',created_at:timestamp(r.created_at),before:repo.fromSnapshot(r.before_snapshot),after:repo.fromSnapshot(r.after_snapshot)}));});}
export async function metadataOptions(token:string,target:string){return transaction(async db=>{const a=await access(db,token),p=await repo.get(db,id(target));if(!correct(a,p))throw new ProjectError(403,'project access denied');const clients=(await db.query('SELECT id,display_name AS name FROM clients WHERE is_active ORDER BY display_name')).rows;if(!clients.some(c=>c.id===p.client_id)){const current=(await db.query('SELECT id,display_name AS name FROM clients WHERE id=$1',[p.client_id])).rows[0];if(current)clients.push(current);}return {clients};});}
export async function preview(token:string,b:Record<string,unknown>,target?:string){return transaction(async db=>{const a=await access(db,token,target?[]:['projects.master.write','projects.portfolio.write']);if(target){const p=await repo.get(db,id(target));if(!correct(a,p))throw new ProjectError(403,'project access denied');}const c=validate(capture(b),target?'draft':'schedule');return (await calculate(db,c)).schedule_calculation;});}
