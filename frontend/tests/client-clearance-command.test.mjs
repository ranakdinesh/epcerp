import assert from 'node:assert/strict';
import test from 'node:test';
import {readFileSync} from 'node:fs';
import ts from 'typescript';
const source=readFileSync(new URL('../src/components/purchase-fms/client-clearance-model.ts',import.meta.url),'utf8');
const compiled=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022}}).outputText;
const {clearanceCommand}=await import(`data:text/javascript;base64,${Buffer.from(compiled).toString('base64')}`);
const form=values=>{const f=new FormData();for(const[k,v]of Object.entries(values))f.set(k,v);return f};
test('MDCC sends actual client fields without forged source, quantity or actor',()=>{
 assert.deepEqual(clearanceCommand(1,'record',form({reason:'Actual clearance',status:'cleared',mdcc_ref:'MDCC-1',clearance_date:'2026-10-11',cleared_by_authority:'Client consultant',remarks:'Actual dispatch instructions',file_name:'Actual reference',quality_id:'forged',uom:'forged',cleared_quantity:'999',actor_id:'forged'})),{version:1,action:'record',reason:'Actual clearance',clearance:{status:'cleared',mdcc_ref:'MDCC-1',clearance_date:'2026-10-11',cleared_by_authority:'Client consultant',remarks:'Actual dispatch instructions',file_name:'Actual reference'}});
});
test('pending MDCC preserves genuinely optional identity and reference fields',()=>{
 assert.deepEqual(clearanceCommand(0,'record',form({reason:'Prepare',status:'pending'})).clearance,{status:'pending',mdcc_ref:'',clearance_date:'',cleared_by_authority:'',remarks:'',file_name:''});
});
test('MDCC revision submits only current version and reason',()=>{
 assert.deepEqual(clearanceCommand(2,'revise',form({reason:'Actual correction',status:'cleared',mdcc_ref:'Old MDCC'})),{version:2,action:'revise',reason:'Actual correction'});
});
