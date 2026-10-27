import assert from "node:assert/strict";
import test from "node:test";
import { readFileSync } from "node:fs";
import ts from "typescript";
const source=readFileSync(new URL("../src/components/tenders/tsc-presentation.ts",import.meta.url),"utf8");
const {outputText}=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.ESNext}});
const {checklistAnswer,editableTSCAnswer,compactTSCText}=await import(`data:text/javascript;base64,${Buffer.from(outputText).toString("base64")}`);
test("checklist displays answers without processing labels and missing answers without explanations",()=>{
 assert.equal(checklistAnswer({status:"answered",answer:"Answered: 90 days excluding monsoon"}),"90 days excluding monsoon");
 assert.equal(checklistAnswer({status:"not_found",answer:"Not found in source 4 at page 10"}),"");
 assert.equal(checklistAnswer({status:"needs_review",answer:"Company approval is pending"}),"");
 assert.equal(checklistAnswer({status:"conflicting",answer:"90 or 120 days"}),"");
});
test("answer editor leaves provider failures blank while preserving real answers and reviewed text",()=>{
 assert.equal(editableTSCAnswer({status:"needs_review",origin:"rejected_ollama_proposal",answer:"The local model proposal failed source-evidence validation. Review originals."}),"");
 assert.equal(editableTSCAnswer({status:"needs_review",origin:"cloud_review_required",answer:"Cloud proposal unavailable or allowance exhausted."}),"");
 assert.equal(editableTSCAnswer({status:"needs_review",origin:"manual_draft",answer:"Not yet extracted."}),"");
 assert.equal(editableTSCAnswer({status:"answered",origin:"ollama_proposal",answer:"Energy Department Uttar Pradesh"}),"Energy Department Uttar Pradesh");
 assert.equal(editableTSCAnswer({status:"needs_review",origin:"manual",answer:"Awaiting the client's reviewed credentials"}),"Awaiting the client's reviewed credentials");
});

test("model checklist lines show direct amounts and applicability without explanation", () => {
 const verbose = "The tender value without GST is INR 3,74,62,212.00. This is stated in the document (S2). The bid value is INR 37462212.";
 assert.equal(editableTSCAnswer({question_id:"particulars.q05",status:"answered",origin:"ollama_proposal",answer:verbose}), "INR 3,74,62,212.00");
 assert.equal(editableTSCAnswer({question_id:"particulars.q05",status:"answered",origin:"ollama_proposal",accepted:true,answer:verbose}), verbose);
 assert.equal(editableTSCAnswer({question_id:"particulars.q05",status:"answered",origin:"manual",answer:verbose}), verbose);
 assert.equal(compactTSCText("particulars.q05", "Between INR 100 and INR 200"), "Between INR 100 and INR 200");
 assert.equal(compactTSCText("particulars.q05", "Approximately INR 100 crore"), "Approximately INR 100 crore");
 assert.equal(compactTSCText("particulars.q06", "Reverse Auction (RA) is applicable to the first 50% of technically qualified bidders, arranged in order of prices from lowest to highest. If the number is odd, round up. For example, 7 bids means 4 bidders."), "Yes; the first 50% of technically qualified bidders, ranked by price (lowest first)");
 assert.equal(compactTSCText("particulars.q07", "90 days from LOA. Except monsoon delays. This is stated in the document (S1)."), "90 days from LOA; Except monsoon delays");
});

test("status-only model text is blank while real yes/no and values remain usable", () => {
 for (const text of ["answered", "Answered:", "needs_review", "Not found", "unknown"]) {
  assert.equal(checklistAnswer({status:"answered",answer:text}), "");
  assert.equal(editableTSCAnswer({status:"answered",origin:"ollama_proposal",answer:text}), "");
 }
 assert.equal(checklistAnswer({status:"answered",answer:"Yes"}), "Yes");
 assert.equal(checklistAnswer({status:"answered",answer:"INR 123.00"}), "INR 123.00");
});
