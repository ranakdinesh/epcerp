import test from'node:test';import assert from'node:assert/strict';import{checkSourceClauses}from'../../src/server/tenders/tsc/source-checks';import type{Answer,Question,Source}from'../../src/server/tenders/tsc/types';
const q=(id:string,question:string):Question=>({id,question,section:'test',printed_number:'1',manual_review:false});const a=(id:string):Answer=>({question_id:id,answer:'An incomplete model draft',status:'answered',citations:[],accepted:false,origin:'gemini'});
const document=(id:string,text:string)=>({doc:{id,path:id+'.pdf'}as Source,text:{reader_version:'pdf-text-v1',pages:[{page:1,text,method:'native'}],warnings:[]}});
test('source validation separates SD and PG validity and preserves reviewed wording',()=>{
 const qs=[q('bg','What is the time period for Bank Guarantee applicable to SD/PBG?')],answers=[a('bg')],docs=[document('gcc','The Irrevocable Bank Guarantee submitted towards Security deposit shall be initially valid up to the Maintenance period plus 47 days. The Performance Guarantee shall be submitted before agreement. This P.G. shall be initially valid up to the date of completion plus 83 days. ')];
 checkSourceClauses(qs,answers,docs);assert.match(answers[0].answer,/Security Deposit bank guarantee:.*47 days/);assert.match(answers[0].answer,/Performance Guarantee:.*83 days/);assert.equal(answers[0].origin,'source_check');assert.equal(answers[0].accepted,false);assert.equal(answers[0].citations.length,2);
 const accepted={...a('bg'),accepted:true};checkSourceClauses(qs,[accepted],docs);assert.equal(accepted.answer,'An incomplete model draft');
});
test('source validation uses source percentages and keeps general and special payments distinct',()=>{
 const qs=[q('pay','What are the payment terms and milestones?')],answers=[a('pay')],docs=[document('prices','On Account Payment will be made on supply at 63 percent. Balance 37 percent will be paid after testing & commissioning. Payment terms for LED luminaries and BLDC Fans are as follows: a. 51 percent on supply b. 19 percent on commissioning c. 30 percent over warranty. d. Normal security deposit is separate.')];
 checkSourceClauses(qs,answers,docs);assert.match(answers[0].answer,/63 percent/);assert.match(answers[0].answer,/37 percent/);assert.match(answers[0].answer,/LED and BLDC payments/);assert.match(answers[0].answer,/19 percent/);assert.doesNotMatch(answers[0].answer,/Normal security deposit/);
});
test('missing evidence does not invent clauses or fill unanswered model slots',()=>{
 const answers=[a('oem'),{...a('blank'),answer:''}];checkSourceClauses([q('oem','Any authorization from OEM required?'),q('blank','What are the payment terms?')],answers,[document('empty','No matching clause here.')]);assert.equal(answers[0].answer,'An incomplete model draft');assert.equal(answers[1].answer,'');
});
test('tax-inclusive rates do not prove advertised GST basis; latest bank reference remains qualified',()=>{
 const answers=[a('gst'),a('bank')];checkSourceClauses([q('gst','Tender value with GST'),q('bank','Any type of Certificate from Bank?')],answers,[document('nit','Advertised Value 12345.67 All rates quoted in the tenders shall be inclusive of Goods and Services tax, duties. Bank reference and other related details. (Latest)')]);assert.equal(answers[0].status,'needs_review');assert.match(answers[0].answer,/do not by themselves establish/);assert.equal(answers[1].status,'needs_review');assert.match(answers[1].answer,/does not establish a separate solvency/);
});
