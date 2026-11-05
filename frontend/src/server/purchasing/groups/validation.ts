import { invalid,uuid,version,reason,object,string,trim,isoDate } from '../core/validation';
import { nameKey } from '../core/name-key';
import type { Command,Group,GroupInput } from './types';
export const distinct=(ids:string[])=>new Set(ids).size===ids.length;
export const subset=(ids:string[],allowed:string[])=>ids.every(id=>allowed.includes(id));
export const sameSet=(a:string[],b:string[])=>a.length===b.length&&subset(a,b)&&subset(b,a);
export const ordered=(a:string[],b:string[])=>a.length===b.length&&a.every((id,index)=>b[index]===id);
function ids(raw:unknown,max:number){if(raw==null)return[];if(!Array.isArray(raw)||raw.length>max)invalid('','Too many selected records');return raw.map(id=>uuid(id));}
export function input(raw:Record<string,unknown>):Command{
    const b=object(raw,['version','bo_version','action','reason','groups','group_id','line_id','line_ids','vendor_ids','selected_line_ids','vendor_id','sent_date','channel','dispatch_reference']);
    const action=string(b.action,'action','');if(!['save','create','add_vendors','select_items','save_groups','set_vendors','invite','create_group','add_items','reset_item'].includes(action))invalid('action','Unknown group action');
    const groups:GroupInput[]=[];if(b.groups!=null&&!Array.isArray(b.groups))invalid('groups','Use at most 100 item groups');for(const raw of b.groups as unknown[]??[]){const g=object(raw,['id','name','line_ids','vendor_ids']),name=trim(string(g.name,'name',''));if(!name||[...name].length>128)invalid('name','Group name must contain 1–128 characters');const line_ids=ids(g.line_ids,100);if(!line_ids.length)invalid('line_ids','Each group needs items');groups.push({id:uuid(g.id),name,line_ids,vendor_ids:ids(g.vendor_ids,50)});}
    if(groups.length>100)invalid('groups','Use at most 100 item groups');
    const channel=trim(string(b.channel,'channel','')),dispatch_reference=trim(string(b.dispatch_reference,'dispatch_reference',''));if([...channel].length>80||[...dispatch_reference].length>500)invalid('dispatch_reference','Dispatch details exceed the permitted length');
    const sent_date=b.sent_date==null?null:string(b.sent_date,'sent_date');if(sent_date!==null&&!isoDate(sent_date))invalid('sent_date','Use a valid dispatch date');
    return{version:version(b.version??0),bo_version:version(b.bo_version??0,'bo_version'),action,reason:reason(b.reason,'Record a reason within 2000 characters'),groups,group_id:b.group_id==null?null:uuid(b.group_id),line_id:b.line_id==null?null:uuid(b.line_id),line_ids:ids(b.line_ids,100),vendor_ids:ids(b.vendor_ids,50),selected_line_ids:ids(b.selected_line_ids,100),vendor_id:b.vendor_id==null?null:uuid(b.vendor_id),sent_date,channel,dispatch_reference};
}
export function validateGroups(groups:Group[],lines:string[],vendors:string[],complete:boolean){
    const ids=new Set<string>(),names=new Set<string>(),assigned:string[]=[];
    for(const g of groups){if(ids.has(g.id))invalid('groups','Group identifiers must be distinct');ids.add(g.id);const key=nameKey(g.name);if(!key||names.has(key))invalid('name','Give every group a distinct name');names.add(key);if(!g.line_ids.length||!subset(g.line_ids,lines))invalid('line_ids','Each group needs items from the current BO list');assigned.push(...g.line_ids);if(!distinct(g.vendor_ids)||!subset(g.vendor_ids,vendors))invalid('vendor_ids','Select distinct active vendors');if(complete&&g.vendor_ids.length<3)invalid('vendor_ids','Group '+g.name+' needs at least three active vendors');}
    if(!distinct(assigned))invalid('line_ids','An item may belong to only one group');if(complete&&(!groups.length||!sameSet(assigned,lines)))invalid('groups','Assign every BO item to a group before creating indents');
}
