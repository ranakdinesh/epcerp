import 'server-only';
import { randomUUID } from 'node:crypto';
import { transaction,type DB } from '../../db';
import { actor,project,scope } from '../core/access';
import { workspace } from '../workspace/repository';
import { jsonStore } from '../core/json-store';
import { PurchaseError,invalid,uuid,object,requiredText,version,trim,string } from '../core/validation';
import { now,instant } from '../core/time';
import { assignmentIdentity } from '../../auth/access';
import { purchaseCategory } from '../../master/categories/purchase-reader';
import { demand } from '../materials/repository';
import type { Data } from './evidence';
const store=jsonStore<Data>({table:'purchase_holds',events:'purchase_hold_events',module:'purchase_holds',type:'purchase_hold',stale:'Purchase hold or linked material requirement changed; reload the saved version',columns:d=>({doer_id:d.doer_id,resolver_id:d.resolver_id})});
async function access(db:DB,token:string,target:string,permission:string,write:boolean){const a=await actor(db,token,[permission]),p=await project(db,target,true),w=await workspace(db,p.id);await scope(db,a,p,w,permission,write,true);return{a,p};}
export async function list(token:string,target:string){return transaction(async db=>{const{p}=await access(db,token,target,'projects.purchase_holds.read',false);return store.list(db,p.id);});}
export async function history(token:string,target:string,id:string){return transaction(async db=>{const{p}=await access(db,token,target,'projects.purchase_holds.read',false);id=uuid(id);if(!await store.get(db,p.id,id))throw new PurchaseError(404,'Purchase hold not found');return store.history(db,p.id,id);});}
export async function create(token:string,target:string,raw:Record<string,unknown>){return transaction(async db=>{
    const{a,p}=await access(db,token,target,'projects.purchase_holds.write',true),b=object(raw,['scope','category_code','demand_ids','reason','evidence','started_at','expected_release_at','doer_id','resolver_id']);
    const started_at=instant(b.started_at,'started_at',true),expected_release_at=b.expected_release_at==null?null:instant(b.expected_release_at,'expected_release_at');
    if(Date.parse(started_at)>Date.now())invalid('started_at','Use a valid hold start that is not in the future');if(expected_release_at&&Date.parse(expected_release_at)<Date.parse(started_at))invalid('expected_release_at','Expected release must not precede the hold start');
    const category_code=trim(string(b.category_code,'category_code','')),scope=string(b.scope,'scope','');if([...category_code].length>40)invalid('category_code','Category code exceeds 40 characters');if(!['project','category','items'].includes(scope))invalid('scope','Choose project, category or items');if((scope==='category')!==!!category_code)invalid('category_code','Category is required only for category holds');
    if(b.demand_ids!=null&&!Array.isArray(b.demand_ids))invalid('demand_ids','Select up to 200 requirements only for item holds');const demand_ids=(b.demand_ids as unknown[]??[]).map(id=>uuid(id));if(scope==='items'&&!demand_ids.length||scope!=='items'&&demand_ids.length||demand_ids.length>200)invalid('demand_ids','Select up to 200 requirements only for item holds');if(new Set(demand_ids).size!==demand_ids.length)invalid('demand_ids','Select distinct requirements');
    const data:Data={scope,category_code,demand_ids,reason:requiredText(b.reason,'reason'),evidence:requiredText(b.evidence,'evidence'),started_at,expected_release_at,doer_id:uuid(b.doer_id,'doer_id','Select an active doer'),resolver_id:uuid(b.resolver_id,'resolver_id','Select an active resolver'),recorded_at:now(),released_at:null,release_evidence:null};
    if(scope==='category'&&!await purchaseCategory(db,category_code))invalid('category_code','Select an active category');for(const id of demand_ids)if(!await demand(db,p.id,id))invalid('demand_ids','Select requirements from this project');for(const id of [data.doer_id,data.resolver_id])if(!(await assignmentIdentity(db,id))?.active)invalid('doer_id','Doer and resolver must be active users');
    const saved=await store.save(db,{id:randomUUID(),project_id:p.id,data,is_active:true,version:1},a.id);await store.event(db,null,saved,'hold',data.reason,a.id);return saved;
});}
export async function release(token:string,target:string,id:string,raw:Record<string,unknown>){return transaction(async db=>{
    const{a,p}=await access(db,token,target,'projects.purchase_holds.release',true),b=object(raw,['version','reason','evidence']),explanation=requiredText(b.reason,'reason'),evidence=requiredText(b.evidence,'evidence'),expected=version(b.version,'version',1);id=uuid(id);const record=await store.get(db,p.id,id);if(!record)throw new PurchaseError(404,'Purchase hold not found');if(record.version!==expected)throw store.stale();if(!record.is_active)invalid('','Hold already released');const before=structuredClone(record);record.data.released_at=now();record.data.release_evidence=evidence;record.is_active=false;record.version++;const saved=await store.save(db,record,a.id);await store.event(db,before,saved,'release',explanation,a.id);return saved;
});}
