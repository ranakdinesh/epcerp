import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';
import { pool } from '../../src/server/db';
import { handleMaster } from '../../src/server/master/http';
import { definition } from '../server-master/catalogue-fixtures';
import { project } from '../server-inbox/fixtures';
export async function workFixture(token:string,actor:string,other:string){
    const sql=pool(),p=await project(actor),prefix='QA'+randomUUID().replaceAll('-','').slice(0,9).toUpperCase();
    const call=async(path:string,body:unknown)=>{const r=await handleMaster(new Request('http://localhost/api/v1'+path,{method:'POST',headers:{Authorization:'Bearer '+token},body:JSON.stringify(body)}));const v=await r.json();assert.ok(r.ok,JSON.stringify(v));return v;};
    const unit=await call('/categories/units',{code:prefix,name:prefix,reason:'Synthetic unit'}),category=await call('/categories/categories',{code:prefix+'C',name:prefix,unit_id:unit.id,reason:'Synthetic category'}),sub=await call('/categories/subcategories',{code:prefix+'T',name:prefix,category_id:category.id,reason:'Synthetic type'}),template=await call('/configuration-templates/'+sub.id+'/publish',{definition:{...definition(),subcategory_code:sub.code,template_name:prefix},expected_revision:0,reason:'Synthetic configuration'}),make=await call('/manufacturers',{code:prefix,name:prefix,data:{eligible:[category.code]},reason:'Synthetic make'}),item=await call('/catalogue-items',{configuration_id:template.id,manufacturer_id:make.id,specifications:{SIZE:'S1'},model:prefix});
    await sql.query("UPDATE projects SET purchase_enabled=true WHERE id=$1",[p]);
    const client=(await sql.query('SELECT client_id FROM projects WHERE id=$1',[p])).rows[0].client_id;
    const demand=randomUUID(),indent=randomUUID(),pkg=randomUUID(),bo=randomUUID(),group=randomUUID();
    await sql.query("INSERT INTO project_material_requirements(id,project_id,item_id,engineering_key,item_snapshot,quantity,required_date,allocations,alternatives,is_active,version)VALUES($1,$2,$3,$4,$5,1,'2026-10-10','[]','[]',true,1)",[demand,p,item.id,item.engineering_key,{}]);
    const released={demand_id:demand,quantity:'1',full_kit:'Synthetic ready',source:{demand_version:1,item:{description:item.description,uom:'EA'},required_date:'2026-10-10'},at:'2026-01-01T00:00:00Z'};
    await sql.query('INSERT INTO purchase_indents(id,project_id,doer_id,resolver_id,data,is_active,version)VALUES($1,$2,$3,$4,$5,true,1)',[indent,p,actor,other,{number:prefix,released:[released]}]);
    await sql.query("INSERT INTO purchase_indent_releases(indent_id,demand_id,quantity,snapshot)VALUES($1,$2,1,'{}')",[indent,demand]);
    const steps=Array.from({length:20},(_,i)=>({number:i+1,title:'Task '+(i+1),phase:'preparation',evidence_required:'Review',doer_id:i===2||i===3?actor:null,resolver_id:other,planned_date:'2026-10-10',state:i<2?'source_linked':i===3?'blocked':'not_started',block_reason:i===3?'Waiting for review':'',blocked_resolver_id:i===3?other:null,resolution_due:i===3?'2026-10-11':null}));
    await sql.query('INSERT INTO purchase_work_packages(id,project_id,indent_id,demand_id,code,description,source_snapshot,steps,version)VALUES($1,$2,$3,$4,$5,$6,$7,$8,1)',[pkg,p,indent,demand,prefix,'Synthetic 100%_package',{},JSON.stringify(steps)]);
    await sql.query("INSERT INTO purchase_bo_lists(id,project_id,preparer_id,name,state,is_active,version)VALUES($1,$2,$3,'Synthetic BO','draft',true,1)",[bo,p,actor]);
    await sql.query("INSERT INTO purchase_indent_group_sets(bo_id,project_id,version,data,is_active)VALUES($1,$2,1,'{}',true)",[bo,p]);
    await sql.query("INSERT INTO purchase_indent_group_entries(id,bo_id,name,scope_revision,data,is_active)VALUES($1,$2,'Synthetic group',1,'{}',true)",[group,bo]);
    const quotation=async(recipient:string,kind:string)=>{const id=randomUUID();await sql.query("INSERT INTO purchase_quotation_work_tasks(id,project_id,bo_id,group_id,attempt_id,recipient_id,kind,title,link,state)VALUES($1,$2,$3,$4,$5,$6,$7,'Synthetic quotation','/dashboard?screen=quotations','pending')",[id,p,bo,group,randomUUID(),recipient,kind]);return id;};
    const alternative=async()=>{const otherMake=await call('/manufacturers',{code:prefix+'A',name:prefix+' alternative',data:{eligible:[category.code]},reason:'Synthetic alternative'});return call('/catalogue-items',{configuration_id:template.id,manufacturer_id:otherMake.id,specifications:{SIZE:'S1'},model:prefix+' alternative'});};
    return {project:p,client,demand,indent,pkg,item,make,category,unit,sub,quotation,alternative};
}
