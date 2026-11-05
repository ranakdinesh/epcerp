// Preserve the existing reviewed Word package while its application owner changes.
import {readFile,writeFile} from 'node:fs/promises';
import {createHash} from 'node:crypto';
import {unzipSync} from 'fflate';
const source=new URL('../assets/templates/tender-tsc.docx',import.meta.url),target=new URL('../src/server/tenders/formats/template-package.json',import.meta.url),bytes=await readFile(source),entries=unzipSync(bytes),parts=Object.fromEntries(Object.keys(entries).sort().filter(k=>k!=='word/document.xml').map(k=>[k,Buffer.from(entries[k]).toString('base64')])),content=JSON.stringify({source_sha256:createHash('sha256').update(bytes).digest('hex'),parts})+'\n';
if(process.argv.includes('--check')){if(await readFile(target,'utf8')!==content)throw Error('Run node scripts/sync-tender-format-template.mjs to sync the reviewed template package')}else await writeFile(target,content);
