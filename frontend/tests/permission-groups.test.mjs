import assert from "node:assert/strict";
import test from "node:test";
import { readFileSync } from "node:fs";
import ts from "typescript";
const { outputText } = ts.transpileModule(readFileSync(new URL("../src/components/roles/permission-groups.ts", import.meta.url), "utf8"), { compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 } });
const { groupPermissions } = await import(`data:text/javascript;base64,${Buffer.from(outputText).toString("base64")}`);
const permissions = [
  { code: "settings.items.write", description: "Maintain SKU records" },
  { code: "projects.items.read", description: "Read project quantities" },
  { code: "settings.items.view", description: "Open item bank" },
  { code: "system:admin", description: "Full platform administration" },
  { code: "legacy", description: "Old permission" },
];
test("separates identical resource names by area and preserves legacy codes", () => {
  const groups = groupPermissions(permissions);
  assert.equal(groups.length, 4);
  assert.equal(groups.flatMap((group) => group.permissions).length, permissions.length);
  assert.deepEqual(groups.find((group) => group.key === "settings.items").permissions.map((p) => p.code), ["settings.items.view", "settings.items.write"]);
  assert.equal(groups.find((group) => group.key === "projects.items").area, "Projects");
});
test("searches area labels, capabilities, codes and descriptions without mutating input", () => {
  assert.equal(groupPermissions(permissions, " Settings ").flatMap((g) => g.permissions).length, 2);
  assert.equal(groupPermissions(permissions, "SKU")[0].permissions[0].code, "settings.items.write");
  assert.equal(groupPermissions(permissions, "projects.items.read")[0].permissions.length, 1);
  assert.deepEqual(groupPermissions(permissions, "missing"), []);
  assert.equal(permissions.length, 5);
});

test("business metadata puts purchasing and finance outside Projects", () => {
  const groups = groupPermissions([
    { code: "projects.budgets.read", description: "Read budgets", group: "accounts" },
    { code: "projects.group_quotations.read", description: "Read quotations", group: "purchasing" },
    { code: "projects.new_finance_resource.read", description: "Future finance", group: "accounts" },
  ]);
  assert.equal(groups.find(g => g.key === "accounts.budgets").area, "Accounts / Finance");
  assert.equal(groups.find(g => g.key === "purchasing.group_quotations").area, "Purchase Workflow");
  assert.equal(groups.find(g => g.key === "accounts.new_finance_resource").area, "Accounts / Finance");
  assert.equal(groupPermissions(groups.flatMap(g => g.permissions), "finance").length, 2);
});
