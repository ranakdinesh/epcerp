import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import ts from 'typescript';
const source=readFileSync(new URL('../src/components/projects/project-capture-model.ts',import.meta.url),'utf8');
const output=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022}}).outputText;
const {metadataPayload}=await import(`data:text/javascript;base64,${Buffer.from(output).toString('base64')}`);
test('metadata edits preserve optional missing values and exact decimal/date input without assignment or computed total overwrite',()=>{
 const data=new FormData();Object.entries({name:'Corrected project',client_id:'client',reason:'Correct site details',contract_value:'99999999999999.99',loi_date:'2026-10-01',project_manager_id:'wrong-owner',procurement_executive_id:'wrong-buyer',planned_duration_days:'999',safety_buffer_days:'888'}).forEach(([k,v])=>data.set(k,v));
 const body=metadataPayload(data,7);assert.equal(body.expected_version,7);assert.equal(body.contract_value,'99999999999999.99');assert.equal(body.loi_date,'2026-10-01');assert.equal(body.actual_start_date,'');assert.equal(body.site_city,'');assert.ok(!('project_manager_id' in body));assert.ok(!('procurement_executive_id' in body));assert.ok(!('planned_duration_days' in body));assert.ok(!('safety_buffer_days' in body));assert.equal(body.reason,'Correct site details');
});
