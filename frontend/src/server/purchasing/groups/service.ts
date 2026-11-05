import 'server-only';
import type { DB } from '../../db';
import { transaction } from '../../db';
import { timestamp } from '../../inbox/repository';
import { actor,project,scope,open,type Actor,type Project } from '../core/access';
import { workspace as purchaseWorkspace } from '../workspace/repository';
import { purchaseVendors } from '../../master/vendors/purchase-reader';
import { get as getBO } from '../bo/repository';
import { PurchaseError,invalid,uuid } from '../core/validation';
import { indiaDate } from '../../personal-tasks/validation';
import { candidates,report } from './report';
import { input,distinct,subset,sameSet,ordered,validateGroups } from './validation';
import type { Group,GroupRecord,Command,Line } from './types';
import * as repo from './repository';
const missing=()=>new PurchaseError(404,'Purchase group not found');
const denied=()=>new PurchaseError(403,'Purchase group permission or project scope denied');
async function source(db:DB,p:Project,bo:string){
    const record=await getBO(db,p.id,bo);if(!record?.is_active)throw missing();const client=(await db.query('SELECT is_active FROM clients WHERE id=$1',[p.client_id])).rows[0];
    const lines:Line[]=record.lines.map(line=>({...line,item_snapshot:{id:line.item_snapshot.id,sku:line.item_snapshot.sku,description:line.item_snapshot.description,engineering_key:line.item_snapshot.engineering_key,uom:line.item_snapshot.uom,manufacturer:line.item_snapshot.manufacturer}}));
    return{version:record.version,state:record.state,lines,available:open(p)&&p.purchase_enabled&&!!client?.is_active};
}
const groupLines=(groups:Group[])=>groups.flatMap(g=>g.line_ids);
const groupVendors=(groups:Group[])=>groups.flatMap(g=>g.vendor_ids);
async function workspaceBound(db:DB,a:Actor,p:Project,bo:string,write:boolean){
    const s=await source(db,p,bo),record=await repo.get(db,p.id,bo),vendors=candidates(record,await purchaseVendors(db));
    return{record,vendors,can_write:write&&s.available&&a.allows('projects.purchase_indent_groups.write'),can_manage_vendors:write&&s.available&&a.allows('projects.purchase_group_vendors.write'),history:record?await repo.history(db,p.id,bo):[],item_report:report(record,s.lines,vendors)};
}
export async function workspace(token:string,target:string,bo:string){return transaction(async db=>{
    const a=await actor(db,token,['projects.purchase_bo.read','projects.purchase_indent_groups.read']);
    if(!['projects.purchase_indent_groups.view','projects.purchase_group_vendors.view','projects.purchase_item_vendor_report.view','projects.purchase_indent_register.view','projects.purchase_indent_documents.view'].some(code=>a.allows(code)))throw denied();
    const p=await project(db,target,true),w=await purchaseWorkspace(db,p.id),access=await scope(db,a,p,w,'projects.purchase_indent_groups.read',false);
    return workspaceBound(db,a,p,uuid(bo),access.can_write);
});}
export async function execute(token:string,target:string,bo:string,raw:Record<string,unknown>){return transaction(async db=>{const a=await actor(db,token,['projects.purchase_bo.read','projects.purchase_indent_groups.read']);return executeBound(db,a,target,uuid(bo),input(raw));});}
// Document generation/dispatch joins this command in the same transaction.
export async function executeBound(db:DB,a:Actor,target:string,bo:string,b:Command){
    if(!a.allows('projects.purchase_bo.read')||!a.allows('projects.purchase_indent_groups.read'))throw denied();
    let permission='projects.purchase_indent_groups.write';if(['set_vendors','add_vendors'].includes(b.action))permission='projects.purchase_group_vendors.write';if(b.action==='invite')permission='projects.purchase_indent_documents.dispatch';
    const issueOnly=b.action==='create'&&!a.allows(permission)&&a.allows('projects.purchase_indent_documents.generate');if(issueOnly)permission='projects.purchase_indent_documents.generate';if(!a.allows(permission))throw denied();
    const p=await project(db,target,true),w=await purchaseWorkspace(db,p.id),access=await scope(db,a,p,w,permission,true),s=await source(db,p,bo);
    if(!s.available)invalid('','Project purchasing is unavailable, closed or on hold');
    const before=await repo.get(db,p.id,bo),record:GroupRecord=before?structuredClone(before):{bo_id:bo,project_id:p.id,state:'draft',is_active:true,version:0,bo_version:0,groups:[],source_lines:[],source_vendors:[],selected_line_ids:[],number:'',created_at:'0001-01-01T00:00:00Z',updated_at:'0001-01-01T00:00:00Z',created_by_id:null,updated_by_id:null};
    if(b.version!==record.version)throw repo.stale();
    const current=await purchaseVendors(db),vendors=current.map(v=>v.id),lines=s.lines.map(l=>l.id),find=(id:string|null)=>record.groups.find(g=>g.id===id),fresh=()=>b.groups.map(g=>({...g,invitations:[],scope_revision:1} as Group));
    if(['save','create','save_groups'].includes(b.action)&&b.groups.length){
        for(const g of b.groups){const old=find(g.id),changed=!old&&g.vendor_ids.length>0||!!old&&!ordered(old.vendor_ids,g.vendor_ids);if(changed&&!a.allows('projects.purchase_group_vendors.write'))throw denied();}
        if(issueOnly&&(b.groups.length!==record.groups.length||b.groups.some((g,index)=>{const old=record.groups[index];return !old||g.id!==old.id||g.name!==old.name||!ordered(g.line_ids,old.line_ids)||!ordered(g.vendor_ids,old.vendor_ids);})))throw denied();
    }
    const issued=record.state==='created';
    switch(b.action){
        case 'create_group':case 'add_items':case 'reset_item':{
            if(issued&&b.action==='reset_item')invalid('','Issued items cannot be reset; append new items instead');if(issued&&s.state!=='completed')invalid('','Complete the updated BO before adding items');if(b.bo_version!==s.version)throw repo.stale();let additions:string[]=[];
            if(b.action==='create_group'){
                if(b.groups.length!==1||b.groups[0].vendor_ids.length)invalid('groups','Save one item group before choosing its vendors');if(record.groups.length>=100)invalid('groups','At most 100 item groups are supported');const g=fresh()[0];validateGroups([...record.groups,g],[...lines,...record.source_lines.map(l=>l.id)],[...vendors,...groupVendors(record.groups)],false);record.groups.push(g);additions=g.line_ids;
            }else if(b.action==='add_items'){
                const g=find(b.group_id);if(!g)throw missing();additions=b.line_ids;if(!additions.length||!distinct(additions)||!subset(additions,lines))invalid('line_ids','Select distinct items from the current BO list');const assigned=groupLines(record.groups);if(additions.some(id=>assigned.includes(id)))invalid('line_ids','Selected items already belong to a group');if(g.line_ids.length+additions.length>100)invalid('line_ids','A group supports at most 100 items');if(issued)g.scope_revision++;g.line_ids.push(...additions);
            }else{
                if(!b.line_id)invalid('line_id','Select a grouped item');let found=false;record.groups=record.groups.map(g=>{if(g.line_ids.includes(b.line_id!))found=true;return{...g,line_ids:g.line_ids.filter(id=>id!==b.line_id)};}).filter(g=>g.line_ids.length);if(!found)throw missing();
            }
            record.selected_line_ids=groupLines(record.groups);record.bo_version=s.version;
            if(issued){const frozen=record.source_lines.map(l=>l.id);for(const l of s.lines)if(additions.includes(l.id)&&!frozen.includes(l.id))record.source_lines.push(l);}else record.state=record.groups.length?'grouped':'draft';
            break;
        }
        case 'select_items':{
            if(issued)invalid('','Created item selection is immutable');if(b.bo_version!==s.version)throw repo.stale();const ids=b.selected_line_ids;if(!ids.length||!distinct(ids)||!subset(ids,lines))invalid('selected_line_ids','Select distinct items from the current BO list');if(!subset(groupLines(record.groups),ids))invalid('selected_line_ids','Remove items from saved groups before deselecting them');record.selected_line_ids=ids;record.bo_version=s.version;record.state='draft';break;
        }
        case 'save_groups':{
            if(issued)invalid('','Created group scope is immutable');if(b.bo_version!==s.version)throw repo.stale();if(!record.selected_line_ids.length)invalid('','Save the purchase item selection first');const groups=fresh();validateGroups(groups,record.selected_line_ids,vendors,false);if(!groups.length||!sameSet(groupLines(groups),record.selected_line_ids))invalid('groups','Assign every selected purchase item to a group');
            for(const g of groups){const old=find(g.id);if(g.vendor_ids.length&&(!old||!sameSet(old.line_ids,g.line_ids)||!ordered(old.vendor_ids,g.vendor_ids)))invalid('vendor_ids','Save groups first, then select vendors separately');}
            record.groups=groups;record.state='grouped';record.bo_version=s.version;break;
        }
        case 'set_vendors':case 'add_vendors':{
            const g=find(b.group_id);if(!g)throw missing();const ids=b.vendor_ids;if(!distinct(ids)||!subset(ids,vendors))invalid('vendor_ids','Choose distinct active vendors');
            if(b.action==='set_vendors'){if(record.state!=='grouped'&&!issued)invalid('','Save all named groups before selecting vendors');if(b.bo_version!==s.version)throw repo.stale();if(issued&&g.vendor_ids.length)invalid('','Issued vendor selections are retained; add vendors instead');if(ids.length<3)invalid('vendor_ids','Select at least three distinct active vendors');g.vendor_ids=[...ids];}
            else{if(!issued)invalid('','Create grouped indents before adding later vendors');if(!ids.length)invalid('vendor_ids','Choose vendors to add');if(ids.some(id=>g.vendor_ids.includes(id)))invalid('vendor_ids','Vendor already belongs to this group');if(g.vendor_ids.length+ids.length>50)invalid('vendor_ids','A group supports at most 50 vendors');g.vendor_ids.push(...ids);}
            if(issued){const saved=record.source_vendors.map(v=>v.id);for(const v of current)if(ids.includes(v.id)&&!saved.includes(v.id))record.source_vendors.push(v);}break;
        }
        case 'invite':{
            if(!issued)invalid('','Create grouped indents before recording an invitation');const g=find(b.group_id);if(!g||!b.vendor_id||!g.vendor_ids.includes(b.vendor_id))invalid('vendor_id','Select a vendor belonging to this group');if(!vendors.includes(b.vendor_id))invalid('vendor_id','Only active vendors can be invited');
            const revision=b.invitation_scope_revision??g.scope_revision,coverage=b.invitation_line_ids??g.line_ids;if(revision<1||revision>g.scope_revision||!coverage.length||!subset(coverage,g.line_ids))invalid('','Invalid invitation scope');if(g.invitations.some(i=>i.vendor_id===b.vendor_id&&i.scope_revision===revision))invalid('vendor_id','Invitation is already recorded for this group and vendor');if(!b.sent_date||b.sent_date>indiaDate(new Date())||!b.channel||!b.dispatch_reference)invalid('sent_date','Record a dispatch date, channel and reference; future dates are not allowed');
            g.invitations.push({vendor_id:b.vendor_id,scope_revision:revision,line_ids:[...coverage],sent_date:b.sent_date,channel:b.channel,dispatch_reference:b.dispatch_reference,recorded_by_id:a.id,recorded_at:timestamp(new Date().toISOString())!});break;
        }
        case 'save':case 'create':{
            if(issued)invalid('','Created group scope is immutable; add vendors instead');if(b.bo_version!==s.version)throw repo.stale();const groups=fresh(),selection=record.selected_line_ids.length?record.selected_line_ids:lines;validateGroups(groups,selection,vendors,b.action==='create');if(b.action==='create'&&s.state!=='completed')invalid('','Complete the BO list before creating grouped indents');record.groups=groups;record.bo_version=s.version;if(!record.selected_line_ids.length)record.selected_line_ids=[...lines];record.state='draft';
            if(b.action==='create'){record.state='created';record.source_lines=s.lines.filter(l=>record.selected_line_ids.includes(l.id));record.number='GI-'+bo.replaceAll('-','').slice(0,12).toUpperCase();const selected=groupVendors(groups);record.source_vendors=current.filter(v=>selected.includes(v.id));}break;
        }
        default:invalid('action','Unknown group action');
    }
    record.version++;
    if(before)for(const id of await repo.pending(db,p.id,bo)){const old=before.groups.find(g=>g.id===id),current=record.groups.find(g=>g.id===id);if(!old||!current||old.name!==current.name||old.scope_revision!==current.scope_revision||!ordered(old.line_ids,current.line_ids)||!ordered(old.vendor_ids,current.vendor_ids))invalid('groups',"Waiting for Finance approval; the submitted group's items and vendors are frozen");}
    const saved=await repo.save(db,record,a.id);await repo.event(db,before,saved,b.action,b.reason,a.id);return workspaceBound(db,a,p,bo,access.can_write);
}
