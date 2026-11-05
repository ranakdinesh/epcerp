import { invalid,object,uuid,version,string,trim,isoDate } from '../core/validation';
import type { Command,Terms } from './types';
import type { Vendor } from '../groups/types';
export function input(raw:Record<string,unknown>):Command{
    const b=object(raw,['action','group_id','vendor_id','document_id','version','group_version','bo_version','sent_date','channel','dispatch_reference']),action=string(b.action,'action','');
    const group_id=b.group_id==null?null:uuid(b.group_id),vendor_id=b.vendor_id==null?null:uuid(b.vendor_id),document_id=b.document_id==null?null:uuid(b.document_id);
    if(action==='generate'){if(!group_id||!vendor_id)invalid('group_id','Select group and vendor');}else if(['print','send'].includes(action)){if(!document_id)invalid('document_id','Select a saved vendor indent');}else invalid('action','Unknown indent action');
    const sent_date=b.sent_date==null?null:string(b.sent_date,'sent_date'),channel=trim(string(b.channel,'channel','')),dispatch_reference=trim(string(b.dispatch_reference,'dispatch_reference',''));
    if([...channel].length>80||[...dispatch_reference].length>500)invalid('dispatch_reference','Dispatch details exceed permitted length');
    if(action==='send'&&(!sent_date||!isoDate(sent_date)||!channel||!dispatch_reference))invalid('sent_date','Record a valid actual sent date, channel and reference');
    return{action,group_id,vendor_id,document_id,sent_date,channel,dispatch_reference,version:version(b.version??0),group_version:version(b.group_version??0,'group_version'),bo_version:version(b.bo_version??0,'bo_version')};
}
export function terms(vendor:Vendor):Terms{
    const values:Record<string,string>={};
    for(const[key,max]of Object.entries({indent_notes:4000,indent_terms:12000,delivery_terms:2000})){const raw=vendor.data[key];if(raw===undefined)continue;if(typeof raw!=='string'||[...raw].length>max)invalid('vendor_id','Vendor indent terms are invalid; correct them in Vendors / Suppliers');values[key]=trim(raw);}
    if(!values.indent_terms||!values.delivery_terms)invalid('vendor_id',"Configure this vendor's purchase indent conditions and delivery basis in Vendors / Suppliers before generating an indent");
    const lines=(raw:string='')=>raw.replaceAll('\r\n','\n').split('\n').map(trim).filter(Boolean);
    return{source:'vendor_master',notes:lines(values.indent_notes),conditions:lines(values.indent_terms),delivery_basis:values.delivery_terms};
}
