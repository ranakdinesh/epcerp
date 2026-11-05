import {writeFile,readFile,stat,unlink} from 'node:fs/promises';
import {join} from 'node:path';
import {execFile} from 'node:child_process';
import {promisify} from 'node:util';
const run=promisify(execFile),limit=8*1048576;
async function command(binary,args,dir,signal,timeout=45000){return run('/usr/bin/prlimit',['--as=1073741824','--cpu=60','--nofile=64','--',binary,...args],{timeout,maxBuffer:limit,signal,env:{PATH:'/usr/bin:/bin',HOME:dir,LANG:'C.UTF-8'}})}
async function bounded(path){if((await stat(path)).size>limit)throw Error('pdf_text_too_large');return readFile(path,'utf8')}
export async function extractPDF(data,dir,signal,nativeOnly=false){
 const source=join(dir,'source.pdf');await writeFile(source,data,{mode:0o600});
 const {stdout:info}=await command('/usr/bin/pdfinfo',[source],dir,signal);
 const n=Number(/^Pages:\s+(\d+)/m.exec(info)?.[1]);
 if(!Number.isInteger(n)||n<1||n>1000||!/^Encrypted:\s+no\s*$/m.test(info))throw Error('unsupported_pdf');
 const target=join(dir,'native.txt');await command('/usr/bin/pdftotext',['-layout','-enc','UTF-8',source,target],dir,signal);
 const native=(await bounded(target)).split('\f');if(native.length&&native.at(-1).trim()==='')native.pop();
 if(native.length>n)throw Error('pdf_page_count_mismatch');
 const pages=[],warnings=[];let bytes=0,ocr=0;
 for(let i=0;i<n;i++){
  signal.throwIfAborted();let text=(native[i]??'').trim(),method='native';
  if(text.replace(/\s/g,'').length<20&&!nativeOnly){
   if(++ocr>100)throw Error('ocr_page_limit');
   const image=join(dir,'page-'+(i+1));
   await command('/usr/bin/pdftoppm',['-f',String(i+1),'-l',String(i+1),'-r','120','-singlefile','-png',source,image],dir,signal);
   const {stdout}=await command('/usr/bin/tesseract',[image+'.png','stdout','-l','eng+hin','--psm','3'],dir,signal);text=stdout.trim();method='ocr';await unlink(image+'.png');
   warnings.push({page:i+1,code:text?'ocr_used':'empty_page'});
  }
  bytes+=Buffer.byteLength(text);if(bytes>limit)throw Error('pdf_text_too_large');
  pages.push({page:i+1,text,method});
 }
 if(!nativeOnly&&!pages.some(p=>p.text))throw Error('no_readable_text');
 return {reader_version:nativeOnly?'pdf-native-v1':'pdf-text-v1',pages,warnings};
}
