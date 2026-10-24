import assert from 'node:assert/strict';
import test from 'node:test';
import {readFileSync} from 'node:fs';
import ts from 'typescript';
const source=readFileSync(new URL('../src/components/purchase-fms/data.ts',import.meta.url),'utf8');
const {outputText}=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022}});
const {purchaseProgressLabel,purchaseProjectHref}=await import(`data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`);
test('missing progress is not a fabricated zero or complete result',()=>{
 assert.equal(purchaseProgressLabel({progress_percent:null}),'Progress report not available yet');
 assert.equal(purchaseProgressLabel({progress_percent:0}),'0%');
 assert.equal(purchaseProgressLabel({progress_percent:50}),'50%');
});
test('purchase entry keeps its own module and project scope',()=>{
 assert.equal(purchaseProjectHref('11111111-1111-4111-8111-111111111111'),'/dashboard?screen=purchase-fms&project=11111111-1111-4111-8111-111111111111&step=PUR-01');
 assert.equal(purchaseProjectHref('invalid&step=9'),'/dashboard?screen=purchase-fms&project=invalid%26step%3D9&step=PUR-01');
});
