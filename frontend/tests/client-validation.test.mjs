import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import ts from 'typescript';
const source = readFileSync(new URL('../src/components/clients/client-validation.ts', import.meta.url), 'utf8');
const output = ts.transpileModule(source, { compilerOptions: {module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022}}).outputText;
const { clientErrors, gstinError, panError, websiteError, emailError, phoneError } = await import(`data:text/javascript;base64,${Buffer.from(output).toString('base64')}`);
test('required fields produce field-specific errors including each contact', () => {
 const errors = clientErrors({legal_name:'',address_line_1:' ',website:'',gstin:'',pan:'',contacts:[{full_name:'',email:'',phone:''}]});
 assert.deepEqual(Object.keys(errors).sort(), ['legal_name','address_line_1','gstin','pan','contacts.0.full_name','contacts.0.email','contacts.0.phone','client_type_id','industry_id','state_id','city_id','postal_code'].sort());
});
test('optional website accepts domains with or without a scheme and rejects unsafe URLs', () => {
 for (const v of ['', 'example.com', 'https://example.com/path', 'http://example.com']) assert.equal(websiteError(v), '');
 for (const v of ['abc', 'https://', 'javascript:alert(1)', 'https://example .com']) assert.notEqual(websiteError(v), '');
});

test('GSTIN state code must match the selected state', () => {
 const form={legal_name:'Example',address_line_1:'Address',website:'',gstin:'27AAACW4202F1ZQ',pan:'AAACW4202F',state:'Uttar Pradesh',contacts:[{full_name:'Contact',email:'a@example.com',phone:'9876543210'}],client_type_id:'type',industry_id:'industry',state_id:'state',city_id:'city',postal_code:'201001'};
 assert.match(clientErrors(form).gstin, /belongs to Maharashtra/);
 assert.equal(clientErrors({...form,state:'Maharashtra'}).gstin, undefined);
});
test('Indian PAN and GSTIN enforce structure, checksum and matching PAN', () => {
 assert.equal(panError('AAACW4202F'), '');
 assert.equal(gstinError('27AAACW4202F1ZQ','AAACW4202F'), '');
 for (const v of ['AAAQW4202F','AAACW0000F']) assert.notEqual(panError(v), '');
 for (const v of ['27AAACW4202F1Z3','00AAACW4202F1Z2']) assert.notEqual(gstinError(v), '');

});
test('email and phone reject blanks and malformed values', () => {
 for(const v of ['', 'a@', 'a@b', 'a b@example.com']) assert.notEqual(emailError(v), '');
 assert.equal(emailError('a@example.com'), '');
 for(const v of ['', '-------', '123', '+9198765432100']) assert.notEqual(phoneError(v), '');
 for(const v of ['9876543210','+91 98765 43210','0120-2345678','+44 20 7946 0958']) assert.equal(phoneError(v), '');
});

test('classification and location require catalogue selections and a valid Indian PIN',()=>{
 const form={legal_name:'Example',address_line_1:'Address',website:'',gstin:'27AAACW4202F1ZQ',pan:'AAACW4202F',contacts:[{full_name:'Contact',email:'a@example.com',phone:'9876543210'}],client_type_id:'type',industry_id:'industry',state_id:'state',city_id:'city',postal_code:'201001'};
 assert.deepEqual(clientErrors(form),{});
 for (const key of ['client_type_id','industry_id','state_id','city_id','postal_code']) assert.ok(clientErrors({...form,[key]:''})[key]);
 for (const pin of ['012345','12345','20100x']) assert.ok(clientErrors({...form,postal_code:pin}).postal_code);
});

test('GSTIN length explains missing characters and PAN errors do not invalidate GSTIN', () => {
 assert.match(gstinError('06AAGCP0307G1Z'), /14 characters/);
 assert.equal(gstinError('06AAGCP0307G1ZV'), '');
 const form = {legal_name:'AAI',address_line_1:'Address',website:'',gstin:'06AAGCP0307G1ZV',pan:'A234567890',contacts:[]};
 let errors=clientErrors(form);
 assert.equal(errors.gstin, undefined);
 assert.match(errors.pan, /5 letters, 4 digits and 1 letter/);
 errors=clientErrors({...form, pan:'AAACW4202F'});
 assert.equal(errors.gstin, undefined);
 assert.match(errors.pan, /does not match/);
 errors=clientErrors({...form, pan:'AAGCP0307G'});
 assert.equal(errors.gstin, undefined);
 assert.equal(errors.pan, undefined);
});

test('contact coverage does not block creation; entered contacts stay validated', () => {
 const form={legal_name:'Example',address_line_1:'Address',website:'',gstin:'27AAACW4202F1ZQ',pan:'AAACW4202F',contacts:[],client_type_id:'type',industry_id:'industry',state_id:'state',city_id:'city',postal_code:'400001'};
 assert.deepEqual(clientErrors(form),{});
 const contact={full_name:'Fixture',email:'fixture@example.invalid',phone:'9876543210',department:''};
 for (const type of ['', 'EIC', 'HOD', 'SITE_CONTACT']) assert.deepEqual(clientErrors({...form,contacts:[{...contact,contact_type:type},{...contact,contact_type:type}]}),{});
 assert.ok(clientErrors({...form,contacts:[{...contact,contact_type:'INVALID'}]})['contacts.0.contact_type']);
 assert.ok(clientErrors({...form,contacts:[{...contact,email:''}]})['contacts.0.email']);
});
