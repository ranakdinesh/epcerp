import {unzipSync,strFromU8} from 'fflate';
import {AppError} from '../../errors';
import type {Question} from './types';
export type ImportRow={id:number;section:string;number:string;question:string;answer:string;question_id:string|null;match:'exact'|'suggested'|'unmatched'};
const fail=(message:string):never=>{throw new AppError(422,message)};
const key=(s:string)=>s.normalize('NFKC').toLowerCase().replace(/[^\p{L}\p{N}]/gu,'');
const plain=(s:string)=>s.replace(/&(#x[0-9a-f]+|#\d+|amp|lt|gt|quot|apos);/gi,(_,v:string)=>{if(v[0]==='#'){const n=v[1].toLowerCase()==='x'?parseInt(v.slice(2),16):Number(v.slice(1));return n>0&&n<=0x10ffff&&!(n>=0xd800&&n<=0xdfff)?String.fromCodePoint(n):''}return ({amp:'&',lt:'<',gt:'>',quot:'"',apos:"'"}as Record<string,string>)[v.toLowerCase()]}).replace(/\u00a0/g,' ');
type Node={name:string;children:Node[];value:string};
/** Bounded, inert OOXML reader. It never opens relationships, executes fields or interprets document instructions. */
function xmlTree(xml:string){
 if(/<!DOCTYPE|<!ENTITY|<!\[CDATA\[/i.test(xml))fail('This Word document contains unsupported XML declarations');
 const root:Node={name:'root',children:[],value:''},stack=[root];let at=0,nodes=0;
 const token=/<(?:[^<>"']|"[^"]*"|'[^']*')*>|[^<]+/g;
 for(const m of xml.matchAll(token)){if(m.index!==at)fail('The Word document XML is damaged');at+=m[0].length;const t=m[0];if(t.startsWith('<?')||t.startsWith('<!--'))continue;
  if(t[0]!=='<'){stack.at(-1)!.value+=plain(t);continue}
  if(t.startsWith('</')){const name=t.slice(2,-1).trim();if(stack.length===1||stack.at(-1)!.name!==name)fail('The Word document XML is damaged');stack.pop();continue}
  const name=/^<([^\s/>]+)/.exec(t)?.[1];if(!name)fail('The Word document XML is damaged');if(++nodes>150000||stack.length>64)fail('This Word document is too complex to import');const next:Node={name:name!,children:[],value:''};stack.at(-1)!.children.push(next);if(!t.endsWith('/>'))stack.push(next);
 }
 if(at!==xml.length||stack.length!==1)fail('The Word document XML is damaged');return root;
}
const local=(n:Node)=>n.name.split(':').at(-1);
function descendants(n:Node,name:string):Node[]{return n.children.flatMap(c=>local(c)===name?[c]:descendants(c,name))}
function text(n:Node):string {const name=local(n);if(name==='t')return n.value;if(name==='br'||name==='cr')return '\n';if(name==='tab')return ' ';if(name==='del'||name==='instrText')return '';const s=n.children.map(text).join('');return name==='p'?s+'\n':s}
function section(s:string){const v=s.toLowerCase();if(/work location.*risk/.test(v))return'*';if(/tender.*(?:cost|fee)/.test(v))return'tender_fee';if(/earnest|\bemd\b|bid security/.test(v))return'emd';if(/financial|turn.?over/.test(v))return'financial';if(/electrical.*licen/.test(v))return'electrical_licence';if(/security deposit|performance.*guarantee/.test(v))return'security';if(/similar|technical.*(?:work|compliance|eligibility)/.test(v))return'similar_work';if(/payment/.test(v))return'payment';if(/work location/.test(v))return'work_location';if(/risk|commercial conditions/.test(v))return'risk';if(/conclusion|management decision/.test(v))return'conclusion';if(/additional|other.*(?:condition|requirement|question)/.test(v))return'other';if(/tender.*(?:particular|parameter)/.test(v))return'particulars';return''}
const words=(s:string)=>new Set(s.toLowerCase().replace(/joint venture/g,'jv').replace(/licence/g,'license').replace(/\bca\b/g,'chartered accountant ca').match(/[\p{L}\p{N}]{2,}/gu)?.filter(w=>!['the','this','tender','any','for','and','from','required','please','kindly'].includes(w))??[]);
function similarity(a:string,b:string){const x=words(a),y=words(b);const overlap=[...x].filter(w=>y.has(w)).length;return .65*(2*overlap/Math.max(1,x.size+y.size))+.35*(overlap/Math.max(2,Math.min(x.size,y.size)))}
const empty=(s:string)=>/^(?:no answer received\.?|management review required\.?|not filled in\.?|needs review|answer ready to review)$/i.test(s.trim());
export function parseWord(data:Buffer,questions:Question[]):{rows:ImportRow[];warnings:string[]}{
 if(data.length<22||data.length>4*1024*1024||data.subarray(0,2).toString()!=='PK')fail('Upload a valid .docx Word file up to 4 MiB');
 let entries:Record<string,Uint8Array>;let count=0,total=0;
 try{entries=unzipSync(data,{filter:f=>{if(++count>512||!Number.isSafeInteger(f.originalSize)||f.originalSize<0||(total+=f.originalSize)>48*1024*1024)fail('Word archive exceeds the import limits');if(/vbaProject|embeddings\//i.test(f.name))fail('Macro and embedded-file documents are not supported');const chosen=['word/document.xml','[Content_Types].xml'].includes(f.name);if(chosen&&(f.originalSize>6*1024*1024||f.size>4*1024*1024))fail('Word document text exceeds the import limit');return chosen}})}catch(e){if(e instanceof AppError)throw e;return fail('This Word document is damaged or unsupported')}
 if(!entries['word/document.xml']||!entries['[Content_Types].xml'])fail('Upload a .docx document, not a ZIP or legacy .doc file');
 if(!strFromU8(entries['[Content_Types].xml']).includes('application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml'))fail('Upload a standard .docx document without macros');
 const xml=strFromU8(entries['word/document.xml']);if(!xml.includes('http://schemas.openxmlformats.org/wordprocessingml/2006/main'))fail('Unsupported Word document format');
 const tree=xmlTree(xml),body=descendants(tree,'body')[0];if(!body)fail('Word document body is missing');const rows:ImportRow[]=[],warnings:string[]=[];let current='';
 for(const node of body.children){if(local(node)==='p'){current=section(text(node))||current;continue}if(local(node)!=='tbl')continue;let qcol=-1,acol=-1;const first=node.children.find(n=>local(n)==='tr'),head=first?.children.filter(n=>local(n)==='tc').map(text)??[];if(head.some(s=>/S\.?No|No\./i.test(s))&&!head.some(s=>/answer|compliance|response|detail|remark/i.test(s))){warnings.push('An additional table was kept in the original Word file and excluded from question matching.');continue}
  for(const tr of node.children.filter(n=>local(n)==='tr')){const cells=tr.children.filter(n=>local(n)==='tc').map(n=>text(n).trim());if(cells.length<2)continue;
   const headerQ=cells.findIndex(s=>/^(?:checklist question|question|tender selection parameter|parameter|particular|description)\b/i.test(s));const headerA=cells.findIndex(s=>/^(?:answer|details|compliance|response|remark)\b/i.test(s));
   if(headerQ>=0&&headerA>=0){qcol=headerQ;acol=headerA;continue}
   if(cells.every(s=>!s))continue;if(cells.filter(Boolean).length===1){current=section(cells.find(Boolean)!)||current;continue}
   const qi=qcol>=0?qcol:cells.length>=3&&/^[\d.()\s-]+$/.test(cells[0])?1:0,ai=acol>=0?acol:qi+1;if(!cells[qi]||ai>=cells.length)continue;
   const question=cells[qi].trim(),raw=cells[ai].trim();if(question.length>3000||raw.length>12000)fail('A checklist row is too long; split it before importing');if(rows.length>=240)fail('Import up to 240 checklist rows at a time');
   const answer=empty(raw)?'':raw.replace(/\nSource: [^\n]+/g,'').replace(/\n(?:Needs review|Conflicting evidence|Reviewed)\s*$/i,'').trim();if(Buffer.byteLength(answer)>8000)fail('An answer exceeds 8,000 bytes; shorten it before importing');
   const candidates=questions.filter(q=>!current||current==='*'||q.section===current),exact=candidates.filter(q=>key(q.question)===key(question)),rank=candidates.map(q=>({q,score:similarity(question,q.question)})).sort((a,b)=>b.score-a.score);let match:ImportRow['match']='unmatched',question_id:string|null=null;
   if(exact.length===1){match='exact';question_id=exact[0].id}else if(rank[0]?.score>=.7&&(!rank[1]||rank[0].score-rank[1].score>=.12)){match='suggested';question_id=rank[0].q.id}
   if(!question_id&&current&&qi>0){const numeric=cells[0].trim(),repair=candidates.length>1&&candidates.every(q=>/^\d+$/.test(q.printed_number))&&new Set(candidates.map(q=>q.printed_number)).size<candidates.length;const numbered=candidates.filter((q,i)=>(repair?String(i+1):q.printed_number)===numeric);if(numbered.length===1&&similarity(question,numbered[0].question)>=.35){question_id=numbered[0].id;match='suggested'}}
   rows.push({id:rows.length+1,section:current,number:qi>0?cells[0]:'',question,answer,question_id,match});
  }
 }
 if(!rows.length)fail('No question-and-answer table was found. Use a Word checklist with Question and Answer columns.');
 const mapped=new Map<string,ImportRow[]>();for(const row of rows)if(row.question_id)mapped.set(row.question_id,[...(mapped.get(row.question_id)??[]),row]);for(const group of mapped.values())if(group.length>1)for(const row of group){row.question_id=null;row.match='unmatched'}
 if(/<(?:\w+:)?(?:ins|del)\b/.test(xml))warnings.push('Tracked changes were found. Check the imported text against the original document.');if(rows.some(r=>!r.question_id))warnings.push('Some rows need a question match or must be skipped.');return{rows,warnings};
}
