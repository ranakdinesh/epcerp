import test from 'node:test';
import assert from 'node:assert/strict';
import { multipart } from '../../src/server/purchasing/http-input';
import { AppError } from '../../src/server/errors';
const request=(parts:string[],tail='')=>new Request('http://fixture.invalid',{method:'POST',headers:{'Content-Type':'multipart/form-data; boundary="fixture-boundary"'},body:'--fixture-boundary\r\n'+parts.join('\r\n--fixture-boundary\r\n')+'\r\n--fixture-boundary--\r\n'+tail});
const file=(name='original.pdf',bytes='%PDF-1.7\nSynthetic binary \u0000\u00ff\n%%EOF')=>'Content-Disposition: form-data; name="file"; filename="'+name+'"\r\nContent-Type: application/pdf\r\n\r\n'+bytes;
test('bounded multipart preserves evidence bytes and raw filenames for domain inspection',async()=>{const r=await multipart(request([file('../original.pdf')]));assert.equal(r.name,'../original.pdf');assert.equal(r.mime,'application/pdf');assert.equal(r.data.toString('utf8'),'%PDF-1.7\nSynthetic binary \u0000\u00ff\n%%EOF');const reason='Content-Disposition: form-data; name="reason"\r\n\r\nOriginal evidence wording';const withReason=await multipart(request([reason,file()]),true);assert.equal(withReason.reason,'Original evidence wording');});
test('multipart rejects duplicate files, unsupported fields and trailing payloads',async()=>{for(const r of[request([file(),file()]),request(['Content-Disposition: form-data; name="surprise"\r\n\r\nInjected',file()]),request([file()],'extra'),request([file('')])])await assert.rejects(multipart(r),e=>e instanceof AppError&&e.status===422);});
