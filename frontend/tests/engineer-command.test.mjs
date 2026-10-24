import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import ts from 'typescript';
const source=readFileSync(new URL('../src/components/purchase-fms/engineer-model.ts',import.meta.url),'utf8');
const compiled=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.ESNext}}).outputText;
const {engineerCommand}=await import(`data:text/javascript;base64,${Buffer.from(compiled).toString('base64')}`);
const form=o=>{const f=new FormData();for(const[k,v]of Object.entries(o))f.set(k,String(v));return f;};
test('engineer confirmation sends only declared evidence, without inherited quantities or actors',()=>{
const fields={status:'confirmed',confirmation_date:'2026-10-11',engineer_name:'Actual engineer',designation:'Engineer',mbook_ref:'M-Book actual',work_package_ref:'Site package',remarks:'Actual review'};
assert.deepEqual(engineerCommand(1,'record',form({...fields,reason:'Actual confirmation',grn_id:'forged',accepted_qty:'999',actor_id:'forged'})),{version:1,action:'record',reason:'Actual confirmation',confirmation:fields});
});
test('pending confirmation preserves optional empty references',()=>{
assert.deepEqual(engineerCommand(0,'record',form({status:'pending',reason:'Prepare'})).confirmation,{status:'pending',confirmation_date:'',engineer_name:'',designation:'',mbook_ref:'',work_package_ref:'',remarks:''});
});
test('engineer revision sends only reason and expected version',()=>{
assert.deepEqual(engineerCommand(2,'revise',form({reason:'Correct actual GRN',status:'confirmed',engineer_name:'Excluded'})),{version:2,action:'revise',reason:'Correct actual GRN'});
});
