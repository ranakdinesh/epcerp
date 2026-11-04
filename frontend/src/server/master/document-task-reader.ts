import 'server-only';
import type { DB } from '../db';
import type { requireAnyPermission } from '../auth/access';
type Actor=Awaited<ReturnType<typeof requireAnyPermission>>;
// Published read contract. Supplier documents keep ownership of task actions/history.
export async function documentTasks(db:DB,actor:Actor){
    const rows=(await db.query(`SELECT t.*,s.code,s.name,d.title,d.original_filename FROM supplier_document_tasks t JOIN suppliers s ON s.id=t.supplier_id JOIN supplier_documents d ON d.id=t.document_id WHERE t.user_id=$1 AND t.is_active ORDER BY t.created_at DESC,t.id`,[actor.id])).rows;
    return rows.filter(r=>r.kind==='review'?actor.allows('accounts.vendor_verification.read')&&actor.allows('accounts.vendor_verification.write'):r.kind!=='correction'||actor.allows('settings.supplier_documents.write')).map(r=>{
        const title=r.title||r.original_filename,completed=!!r.completed_at;
        return {id:r.id,document_id:r.document_id,title,kind:r.kind,outcome:r.outcome,completed,supplier_id:r.supplier_id,code:r.code,name:r.name,state:completed?'completed':'pending',message:completed?r.outcome+': '+title:(r.kind==='correction'?'Replace rejected document: ':'Review document: ')+title,link:'/dashboard?screen=suppliers&supplier_id='+r.supplier_id+'&document_id='+r.document_id};
    });
}
