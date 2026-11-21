import type { Answer, Processing, Snapshot } from './types';
type State = { state:string; error_code:string; mode:string; created_at:Date|string };
export function processingStatus(job:State, snapshot:Snapshot, answers:Answer[], task:{state:string;created_at:Date|string;updated_at:Date|string}|undefined, call:{state:string;http_status:number|null;input_evidence?:{partial?:boolean;total_pages?:number;supplied_pages?:number}}|undefined, now=Date.now()):Processing {
    const modelQuestions=snapshot.questions.filter(q=>!q.manual_review);
    const returned=answers.filter(a=>a.origin!=='manual'&&(a.answer.trim()||a.status==='not_found'||a.status==='conflicting')).length;
    const error=job.error_code;
    const phase:Processing['phase']=job.state==='queued'?'queued':job.state==='processing'?(call?.state==='failed'&&[429,503].includes(call.http_status??0)?'retrying':call?.http_status!=null?'saving_answers':call?.state==='sent'?'waiting_for_model':'preparing_documents'):error?(returned?'partial':'failed'):snapshot.preparation_method==='upload'?'imported':job.mode==='manual'?'manual':'complete';
    const start=new Date(task?.created_at??job.created_at).getTime(),end=task&&['ready','failed'].includes(task.state)?new Date(task.updated_at).getTime():now;
    return {evidence:typeof call?.input_evidence?.partial==='boolean'?call.input_evidence as {partial:boolean;total_pages:number;supplied_pages:number}:undefined,phase,elapsed_seconds:job.mode==='manual'?0:Math.max(0,Math.floor((end-start)/1000)),source_count:snapshot.sources.length,model_questions:modelQuestions.length,manual_questions:snapshot.questions.length-modelQuestions.length,returned_answers:returned,answered_questions:answers.filter(a=>a.status==='answered'&&a.answer.trim()).length,provider_status:call?.http_status??null,error_code:error};
}
