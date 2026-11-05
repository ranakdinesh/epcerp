import type { DB } from '../../db';
import { project,open } from '../core/access';
import { get as getBO } from '../bo/repository';
import { list as indentDocuments } from '../indent-documents/repository';
import { get as grouping } from '../groups/repository';
import { purchaseVendors } from '../../master/vendors/purchase-reader';
import type { Document } from './types';
import { PurchaseError } from '../core/validation';
export async function source(db:DB,target:string,bo:string){
    const b=await getBO(db,target,bo);if(!b?.is_active)throw new PurchaseError(404,'Quotation or evidence not found in this project BO');
    const p=await project(db,target),client=(await db.query('SELECT is_active FROM clients WHERE id=$1',[p.client_id])).rows[0],groups=await grouping(db,target,bo),rev=new Map(groups?.groups.map(g=>[g.id,g.scope_revision])??[]),vendors=await purchaseVendors(db),docs=await indentDocuments(db,target,bo),documents:Document[]=[];
    for(const d of docs)if(d.is_active){const data:Document['data']={number:d.data.number,group_name:d.data.group_name,scope_revision:d.data.scope_revision,is_current:d.data.scope_revision===rev.get(d.group_id),sent_date:d.data.sent_date,vendor:d.data.vendor,lines:d.data.lines};if(d.data.terms)data.terms=d.data.terms;documents.push({id:d.id,project_id:d.project_id,bo_id:d.bo_id,group_id:d.group_id,vendor_id:d.vendor_id,version:d.version,vendor_available:vendors.some(v=>v.id===d.vendor_id),data});}
    const currency=(await db.query('SELECT currency FROM projects WHERE id=$1',[target])).rows[0].currency as string;
    return{documents,currency,available:open(p)&&p.purchase_enabled&&!!client?.is_active};
}
