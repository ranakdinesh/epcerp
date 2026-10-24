import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';
import { join, sep } from 'node:path';
import { fileURLToPath } from 'node:url';
import ts from 'typescript';
const root = fileURLToPath(new URL('../src/components/', import.meta.url));
function files(dir) { return readdirSync(dir,{withFileTypes:true}).flatMap(entry=>entry.isDirectory()?files(join(dir,entry.name)):[join(dir,entry.name)]); }
test('all component forms use shared controls or the explicit Client Master validator',()=>{
 const violations=[];
 for(const file of files(root).filter(f=>f.endsWith('.tsx')&&!f.includes(`${sep}ui${sep}`)&&!f.endsWith(join('clients','client-management.tsx')))){
 const source=readFileSync(file,'utf8');const ast=ts.createSourceFile(file,source,ts.ScriptTarget.Latest,true,ts.ScriptKind.TSX);
 function visit(node){if((ts.isJsxOpeningElement(node)||ts.isJsxSelfClosingElement(node))&&['form','input','select','textarea','label'].includes(node.tagName.getText(ast)))violations.push(file+':'+node.tagName.getText(ast));ts.forEachChild(node,visit);}visit(ast);
 }
 assert.deepEqual(violations,[]);
});
test('server field errors preserve paths without request-input details',async()=>{
 const output=ts.transpileModule(readFileSync(new URL('../src/components/ui/server-field-errors.ts',import.meta.url),'utf8'),{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022}}).outputText;
 const {FieldValidationError}=await import(`data:text/javascript;base64,${Buffer.from(output).toString('base64')}`);
 const issue=[{loc:['body','email'],msg:'Enter a valid email',input:'sensitive-input'}];
 const error=new FieldValidationError(issue);
 assert.deepEqual(error.detail,issue);assert.ok(!error.message.includes('sensitive-input'));
});

test('server messages are dispatched explicitly while a save fieldset is disabled', async()=>{
 const output=ts.transpileModule(readFileSync(new URL('../src/components/ui/server-field-errors.ts',import.meta.url),'utf8'),{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022}}).outputText;
 const {displayServerErrors}=await import(`data:text/javascript;base64,${Buffer.from(output).toString('base64')}`);
 class Input extends EventTarget { name='name'; disabled=true; message=''; focused=false; setCustomValidity(message){this.message=message;} focus(){this.focused=true;} }
 const previous={input:globalThis.HTMLInputElement,select:globalThis.HTMLSelectElement,area:globalThis.HTMLTextAreaElement,raf:globalThis.requestAnimationFrame};
 try {
 globalThis.HTMLInputElement=Input;globalThis.HTMLSelectElement=class{};globalThis.HTMLTextAreaElement=class{};
 const frames=[];globalThis.requestAnimationFrame=callback=>frames.push(callback);
 const input=new Input(),elements=[input];elements.namedItem=name=>name==='name'?input:null;
 let received;input.addEventListener('invalid',event=>{received=event.detail;});
 assert.equal(displayServerErrors({elements},{detail:[{loc:['body','name'],msg:'Name already exists',input:'private'}]}),true);
 assert.equal(received,'Name already exists');assert.equal(input.message,'Name already exists');assert.equal(input.focused,false);
 input.disabled=false;frames.forEach(callback=>callback());assert.equal(input.focused,true);
 } finally {globalThis.HTMLInputElement=previous.input;globalThis.HTMLSelectElement=previous.select;globalThis.HTMLTextAreaElement=previous.area;globalThis.requestAnimationFrame=previous.raf;}
});

test('required combobox errors use the accessible field name, never nested catalogue options',async()=>{
 const file=readFileSync(new URL('../src/components/ui/validated-controls.tsx',import.meta.url),'utf8');
 const ast=ts.createSourceFile('controls.tsx',file,ts.ScriptTarget.Latest,true,ts.ScriptKind.TSX);
 const fn=ast.statements.find(node=>ts.isFunctionDeclaration(node)&&node.name?.text==='controlError');
 const output=ts.transpileModule(fn.getText(ast),{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022}}).outputText;
 const {controlError}=await import(`data:text/javascript;base64,${Buffer.from(output).toString('base64')}`);
 const label={textContent:'Item name / SKU * Flood Light private catalogue SKU Already added',querySelectorAll:()=>[]};
 const control={willValidate:true,required:true,value:'',labels:[{cloneNode:()=>label}],getAttribute:name=>name==='aria-label'?'Item name or SKU':null};
 assert.equal(controlError(control),'Item name or SKU is required.');
 assert.ok(!controlError(control).includes('catalogue'));
});
