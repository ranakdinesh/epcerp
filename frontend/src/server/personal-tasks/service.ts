import 'server-only';
import { transaction } from '../db';
import { requireAnyPermission } from '../auth/access';
import { AppError } from '../errors';
import { documentTasks } from '../master/document-task-reader';
import { quotationTasks } from '../purchasing/quotation-tasks';
import { source } from '../purchasing/task-source';
import { query,overdue } from './validation';
export async function page(token:string,params:URLSearchParams,now=new Date()){
    return transaction(async db=>{
        const actor=await requireAnyPermission(db,token,['work.tasks.view'],'business');
        if(!actor.allows('work.tasks.read'))throw new AppError(403,'Current personal task permissions required');
        const q=query(params),execute=actor.allows('projects.purchase_workflow.execute'),resolve=actor.allows('projects.purchase_workflow.resolve');
        const rows=[],context=new Map<string,Awaited<ReturnType<typeof source>>>();let total=0;
        if(execute||resolve){
            const search='%'+q.search.replace(/[\\%_]/g,c=>'\\'+c)+'%',values=[actor.id,execute,resolve,q.role,q.state,search];
            const filter=`p.is_active AND (p.steps @> jsonb_build_array(jsonb_build_object('doer_id',$1::text)) OR p.steps @> jsonb_build_array(jsonb_build_object('blocked_resolver_id',$1::text))) AND (s->>'number')::integer BETWEEN 3 AND 20
              AND (($2::boolean AND s->>'doer_id'=$1 AND $4 IN('all','doer')) OR ($3::boolean AND s->>'state'='blocked' AND s->>'blocked_resolver_id'=$1 AND $4 IN('all','resolver')))
              AND ($5='all' OR s->>'state'=$5) AND (p.code ILIKE $6 OR p.description ILIKE $6 OR s->>'title' ILIKE $6)`;
            total=Number((await db.query(`SELECT count(*) FROM purchase_work_packages p CROSS JOIN LATERAL jsonb_array_elements(p.steps) s WHERE ${filter}`,values)).rows[0].count);
            const assigned=(await db.query(`SELECT p.id,p.project_id,p.code,p.version,s AS step FROM purchase_work_packages p CROSS JOIN LATERAL jsonb_array_elements(p.steps) s WHERE ${filter} ORDER BY CASE WHEN s->>'state'='blocked' THEN 0 ELSE 1 END,coalesce(nullif(s->>'planned_date',''),'9999-12-31'),p.id,(s->>'number')::integer LIMIT $7 OFFSET $8`,[...values,q.size,(q.page-1)*q.size])).rows;
            for(const r of assigned){let current=context.get(r.id);if(!current){current=await source(db,r.id);context.set(r.id,current);}const s=r.step,relation=execute&&s.doer_id===actor.id&&q.role!=='resolver'?'doer':'resolver',planned=s.planned_date??'',due=s.resolution_due??'';
                rows.push({id:r.id+':'+s.number,project_id:r.project_id,project_name:current.project_name,package_id:r.id,package_code:r.code,step_number:s.number,title:s.title,phase:s.phase,relation,coordination_state:s.state,planned_date:planned,resolution_due:due,block_reason:s.block_reason??'',available:current.available,blockers:current.blockers,package_version:r.version,is_overdue:overdue(relation,planned,due,now)});
            }
        }
        return {supplier_review_tasks:await documentTasks(db,actor),quotation_tasks:await quotationTasks(db,actor),rows,total,page:q.page,page_size:q.size};
    });
}
