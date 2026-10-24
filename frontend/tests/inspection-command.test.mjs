import assert from 'node:assert/strict';
import test from 'node:test';
import {readFileSync} from 'node:fs';
import ts from 'typescript';
const source=readFileSync(new URL('../src/components/purchase-fms/inspection-model.ts',import.meta.url),'utf8');
const compiled=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022}}).outputText;
const {inspectionCommand}=await import(`data:text/javascript;base64,${Buffer.from(compiled).toString('base64')}`);
const form=values=>{const f=new FormData();for(const[k,v]of Object.entries(values))f.set(k,v);return f};
test('inspection command retains explicit false flags and ignores forged source and actor',()=>{
 const p={code:'WP-01',title:'Actual QAP test',is_witness:true,is_cleared:false,remarks:'Pending',actor_id:'forged'};
 const q=inspectionCommand(4,'record',form({reason:'Actual visit',status:'scheduled',scheduled_date:'2026-10-12',factory_location:'Actual factory',source_snapshot:'forged',qap_approved:'yes',no_witness_points_reason:'old empty reason'}),[p]);
 assert.equal(q.version,4);assert.equal(q.inspection.witness_points[0].is_cleared,false);assert.equal(q.inspection.no_witness_points_reason,'');assert.equal('actor_id' in q.inspection.witness_points[0],false);assert.equal('source_snapshot' in q,false);assert.equal('qap_approved' in q.inspection,false);
 p.title='Changed local draft';assert.equal(q.inspection.witness_points[0].title,'Actual QAP test');
});
test('revision command sends only current version and reason, discarding prior findings and checks',()=>{
 assert.deepEqual(inspectionCommand(5,'revise',form({reason:'Correct actual source',status:'cleared',findings:'Old findings'}),[{code:'X',title:'Old point',is_witness:true,is_cleared:true,remarks:''}]),{version:5,action:'revise',reason:'Correct actual source'});
});
test('empty approved QAP checklist retains explicit clearance reason and all truthful dates',()=>{
 const q=inspectionCommand(0,'record',form({reason:'Actual no-points QAP',status:'cleared',call_date:'2026-10-10',scheduled_date:'2026-10-13',actual_date:'2026-10-11',no_witness_points_reason:'Actual approved QAP has no witness requirements'}),[]);
 assert.deepEqual(q.inspection.witness_points,[]);assert.equal(q.inspection.no_witness_points_reason,'Actual approved QAP has no witness requirements');assert.equal(q.inspection.actual_date,'2026-10-11');assert.equal(q.inspection.scheduled_date,'2026-10-13');
});
