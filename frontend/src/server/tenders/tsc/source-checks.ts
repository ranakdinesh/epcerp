import type {Answer,Question,Source,Citation} from './types';
import type {PDFText} from './text-evidence';
type Clause={quote:string;citation:Citation};
const normal=(s:string)=>s.normalize('NFKC').replace(/\u00ad/g,'').replace(/\s+/g,' ').trim();
/** Source clauses supplement the draft; they never mark a manager's answer accepted. No tender values are hard-coded. */
export function checkSourceClauses(questions:Question[],answers:Answer[],documents:{doc:Source;text:PDFText}[]){
 const pages=documents.flatMap(({doc,text})=>text.pages.map(p=>({id:doc.id,page:p.page,text:normal(p.text)})));
 function clauses(pattern:RegExp):Clause[]{const seen=new Set<string>(),found:Clause[]=[];for(const p of pages){const match=pattern.exec(p.text),quote=match?.[0].trim();if(!quote||quote.length>1900)continue;const key=quote.toLowerCase().replace(/\bupto\b/g,'up to');if(seen.has(key))continue;seen.add(key);found.push({quote,citation:{document_id:p.id,page:p.page,quote}})}return found}
 function apply(a:Answer,groups:{label:string;clauses:Clause[]}[],complete=true){
  const found=groups.flatMap(g=>g.clauses);if(!found.length||a.accepted)return;
  const ambiguous=groups.some(g=>g.clauses.length>1)||a.status==='conflicting';
  a.answer=groups.filter(g=>g.clauses.length).map(g=>g.label+': '+g.clauses.map(c=>c.quote).join('\nAlternative source clause: ')).join('\n\n');a.citations=found.slice(0,8).map(c=>c.citation);a.status=ambiguous?'conflicting':complete?'answered':'needs_review';a.origin='source_check';
 }
 const sd=clauses(/The\s+Irrevocable\s+Bank\s+Guarantee\s+submitted\s+towards\s+Security\s+deposit.{0,250}?initially\s+valid.{0,300}?(?:\.\s|\.$)/i);
 const pg=clauses(/The\s+Performance\s+Guarantee\s+shall\s+be\s+submitted.{0,900}?initially\s+valid.{0,300}?(?:\.\s|\.$)/i);
 const general=clauses(/On\s*Account\s*Payment\s*will\s*be\s*made.{0,750}?Balance.{0,250}?after\s+testing\s*&?\s*commissioning\s*\./i);
 const led=clauses(/Payment\s+terms\s+for\s+LED.{0,180}?as\s+follows.{0,1400}?(?=\s+d[.)]\s+Normal\s+security|\s+b[.)]\s+For\s+the\s+period|$)/i);
 const oem=clauses(/Cable[s]?\s+shall\s+be\s+supplied\s+by\s+the\s+OEM.{0,250}?certificate\s*\./i);
 const tests=clauses(/Cable[s]?\s+shall\s+be\s+tested.{0,650}?along\s+with\s+(?:the\s+)?supply.{0,100}?\./i);
 const startup=clauses(/Any\s+firm\s+recognized\s+by\s+Department\s+of\s+Industrial\s+Policy\s+and\s+Promotion.{0,120}?Startups.{0,160}?exempted.{0,140}?Bid\s+Security.{0,70}?\./i);
 const bank=clauses(/Bank\s+reference\s+and\s+other\s+related\s+details\s*\.\s*\(Latest\)/i);
 const advertised=clauses(/Advertised\s+Value\s*[:=]?\s*(?:Rs\.?|INR|₹)?\s*[\d,]+(?:\.\d+)?/i);
 const gst=clauses(/All\s+rates\s+quoted.{0,220}?inclusive\s+of\s+Goods\s+and\s+Services\s+tax.{0,180}?(?:\.\s|\.$)/i);
 const manual=clauses(/Manual\s+offers\s+are\s+not\s+allowed.{0,240}?ignored\s*\./i);
 const delivery=clauses(/The\s+original\s+Bank\s+Guarantee\s+should\s+be\s+delivered.{0,180}?working\s+days.{0,140}?(?:\.\s|\.$)/i);
 const cooperative=clauses(/Labour\s+Cooperative\s+Societies\s+shall\s+submit\s+only\s+\d+(?:\.\d+)?\s*%\s+of\s+above\s+Bid\s+Security.{0,70}?\./i);
 for(const q of questions){const a=answers.find(a=>a.question_id===q.id);if(!a||a.accepted||q.manual_review||!a.answer.trim())continue;const question=q.question;
  if(/certificate.*bank|bank.*certificate/i.test(question)&&bank.length){apply(a,[{label:'Bank information listed in the tender guidelines',clauses:bank}],false);a.answer+='\nA latest bank reference is listed; this wording does not establish a separate solvency certificate amount or format.';continue}
  if(/value.*gst|gst.*value/i.test(question)&&advertised.length&&gst.length){apply(a,[{label:'Advertised value',clauses:advertised},{label:'Tax treatment of quoted rates',clauses:gst}],false);a.answer+='\nThese clauses do not by themselves establish the GST basis of the advertised total or a separate amount excluding GST.';continue}
  if(/hard copy|physical|offline/i.test(question)&&manual.length){apply(a,[{label:'Electronic offer rule',clauses:manual},{label:'Original bank-guarantee delivery provisions',clauses:delivery}],false);if(delivery.length>1)a.answer+='\nReview the applicable amendment and closing-date requirement before arranging delivery.';continue}
  if(/bank guarantee/i.test(question)&&/sd|pbg|performance|security/i.test(question)&&/period|valid/i.test(question)){apply(a,[{label:'Security Deposit bank guarantee',clauses:sd},{label:'Performance Guarantee',clauses:pg}],!!sd.length&&!!pg.length);continue}
  if(/payment terms|payment.*milestone/i.test(question)){apply(a,[{label:'General on-account supply payments',clauses:general},{label:'LED and BLDC payments',clauses:led}],!!general.length&&!!led.length);continue}
  if(/oem/i.test(question)&&/authori[sz]ation/i.test(question)){apply(a,[{label:'OEM or dealer condition',clauses:oem}]);continue}
  if(/oem/i.test(question)&&/document|certificate/i.test(question)){apply(a,[{label:'Cable testing and OEM warranty documents',clauses:tests}]);continue}
  if(/emd|bid security|bsd/i.test(question)&&/exempt/i.test(question)&&!/msme|udyam|declaration/i.test(question)){apply(a,[{label:'Startup exemption',clauses:startup},...(/startup/i.test(question)?[]:[{label:'Labour cooperative deposit rule',clauses:cooperative}])],!!startup.length)}
 }
 return answers;
}
