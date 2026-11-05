import 'server-only';
import { randomUUID } from 'node:crypto';
import { timestamp } from '../../inbox/repository';
import { transaction,type DB } from '../../db';
import { assignmentIdentities,assignmentIdentity } from '../../auth/access';
import { actor,project,scope } from '../core/access';
import { workspace as purchaseWorkspace } from '../workspace/repository';
import { PurchaseError,invalid,uuid } from '../core/validation';
import { quantity,format } from '../core/decimal';
import { demands } from '../materials/repository';
import { identities,lineSource } from './source';
import { saveInput,actionInput } from './validation';
import { store,save as persist,type Indent } from './repository';
import type { Data,Release } from './types';
const missing=()=>new PurchaseError(404,'Purchase indent not found');
async function access(db:DB,token:string,target:string,codes:string[],write=false){const a=await actor(db,token,codes),p=await project(db,target,true),w=await purchaseWorkspace(db,p.id);await scope(db,a,p,w,codes[0],write);return{a,p,w};}
export async function list(token:string,target:string){return transaction(async db=>{const{p}=await access(db,token,target,['projects.purchase_indents.read']);return store.list(db,p.id);});}
export async function history(token:string,target:string,id:string){return transaction(async db=>{const{p}=await access(db,token,target,['projects.purchase_indents.read']);id=uuid(id);if(!await store.get(db,p.id,id))throw missing();return store.history(db,p.id,id);});}
export async function save(token:string,target:string,id:string|undefined,raw:Record<string,unknown>){return transaction(async db=>{
    const{a,p}=await access(db,token,target,['projects.purchase_indents.write'],true),b=saveInput(raw);if(id)id=uuid(id);await identities(db,b.data);
    let before:Indent|null=null,number='PI-'+randomUUID().replaceAll('-','').slice(0,12).toUpperCase();
    if(id){before=await store.get(db,p.id,id);if(!before)throw missing();if(before.version!==b.version)throw store.stale();if(before.data.state!=='draft')invalid('','Only draft indents can be edited');number=before.data.number;}else if(b.version!==1)throw store.stale();
    for(const line of b.data.lines){const source=await lineSource(db,p.id,line.demand_id);if(!source.active)invalid('demand_id','Select an active project requirement');if(quantity(line.quantity)>quantity(source.quantity))invalid('quantity','Requested quantity exceeds project requirement');}
    const data:Data={...b.data,number,state:'draft',released:[],review:null},record={id:id??randomUUID(),project_id:p.id,data,version:before?before.version+1:1,is_active:true};
    const saved=await persist(db,record,a.id);await store.event(db,before,saved,before?'edit':'create',b.reason,a.id);return saved;
});}
export async function transition(token:string,target:string,id:string,raw:Record<string,unknown>){
    const b=actionInput(raw),permission='projects.purchase_indents.'+(['review','return'].includes(b.action)?'review':b.action==='release'?'release':'write');
    return transaction(async db=>{
        const{a,p}=await access(db,token,target,[permission],true);id=uuid(id);const record=await store.get(db,p.id,id);if(!record)throw missing();if(record.version!==b.version)throw store.stale();const before=structuredClone(record),state=record.data.state,now=timestamp(new Date().toISOString())!;
        switch(b.action){
            case 'submit':if(state!=='draft')invalid('','Only drafts may be submitted');await identities(db,record.data);record.data.state='submitted';break;
            case 'return':if(!['submitted','reviewed'].includes(state))invalid('','Only unreleased review work may be returned');record.data.state='draft';record.data.review=null;break;
            case 'review':{
                if(state!=='submitted')invalid('','Submit before review');await identities(db,record.data);const versions:Record<string,number>={};
                for(const line of record.data.lines){const source=await lineSource(db,p.id,line.demand_id);if(!source.active)invalid('demand_id','Requirement is inactive');versions[line.demand_id]=source.demand_version;}
                record.data.review={actor_id:a.id,at:now,evidence:b.evidence,demand_versions:versions};record.data.state='reviewed';break;
            }
            case 'release':{
                if(!['reviewed','part_released'].includes(state))invalid('','Reviewed demand is required before release');await identities(db,record.data);
                if(!b.line_ids.length||!record.data.review)invalid('line_ids','Select distinct reviewed lines for release');
                const lines=new Map(record.data.lines.map(l=>[l.demand_id,l])),prior=new Set(record.data.released.map(l=>l.demand_id)),all=await store.list(db,p.id),additions:Release[]=[];
                for(const demand of b.line_ids){const line=lines.get(demand);if(!line||prior.has(demand))invalid('line_ids','Unknown or previously released line');const source=await lineSource(db,p.id,demand);
                    if(record.data.review.demand_versions[demand]!==source.demand_version)invalid('demand_id','Project requirement changed after review; create a revised indent for remaining demand');
                    let committed=BigInt(0);for(const other of all)for(const r of other.data.released)if(r.demand_id===demand){committed+=quantity(r.quantity);if(committed>BigInt('9223372036854775807'))invalid('quantity','Released quantity exceeds supported requirement');}
                    if(!source.active)invalid('demand_id','Project requirement is inactive');if(source.policy.status!=='eligible')invalid('demand_id','Client make policy blocks release: '+source.policy.reasons.join('; '));if(source.holds.holds.length)invalid('demand_id','Client purchase hold blocks release');
                    const total=quantity(source.quantity);if(committed>total||quantity(line.quantity)>total-committed)invalid('quantity','Quantity exceeds remaining project requirement');
                    additions.push({...line,at:now,actor_id:a.id,evidence:b.evidence,source});
                }
                record.data.released.push(...additions);record.data.state=record.data.released.length===record.data.lines.length?'released':'part_released';break;
            }
            case 'withdraw':if(state==='released')invalid('','Released commitments cannot be withdrawn here');if(state==='withdrawn')invalid('','Indent already withdrawn');record.data.state='withdrawn';record.is_active=false;break;
        }
        record.version++;const saved=await persist(db,record,a.id);await store.event(db,before,saved,b.action,b.reason,a.id);return saved;
    });
}
export async function workspace(token:string,target:string){return transaction(async db=>{
    const{a,p,w}=await access(db,token,target,['projects.purchase_indents.read','projects.purchase_indents.view']),rows=await store.list(db,p.id),permissions:Record<string,boolean>={'purchase_indents.read':true};
    for(const action of ['write','review','release']){const code='projects.purchase_indents.'+action;if(!a.allows(code))continue;try{await scope(db,a,p,w,code,true);permissions['purchase_indents.'+action]=true;}catch(e){if(!(e instanceof PurchaseError)||![403,422].includes(e.status))throw e;}}
    const lines=[];for(const d of await demands(db,p.id)){const source=await lineSource(db,p.id,d.id),total=quantity(source.quantity);let used=BigInt(0);for(const row of rows)for(const r of row.data.released)if(r.demand_id===d.id){used+=quantity(r.quantity);if(used>BigInt('9223372036854775807'))invalid('quantity','Released quantity exceeds the supported limit');}lines.push({...source,id:d.id,released_quantity:format(used,3),remaining_quantity:format(total>used?total-used:BigInt(0),3)});}
    const members:{id:string;label:string}[]=[];
    if(permissions['purchase_indents.write'])for(const u of await assignmentIdentities(db,[]))members.push({id:u.id,label:u.name});
    else{const seen=new Set<string>();for(const row of rows)for(const id of [row.data.doer_id,row.data.resolver_id])if(!seen.has(id)){seen.add(id);const u=await assignmentIdentity(db,id);if(u)members.push({id:u.id,label:u.name});}}
    const currency=(await db.query('SELECT currency FROM projects WHERE id=$1',[p.id])).rows[0].currency;
    return{currency,lines,members,rows,permissions};
});}
