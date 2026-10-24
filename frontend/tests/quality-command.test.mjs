import assert from 'node:assert/strict';
import test from 'node:test';
import {readFileSync} from 'node:fs';
import ts from 'typescript';
const source=readFileSync(new URL('../src/components/purchase-fms/quality-model.ts',import.meta.url),'utf8');
const compiled=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022}}).outputText;
const {qualityCommand,certificateTitles}=await import(`data:text/javascript;base64,${Buffer.from(compiled).toString('base64')}`);
const form=values=>{const f=new FormData();for(const[k,v]of Object.entries(values))f.set(k,v);return f};
test('quality metadata preserves only declared details without verification or source attribution',()=>{
 assert.deepEqual(qualityCommand(3,'record','TC',form({reason:'Correction',cert_number:'TC-2',issued_by:'Actual lab',valid_until:'2026-10-11',file_name:'ref.pdf',verified:'true',verified_by_id:'forged',report_id:'forged'})),{version:3,action:'record',reason:'Correction',certificate_code:'TC',details:{cert_number:'TC-2',issued_by:'Actual lab',valid_until:'2026-10-11',file_name:'ref.pdf'}});
});
test('quality verification and revocation cannot change saved certificate metadata',()=>{
 for(const action of ['verify','revoke'])assert.deepEqual(qualityCommand(4,action,'MTC',form({reason:'Reviewed',cert_number:'forged',verified_at:'forged'})),{version:4,action,reason:'Reviewed',certificate_code:'MTC'});
});
test('quality source revision sends no certificate identity and retains five fixed required types',()=>{
 assert.deepEqual(qualityCommand(5,'revise','TC',form({reason:'New IRN',file_name:'old.pdf'})),{version:5,action:'revise',reason:'New IRN'});
 assert.deepEqual(Object.keys(certificateTitles),['TC','TR','MTC','RTC','Warranty']);
});
