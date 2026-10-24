import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import ts from 'typescript';
const source = readFileSync(new URL('../src/components/ui/prefix-options.ts', import.meta.url), 'utf8');
const output = ts.transpileModule(source, {compilerOptions:{module:ts.ModuleKind.ESNext}}).outputText;
const {prefixOptions} = await import(`data:text/javascript;base64,${Buffer.from(output).toString('base64')}`);
const options = ['Mumbai','Amritsar','mohali','Ahmedabad','Navi Mumbai'].map(name=>({name}));
test('empty query sorts names A–Z without mutating source',()=>{
 assert.deepEqual(prefixOptions(options,'').map(x=>x.name),['Ahmedabad','Amritsar','mohali','Mumbai','Navi Mumbai']);
 assert.equal(options[0].name,'Mumbai');
});
test('one letter matches prefix case-insensitively, never a middle substring',()=>{
 assert.deepEqual(prefixOptions(options,' M ').map(x=>x.name),['mohali','Mumbai']);
 assert.deepEqual(prefixOptions(options,'mum').map(x=>x.name),['Mumbai']);
 assert.deepEqual(prefixOptions(options,'bai'),[]);
});
