import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import ts from 'typescript';
const output = ts.transpileModule(readFileSync(new URL('../src/components/projects/project-activation-model.ts', import.meta.url), 'utf8'), {compilerOptions:{module:ts.ModuleKind.ESNext}}).outputText;
const { activationValidationMessage: validate } = await import(`data:text/javascript;base64,${Buffer.from(output).toString('base64')}`);
const input = changes => ({currentStatus:'draft',targetStatus:'active',loiNumber:'',poNumber:'',evidenceReady:false,...changes});
test('ordinary edits and unchanged Active status do not require activation evidence', () => {
  for (const status of ['draft','on_hold','closed','active']) assert.equal(validate(input({currentStatus:status,targetStatus:status})), '');
});
test('activation tells users exactly which prerequisites are missing', () => {
  assert.match(validate(input({})), /Enter an LOI or PO reference.*Upload qualifying contract evidence/);
  assert.equal(validate(input({loiNumber:' LOI-001 '})), 'Upload qualifying contract evidence using Add evidence.');
  assert.equal(validate(input({evidenceReady:true,loiNumber:'  '})), 'Enter an LOI or PO reference.');
});
test('either reference satisfies the reference requirement for activation or resumption', () => {
  for (const currentStatus of ['draft','on_hold']) for (const reference of [{loiNumber:'LOI-001'}, {poNumber:'PO-001'}]) assert.equal(validate(input({currentStatus,evidenceReady:true,...reference})), '');
});
test('unavailable evidence checks defer to the server without claiming missing evidence', () => {
  assert.equal(validate(input({poNumber:'PO-001',evidenceReady:null})), '');
});
