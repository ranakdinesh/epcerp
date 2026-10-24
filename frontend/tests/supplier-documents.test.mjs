import assert from "node:assert/strict";
import test from "node:test";
import {readFileSync} from "node:fs";
import {createElement} from "react";
import {renderToStaticMarkup} from "react-dom/server";
import ts from "typescript";
const data=source=>`data:text/javascript;base64,${Buffer.from(source).toString("base64")}`;
const compile=source=>ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022,jsx:ts.JsxEmit.ReactJSX}}).outputText;
const stub=data(`import {createElement} from ${JSON.stringify(import.meta.resolve("react"))}; export const SupplierDocumentUpload=()=>null;export const SupplierDocumentActions=()=>null;export const SupplierForm=()=>createElement("div",null,"Supplier details form"); export const SupplierEvidence=()=>createElement("div",null,"Supplier evidence"); export const SupplierReviewAssignment=()=>null;export const SupplierPDFViewer=()=>null;export const RecordDrawer=()=>null; export const displayServerErrors=()=>false;export const supplierRequest=()=>{};export const failureMessage=()=>"Failure"; export const ValidatedForm=({children,...props})=>createElement("form",props,children); export const ValidatedLabel=({children})=>createElement("label",null,children);export const ValidatedInput=props=>createElement("input",props);export const ValidatedSelect=props=>createElement("select",props);export const ValidatedTextarea=()=>null;export const DataGrid=()=>null;export const GridAction=()=>null;export const documentReviewRows=()=>[];export const documentDecisionUpdate=()=>({});export const documentTypeNames={pan:"PAN",gst:"GST",cheque:"Cancelled cheque",other:"Other"};export default {};`);
async function load(name){let source=compile(readFileSync(new URL(`../src/components/items/${name}.tsx`,import.meta.url),"utf8"));source=source.replaceAll('"react/jsx-runtime"',JSON.stringify(import.meta.resolve("react/jsx-runtime"))).replaceAll('"react"',JSON.stringify(import.meta.resolve("react"))).replace(/from "(?:@[^"]*|\.[^"]*)"/g,`from ${JSON.stringify(stub)}`);return import(data(source));}
const {SupplierEditor}=await load("supplier-editor");const {SupplierEvidence}=await load("supplier-evidence");
const access={canReadDocuments:true,canWriteDocuments:true,canReview:false,onSaved:()=>{}};
test("new supplier setup explains the document step and waits for an owned supplier identity",()=>{const html=renderToStaticMarkup(createElement(SupplierEditor,access));assert.match(html,/Save supplier details first/);assert.match(html,/<button[^>]* disabled=""[^>]*>2\. Documents/);assert.match(html,/PAN, cancelled cheque and applicable GST/);});
test("existing supplier edit exposes the enabled document step",()=>{const html=renderToStaticMarkup(createElement(SupplierEditor,{...access,record:{id:"supplier",version:1,data:{},code:"SUP",name:"Supplier"}}));assert.doesNotMatch(html,/<button[^>]* disabled=""[^>]*>2\. Documents/);assert.match(html,/>2\. Documents/);assert.doesNotMatch(html,/Save supplier details first/);});
test("document writers can reach upload independently of document read authority",()=>{const writer=renderToStaticMarkup(createElement(SupplierEvidence,{supplierId:"supplier",canReadDocuments:false,canWriteDocuments:true,canReview:false}));assert.match(writer,/Upload document/);const reader=renderToStaticMarkup(createElement(SupplierEvidence,{supplierId:"supplier",canReadDocuments:true,canWriteDocuments:false,canReview:false}));assert.doesNotMatch(reader,/Upload document/);});


const helpers=await import(data(compile(readFileSync(new URL("../src/components/items/supplier-document-review.ts",import.meta.url),"utf8"))));
const documents=[{id:"pan",doc_type:"pan",title:"PAN certificate",is_active:true},{id:"gst",doc_type:"gst",title:"GST certificate",is_active:true},{id:"old",doc_type:"aadhar",is_active:false}];
const status={fingerprint:"now",missing_types:["cheque"],review:{version:2,fingerprint:"now",decisions:[{document_id:"pan",decision:"accept",comment:"Checked"}]}};
test("document grid uses saved approvals, pending and missing rows with archived evidence secondary",()=>{
 const rows=helpers.documentReviewRows(documents,status);
 assert.deepEqual(rows.map(r=>r.status),["Approved","Pending","Missing"]);
 assert.equal(rows[2].type,"cheque");assert.equal(helpers.documentReviewRows(documents,status,true).filter(r=>r.status==="Archived").length,1);
 assert.equal(helpers.documentReviewRows(documents,{...status,fingerprint:"changed"})[0].status,"Pending");
});
test("one-click document approval preserves current saved decisions and optimistic version",()=>{
 const command=helpers.documentDecisionUpdate(status,"gst","accept","Document reviewed and approved");
 assert.equal(command.version,2);assert.equal(command.fingerprint,"now");assert.equal(command.decisions.length,2);assert.equal(command.decisions[0].document_id,"pan");
 const correction=helpers.documentDecisionUpdate(status,"pan","reject","Wrong supplier");
 assert.equal(correction.decisions.length,1);assert.equal(correction.decisions[0].decision,"reject");
 const changed=helpers.documentDecisionUpdate({...status,fingerprint:"changed"},"gst","accept","Checked");
 assert.equal(changed.decisions.length,1);assert.match(changed.reason,/Approved supplier document/);
});

test("uploaders can see document decisions independently of confidential vendor review access",()=>{
 const rows=helpers.documentReviewRows([{id:"one",doc_type:"pan",title:"PAN",is_active:true,approval_state:"reject",approval_comment:"Wrong supplier"}],{fingerprint:"current",missing_types:[]});
 assert.equal(rows[0].status,"Rejected");assert.equal(rows[0].comment,"Wrong supplier");
 const accepted=helpers.documentReviewRows([{id:"one",doc_type:"pan",is_active:true,approval_state:"accept",approval_comment:"Automatically approved by upload permission"}],{fingerprint:"new-upload",missing_types:[]});
 assert.equal(accepted[0].status,"Approved");
});

const {SupplierDocumentActions}=await load("supplier-document-actions");
const {SupplierDocumentUpload}=await load("supplier-document-upload");
const actions={canWrite:true,canReview:true,busy:false,supplierActive:true,hasError:false,onUpload:()=>{},onView:()=>{},onApprove:()=>{},onReject:()=>{}};
test("writers can replace pending, rejected and approved files without opening preview first",()=>{
 for(const status of ["Pending","Rejected","Approved"]){
  const html=renderToStaticMarkup(createElement(SupplierDocumentActions,{...actions,row:{id:"gst",title:"GST certificate",type:"gst",status,document:{id:"gst",is_active:true}}}));
  assert.match(html,/aria-label="Replace GST certificate"/);
  assert.equal(html.includes('aria-label="Approve GST certificate"'),status!=="Approved");
 }
});
test("readers and archived rows never gain replace or approval actions",()=>{
 const row={id:"gst",title:"GST certificate",type:"gst",status:"Archived",document:{id:"gst",is_active:false}};
 for(const props of [{...actions,canWrite:false,canReview:false,row:{...row,status:"Pending",document:{...row.document,is_active:true}}},{...actions,row}]){
  const html=renderToStaticMarkup(createElement(SupplierDocumentActions,props));assert.doesNotMatch(html,/>Replace<|>Approve<|>Reject</);
 }
});
test("a failed approval keeps replacement available and stops repeated acceptance attempts",()=>{
 const html=renderToStaticMarkup(createElement(SupplierDocumentActions,{...actions,hasError:true,row:{id:"gst",title:"GST certificate",type:"gst",status:"Pending",document:{id:"gst",is_active:true}}}));
 assert.match(html,/>Replace</);assert.doesNotMatch(html,/>Approve</);assert.match(html,/>Reject</);
});
test("replacement editor stays in the panel and preserves other approvals",()=>{
 const html=renderToStaticMarkup(createElement(SupplierDocumentUpload,{supplierId:"supplier",supplierName:"Polycab Power Systems India Ltd",initialType:"gst",replacement:{id:"gst",title:"GST certificate"},onCancel:()=>{},onSaved:()=>{}}));
 assert.match(html,/aria-label="Replace GST"/);assert.match(html,/Polycab Power Systems India Ltd/);assert.match(html,/other documents keep their approvals/);assert.match(html,/Upload replacement/);assert.match(html,/name="replaces_document_id" value="gst"/);assert.match(html,/name="doc_type" value="gst"/);assert.doesNotMatch(html,/<select/);assert.doesNotMatch(html,/role="dialog"|Archive reason|Upload reason/);
});

test("missing PAN upload is preselected without suggesting an existing file",()=>{
 const html=renderToStaticMarkup(createElement(SupplierDocumentUpload,{supplierId:"supplier",supplierName:"Polycab Power Systems India Ltd",initialType:"pan",onCancel:()=>{},onSaved:()=>{}}));
 assert.match(html,/aria-label="Upload PAN"/);assert.match(html,/Polycab Power Systems India Ltd/);assert.match(html,/name="doc_type" value="pan"/);assert.doesNotMatch(html,/An existing|Replacing |replaces_document_id|<select/);
});
