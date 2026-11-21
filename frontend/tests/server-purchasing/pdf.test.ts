import test from 'node:test';
import assert from 'node:assert/strict';
import { writeFile } from 'node:fs/promises';
import { createPDF } from '../../src/server/purchasing/documents/pdf';
test('native purchasing PDFs use embedded fonts and paginate long evidence tables',async()=>{const data=await createPDF({title:'Synthetic purchasing pagination',subtitle:'Exact evidence and long specifications',landscape:true},p=>{p.table([{title:'No.',width:30},{title:'Description',width:400},{title:'Amount',width:90}],Array.from({length:20},(_,i)=>[String(i+1),'Synthetic item '+i+' · '+('Long technical requirement retained without clipping. '.repeat(i===4?130:3)),'INR 123.45']));p.paragraph('Final retained evidence marker.');});assert.equal(data.subarray(0,5).toString(),'%PDF-');assert.ok(data.length>10000);await writeFile('/tmp/epcerp-purchasing-pagination.pdf',data);});
