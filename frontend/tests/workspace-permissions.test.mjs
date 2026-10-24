import assert from "node:assert/strict";
import test from "node:test";
import { readFileSync } from "node:fs";
import ts from "typescript";
const { outputText } = ts.transpileModule(readFileSync(new URL("../src/components/projects/workspace-permissions.ts",import.meta.url),"utf8"),{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022}});
const {indentTabs,purchaseStepAllowed,projectTabAllowed}=await import(`data:text/javascript;base64,${Buffer.from(outputText).toString("base64")}`);
test("item grouping and vendor allocation can be assigned independently",()=>{
 const read="projects.purchase_indent_groups.read";
 assert.deepEqual(indentTabs([read,"projects.purchase_indent_groups.view"]),["assignment"]);
 assert.deepEqual(indentTabs([read,"projects.purchase_group_vendors.view"]),["vendors"]);
 assert.deepEqual(indentTabs(["projects.purchase_group_vendors.view"]),[]);
 assert.deepEqual(indentTabs([read,"projects.purchase_indent_register.view"]),[]);
});
test("receipt users cannot navigate to comparison and vice versa",()=>{
 for(const [area,step,other] of [["group_quotations",3,4],["group_comparisons",4,3]]){
  const access=[`projects.${area}.view`,`projects.${area}.read`];
  assert.ok(purchaseStepAllowed(access,step));
  assert.equal(purchaseStepAllowed(access,other),false);
  assert.ok(projectTabAllowed(access,"purchasing"));
  assert.equal(projectTabAllowed(access,"budget"),false);
 }
});
test("a write or reserved approve grant does not open a tab",()=>{
 assert.equal(purchaseStepAllowed(["projects.group_quotations.write"],3),false);
 assert.equal(purchaseStepAllowed(["projects.group_awards.approve"],5),false);
 assert.ok(purchaseStepAllowed(["projects.group_awards.view"],5));
 assert.equal(projectTabAllowed(["projects.budgets.write"],"budget"),false);
 assert.equal(projectTabAllowed([],"team"),false);
 assert.equal(purchaseStepAllowed([],2),false);
});

test("Formal indent access opens Step 2 without vendor-group access",()=>{assert.equal(purchaseStepAllowed(["projects.purchase_indents.view","projects.purchase_indents.read"],2),true);assert.equal(purchaseStepAllowed(["projects.purchase_indents.write"],2),false)});
