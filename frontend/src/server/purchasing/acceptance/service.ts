import 'server-only';
import { randomUUID } from 'node:crypto';
import { transaction,type DB } from '../../db';
import type { Actor } from '../core/access';
import { now } from '../core/time';
import { object,string,version } from '../core/validation';
import { id } from '../sourcing/values';
import { accessGuard,guard,stale,missing,apply,validate,accepted } from './policy';
import type { Command } from './types';
import * as context from './context';
import * as repo from './repository';
export async function workspaceBound(db:DB,a:Actor,p:string,id:string){const caps=await context.access(db,a,p,id);accessGuard(a,caps,false);const source=await context.source(db,p,id),record=await repo.get(db,p,id);if(record&&!record.is_active)throw missing();const out={record,source,actions:[]as string[],history:record&&caps.scoped?await repo.history(db,record.id):[],task_only:!caps.scoped};try{guard(source);accessGuard(a,caps,true);out.actions.push('record');}catch{}return out;}
export async function workspace(token:string,p:string,record:string){return transaction(async db=>{const a=await context.actor(db,token);id(p);id(record);return workspaceBound(db,a,p,record);});}
export function input(raw:Record<string,unknown>):Command{const r=object(raw,['version','action','reason','response']),v=object(r.response??{},['status','accepted_date','signatory_name','document_reference','remarks']);return validate({version:version(r.version??0),action:string(r.action,'action',''),reason:string(r.reason,'reason',''),response:{status:string(v.status,'status',''),accepted_date:string(v.accepted_date,'accepted_date',''),signatory_name:string(v.signatory_name,'signatory_name',''),document_reference:string(v.document_reference,'document_reference',''),remarks:string(v.remarks,'remarks','')}});}
export async function execute(token:string,p:string,record:string,raw:Record<string,unknown>){return transaction(async db=>{const a=await context.actor(db,token);id(p);id(record);const q=input(raw),caps=await context.access(db,a,p,record);accessGuard(a,caps,true);const before=await repo.get(db,p,record);if(q.version!==(before?.version??0))throw stale();const source=await context.source(db,p,record),next=apply(before,a,caps,q,source,now(),randomUUID()),saved=await repo.save(db,next);await repo.event(db,before,saved,q.reason,a.id);return workspaceBound(db,a,p,record);});}
export async function acceptedFact(db:DB,p:string,id:string){return accepted(await repo.get(db,p,id),await context.source(db,p,id));}
