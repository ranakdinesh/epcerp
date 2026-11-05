import type { DB } from '../../db';
import { source as quotationsSource } from '../quotations/source';
import { revisions } from '../quotations/repository';
import { compare } from '../quotations/calculation';
import { get as groups } from '../groups/repository';
import { indiaDate } from '../../personal-tasks/validation';
import { evaluate } from '../../master/verification/domain';
import { VerificationRepository } from '../../master/verification/repository';
import type { Source,Budget } from './types';
export async function source(db:DB,project:string,bo:string):Promise<Source>{
    const current=await quotationsSource(db,project,bo),grouping=await groups(db,project,bo),quoted=compare(await revisions(db,project,bo),current.documents,indiaDate(new Date()),current.currency),verifications:Source['verifications']={};
    const row=(await db.query('SELECT is_active,approved_snapshot FROM project_budgets WHERE project_id=$1',[project])).rows[0],approved=row?.is_active?row.approved_snapshot:null;
    const budget:Budget|null=approved?{revision:approved.revision,currency:approved.data.currency,decision_id:approved.decision_id,allocations:approved.data.allocations.filter((a:{is_active:boolean})=>a.is_active).map((a:{bo_id:string;line_id:string;amount:string})=>({bo_id:a.bo_id,line_id:a.line_id,amount:a.amount}))}:null;
    const quotes:Source['quotes']=[];
    for(const q of quoted){
        quotes.push({lowest:q.suggested,terms:`Delivery: ${q.delivery_days} days; ${q.delivery_terms}. Payment: ${q.payment_terms}`,valid_until:q.valid_until,key:q.document_id+':'+q.revision,document_id:q.document_id,group_id:q.group_id,group_name:q.group_name,vendor_id:q.vendor_id,vendor:q.vendor,reference:q.reference,total:q.total,revision:q.revision,scope_revision:q.scope_revision,eligible:q.eligible,exclusions:[...q.exclusions],lines:q.lines.map(l=>({coverage:l.coverage_type,months:l.warranty_months,notes:l.notes,price:{discount_percent:l.discount_pct,tax_percent:l.tax_pct,pricing_mode:l.pricing_mode||'unit_price',unit_price:l.unit_price??'',base:l.base??'',discount:l.discount_amount??'',tax:l.tax_amount??'',allocated_charges:l.allocated_charges??''},line_id:l.line_id,quantity:l.quantity,quoted:l.landed??'',item:{id:l.source.item_snapshot.id,sku:l.source.item_snapshot.sku,description:l.source.item_snapshot.description,uom:l.source.item_snapshot.uom,manufacturer:l.source.item_snapshot.manufacturer,engineering_key:l.source.item_snapshot.engineering_key}}))});
        if(!verifications[q.vendor_id]){const repo=new VerificationRepository(db),status=evaluate(await repo.snapshot(q.vendor_id),await repo.review(q.vendor_id));verifications[q.vendor_id]={state:status.state,fingerprint:status.fingerprint,message:status.message};}
    }
    return{project_id:project,bo_id:bo,currency:current.currency,available:current.available,groups:grouping?.is_active?grouping.groups.map(g=>({id:g.id,name:g.name,scope_revision:g.scope_revision})):[],quotes,budget,verifications};
}
