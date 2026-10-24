import assert from 'node:assert/strict';
import test from 'node:test';
import {readFileSync} from 'node:fs';
import ts from 'typescript';
const output=ts.transpileModule(readFileSync(new URL('../src/components/dashboards/dashboard-model.ts',import.meta.url),'utf8'),{compilerOptions:{module:ts.ModuleKind.ESNext}}).outputText;
const {portfolioMetrics,completionBuckets}=await import(`data:text/javascript;base64,${Buffer.from(output).toString('base64')}`);
const project=(overrides={})=>({id:'1',name:'Test',lifecycle_status:'active',planned_completion_date:'2026-09-08',contract_value:null,currency:'INR',loi_number:'',po_number:'',...overrides});
test('past targets exclude closed projects and missing dates; currencies never combine',()=>{
 const rows=[project({planned_completion_date:'2026-09-07',contract_value:'100'}),project({lifecycle_status:'closed',planned_completion_date:'2026-01-01',contract_value:'25',currency:'USD'}),project({planned_completion_date:null,contract_value:'invalid'}),project({contract_value:'0',loi_number:'LOI'})];
 const m=portfolioMetrics(rows,'2026-09-08');assert.equal(m.overdue.length,1);assert.equal(m.undated,1);assert.equal(m.active,3);assert.equal(m.missingReference,3);assert.deepEqual(m.currencies,{INR:100,USD:25});
});
test('completion buckets reconcile with open projects at date boundaries',()=>{
 const rows=['2026-09-07','2026-09-08','2026-10-08','2026-10-09','2026-12-07','2026-12-08',null].map(date=>project({planned_completion_date:date}));rows.push(project({lifecycle_status:'closed'}));assert.deepEqual(completionBuckets(rows,'2026-09-08').map(d=>d.value),[1,2,2,1,1]);
});
test('empty project sets yield genuine zero counts without invented contracts',()=>{assert.deepEqual(portfolioMetrics([],'2026-09-08').currencies,{});assert.equal(completionBuckets([],'2026-09-08').reduce((s,d)=>s+d.value,0),0);});
const {matchesProjectStatus, assignmentHref}=await import(`data:text/javascript;base64,${Buffer.from(output).toString('base64')}`);
test('project status defaults to current projects and includes deactivated or closed in archive',()=>{
 const rows=[project({is_active:true}),project({is_active:true,lifecycle_status:'draft'}),project({is_active:true,lifecycle_status:'on_hold'}),project({is_active:true,lifecycle_status:'closed'}),project({is_active:false})];
 assert.equal(rows.filter(p=>matchesProjectStatus(p,'active')).length,3);
 assert.equal(rows.filter(p=>matchesProjectStatus(p,'archived')).length,2);
 assert.equal(rows.filter(p=>matchesProjectStatus(p,'all')).length,5);
 assert.equal(rows.filter(p=>matchesProjectStatus(p,'invalid')).length,3);
});
test('assignment notifications link to the exact source record',()=>{
 for(const [kind,key,expected] of [['processes','process-1',{tab:'processes',process:'process-1'}],['control','A.1',{tab:'control',task:'A.1'}],['purchasing','package-1:4',{tab:'purchasing',purchase:'package-1',purchaseStep:'4',purchaseView:'workflow'}]]) {
  const url=new URL(assignmentHref('project-1',{kind,key}),'http://localhost');
  assert.equal(url.searchParams.get('project'),'project-1');
  for(const [param,value] of Object.entries(expected)) assert.equal(url.searchParams.get(param),value);
 }
});
