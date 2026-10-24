import assert from 'node:assert/strict';
import test from 'node:test';
import {readFileSync} from 'node:fs';
import ts from 'typescript';
const source=readFileSync(new URL('../src/components/purchase-fms/technical-model.ts',import.meta.url),'utf8');
const compiled=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022}}).outputText;
const {technicalCommand}=await import(`data:text/javascript;base64,${Buffer.from(compiled).toString('base64')}`);
const form=values=>{const f=new FormData();for(const[k,v]of Object.entries(values))f.set(k,v);return f};
test('client decision preserves explicit negative clearance and excludes caller source/actors',()=>{
 const command=technicalCommand('client_approval',8,form({status:'approved_with_comments',reference:'Actual letter',date:'2026-10-11',approver_name:'External client',approver_designation:'Engineer',file_id:'owned-letter',qap_approved:'no',conditions_closed:'no',closure_file_id:'',remarks:'Open QAP',reason:'Record actual decision',recorded_by_id:'forged',revision:'999',source:'forged'}));
 assert.equal(command.qap_approved,false);assert.equal(command.conditions_closed,false);assert.equal(command.approval.file_id,'owned-letter');assert.equal(command.approval.revision,undefined);assert.equal(command.approval.recorded_by_id,undefined);assert.equal(command.source,undefined);
});
test('owned document identity and kind cannot be replaced by form fields',()=>{
 const command=technicalCommand('document',4,form({id:'forged',kind:'forged',title:'QAP',category:'Quality',standard:'Applicable client specification',required:'yes',is_active:'yes',status:'received',revision:'Vendor Rev 2',received_date:'2026-10-11',file_id:'actual',notes:'',na_reason:'',reason:'Receive'}),{id:'owned',kind:'qap'});
 assert.equal(command.document.id,'owned');assert.equal(command.document.kind,'qap');assert.equal(command.document.required,true);assert.equal(command.document.revision,'Vendor Rev 2');
});
test('independent rejection is explicit false and captures no reviewer identity',()=>assert.deepEqual(technicalCommand('team_review',9,form({approve:'no',remarks:'Correct kit',reason:'Review',reviewer_id:'forged'})),{action:'team_review',version:9,approve:false,remarks:'Correct kit',reason:'Review'}));
test('revision command does not carry obsolete approval fields',()=>assert.deepEqual(technicalCommand('revise',10,form({reason:'Correct vendor revision',status:'approved',file_id:'old'})),{action:'revise',version:10,reason:'Correct vendor revision'}));
