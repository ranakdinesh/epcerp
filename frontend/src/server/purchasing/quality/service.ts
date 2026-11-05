import 'server-only';
import { randomUUID } from 'node:crypto';
import { transaction,type DB } from '../../db';
import type { Actor } from '../core/access';
import { now } from '../core/time';
import { indiaDate } from '../../personal-tasks/validation';
import { object,string,version } from '../core/validation';
import { id } from '../sourcing/values';
import { accessGuard,actionAccess,missing,stale,can,validate,apply,readiness,verified,definitions } from './policy';
import type { Command,Details } from './types';
import * as context from './context';
import * as repo from './repository';
export function input(raw:Record<string,unknown>):Command{const r=object(raw,['version','action','reason','certificate_code','details']),c:Command={version:version(r.version??0),action:string(r.action,'action',''),reason:string(r.reason,'reason',''),certificate_code:string(r.certificate_code,'certificate_code',''),details:null};if(r.details!=null){const fields=['cert_number','issued_by','valid_until','file_name'],q=object(r.details,fields),out:Record<string,string>={};for(const k of fields)out[k]=string(q[k],k,'');c.details=out as Details;}return validate(c);}
export async function workspaceBound(db:DB,a:Actor,p:string,id:string){const caps=await context.access(db,a,p,id);accessGuard(a,caps,false);const source=await context.source(db,p,id),record=await repo.store.get(db,p,id);if(record&&!record.is_active)throw missing();const today=indiaDate(new Date()),actions=[],certificate_actions:Record<string,string[]>={};for(const action of ['revise'])try{can(record,a,caps,{action,certificate_code:action==='record'?'TC':'',version:0,reason:'Workspace',details:null},source,today);actions.push(action);}catch{}for(const[code]of definitions){certificate_actions[code]=[];for(const action of ['record','verify','revoke'])try{can(record,a,caps,{action,certificate_code:code,version:0,reason:'Workspace',details:null},source,today);certificate_actions[code].push(action);}catch{}}return{record,source,actions,eligibility:readiness(record,source,today),certificate_actions,history:record&&caps.scoped?await repo.history(db,record.id):[],task_only:!caps.scoped};}
export async function workspace(token:string,p:string,record:string){return transaction(async db=>{const a=await context.actor(db,token);id(p);id(record);return workspaceBound(db,a,p,record);});}
export async function execute(token:string,p:string,record:string,raw:Record<string,unknown>){return transaction(async db=>{const a=await context.actor(db,token);id(p);id(record);const q=input(raw),caps=await context.access(db,a,p,record);actionAccess(a,caps,q.action);const before=await repo.store.get(db,p,record);if(q.version!==(before?.version??0))throw stale();const source=await context.source(db,p,record),next=apply(before,a,caps,q,source,now(),randomUUID()),saved=await repo.save(db,next);await repo.event(db,before,saved,q.action,q.certificate_code,q.reason,a.id);return workspaceBound(db,a,p,record);});}
export async function verifiedFact(db:DB,p:string,id:string){return verified(await repo.store.get(db,p,id),await context.source(db,p,id),indiaDate(new Date()));}
