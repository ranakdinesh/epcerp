import { randomUUID } from 'node:crypto';
import type { DB } from '../../db';
import type { Actor } from '../core/access';
import { project } from '../core/access';
import { projectAccess } from '../core/project-access';
import { assignmentIdentity } from '../../auth/access';
import { PurchaseError } from '../core/validation';
import { now } from '../core/time';
import { journal } from '../core/journal';
import { directorDecisionNotice } from '../../inbox/purchasing-notices';
import { submission } from './fingerprint';
import type { Record } from './types';
export async function decide(db:DB,a:Actor,r:Record,outcome:string,reason:string){const live=await assignmentIdentity(db,a.id);if(!live?.active)throw new PurchaseError(401,'Invalid or expired credentials');const p=await project(db,r.project_id,true);for(const code of ['accounts.purchase_advances.view','accounts.purchase_advances.read','accounts.purchase_advances.write']){if(!a.allows(code)||!live.allows(code))throw new PurchaseError(403,'Financial decision permission or project scope denied');await projectAccess(db,a,p,code,code.endsWith('.write'));}if(a.id===r.submitted_by_id)throw new PurchaseError(403,'Financial decision permission or project scope denied');const id=randomUUID(),at=now();try{await db.query(`INSERT INTO finance_advance_decisions(id,project_id,package_id,request_id,order_id,order_version,source_version,submitted_by_id,outcome,kind,amount,currency,source_fingerprint,reason,is_active,created_at,updated_at,created_by_id,updated_by_id)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,true,$15,$15,$16,$16)`,[id,r.project_id,r.package_id,r.id,r.source_snapshot.order_id,r.source_snapshot.order_version,r.submitted_version,r.submitted_by_id,outcome,r.details.kind,r.amount,r.source_snapshot.currency,submission(r).fingerprint,reason,at,a.id]);}catch(e){if((e as {code?:string}).code==='23505')throw new PurchaseError(409,'Financial decision already recorded');throw e;}await journal(db,{id,module:'finance_approvals',type:'financial_decision',entity:id,version:1,action:outcome,actor:a.id,project:r.project_id,at});await directorDecisionNotice(db,id);return id;}
