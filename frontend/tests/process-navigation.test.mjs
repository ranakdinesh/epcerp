import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import ts from 'typescript';
const source = readFileSync(new URL('../src/components/processes/process-directory.ts', import.meta.url), 'utf8');
const { outputText } = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 } });
const { tenderProcess, projectProcess, processForScreen, availableProcessStep, processStepHref, filterProcessSteps } = await import(`data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`);
test('tender reference retains 26 ordered unique stages with only delivered intake routable', () => {
 assert.deepEqual(tenderProcess.steps.map(s=>s.number), Array.from({length:26},(_,i)=>i+1));
 assert.equal(new Set(tenderProcess.steps.map(s=>s.code)).size,26);
 assert.deepEqual(tenderProcess.steps.filter(s=>s.available).map(s=>s.code),['TS-01']);
 for (const step of tenderProcess.steps.slice(1)) assert.equal(processStepHref(tenderProcess,step),undefined);
 assert.equal(processStepHref(tenderProcess,tenderProcess.steps[0]),'/dashboard?screen=tender-fms&step=TS-01');
});
test('unavailable and invalid query selections resolve to real entry step', () => {
 for (const process of [tenderProcess,projectProcess]) {
  for (const query of [null,'not-a-step',process.steps[1].code]) assert.equal(availableProcessStep(process,query),process.steps[0]);
  assert.equal(availableProcessStep(process,process.steps[0].code),process.steps[0]);
 }
 assert.equal(processForScreen('items'),undefined);
 assert.equal(processForScreen('tender-fms'),tenderProcess);
 assert.equal(processForScreen('projects'),projectProcess);
});
test('step search matches number, code, label and group without mutating directory',()=>{
 assert.equal(filterProcessSteps(tenderProcess,'  ts-01  ')[0].number,1);
 assert.equal(filterProcessSteps(tenderProcess,'26')[0].number,26);
 assert.equal(filterProcessSteps(tenderProcess,'not-existing').length,0);
 assert.ok(filterProcessSteps(tenderProcess,'technical').length>1);
 assert.equal(tenderProcess.steps.length,26);
});

test('confirmed project delivery remains 19 processes, with register separate from numbered stages',()=>{
 const stages = projectProcess.steps.filter(s=>s.number>0);
 assert.equal(stages.length,19);
 assert.deepEqual(stages.map(s=>s.number),Array.from({length:19},(_,i)=>i+1));
 assert.deepEqual(stages.map(s=>s.code),['order_receipt','site_survey','risk_assessment','design_boq_drawings','bom_finalisation','quantity_item_approval','project_planning','manpower_assessment','contract_scope_boq','contractor_enquiries','contractor_rates','contractor_negotiation','contractor_work_order','manpower_mobilisation','site_preparation','material_delivery','material_installation','commissioning','project_completion']);
 assert.ok(stages.every(s=>!s.available && s.purpose && s.evidence.length));
 assert.equal(projectProcess.steps[0].number,0);
});
test('Purchase map routes BO, indent, quotation receipt and comparison only within a selected project', async()=>{
 const {purchaseProcess,processHandoffs,processMapHref}=await import(`data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`);
 const stages=purchaseProcess.steps.filter(s=>s.number>0);
 assert.equal(stages.length,21);
 assert.deepEqual(stages.map(s=>s.number),Array.from({length:21},(_,i)=>i+1));
 assert.deepEqual([...new Set(stages.map(s=>s.group.split(' · ')[0]))],['WO2I','I2LOI','LOI2PO','PO2INS','INS2D']);
 assert.ok(stages.every(s=>s.evidence.length && s.team));
 assert.deepEqual(stages.filter(s=>s.available).map(s=>s.code),["PUR-01","PUR-02","PUR-03","PUR-04","PUR-05","PUR-06","PUR-07","PUR-08","PUR-PO-CLIENT","PUR-09","PUR-10","PUR-11","PUR-12","PUR-13","PUR-14","PUR-15","PUR-16","PUR-17","PUR-18","PUR-19","PUR-20"]);
 for(const stage of stages.filter(s=>s.available)) { assert.equal(processStepHref(purchaseProcess,stage),undefined); assert.ok(processStepHref(purchaseProcess,stage,"project-id").endsWith("&project=project-id")); }
 assert.equal(availableProcessStep(purchaseProcess,'PUR-20').code,'PUR-20');
 assert.equal(processStepHref(purchaseProcess,purchaseProcess.steps[0]),undefined);
 assert.equal(processStepHref(purchaseProcess,purchaseProcess.steps[0],'project-id'),'/dashboard?screen=purchase-fms&step=PUR-01&project=project-id');
 assert.equal(processStepHref(purchaseProcess,purchaseProcess.steps[8]),undefined);
 assert.equal(processMapHref(purchaseProcess),'/dashboard?screen=purchase-fms&view=map');
 assert.deepEqual(processHandoffs.map(h=>[h.from,h.to]),[['tender-fms','projects'],['projects','purchase-fms'],['purchase-fms','projects']]);
});
