import type { DB } from '../../db';
import { timeSQL,timestamp } from '../../inbox/repository';
import { journal } from '../core/journal';
export type Package={id:string;description:string;quantity:string;uom:string;state:string;loi_reference:string};
export type Handoff={id:string;project_id:string;bo_id:string;attempt_id:string;packages:Package[];created_by:string;created_at:string};
export async function get(db:DB,p:string,a:string):Promise<Handoff|null>{const r=(await db.query(`SELECT id,project_id,bo_id,attempt_id,packages,created_by_id AS created_by,${timeSQL('created_at')} AS created_at FROM purchase_award_handoffs WHERE project_id=$1 AND attempt_id=$2`,[p,a])).rows[0];return r?{...r,created_by:r.created_by??'00000000-0000-0000-0000-000000000000',created_at:timestamp(r.created_at)!}:null;}
export async function save(db:DB,h:Handoff,reason:string){await db.query('INSERT INTO purchase_award_handoffs(id,project_id,bo_id,attempt_id,packages,reason,is_active,created_at,updated_at,created_by_id,updated_by_id)VALUES($1,$2,$3,$4,$5,$6,true,$7,$7,$8,$8)',[h.id,h.project_id,h.bo_id,h.attempt_id,JSON.stringify(h.packages),reason,h.created_at,h.created_by]);await journal(db,{id:h.id,module:'purchase_awards',type:'award_handoff',entity:h.id,version:1,action:'created',actor:h.created_by,project:h.project_id,at:h.created_at});}
export const handoffView=(h:Handoff)=>({ID:h.id,ProjectID:h.project_id,BOID:h.bo_id,AttemptID:h.attempt_id,Packages:h.packages,CreatedBy:h.created_by,CreatedAt:h.created_at});
