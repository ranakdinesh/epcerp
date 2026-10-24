import assert from "node:assert/strict";
import test from "node:test";
import { readFileSync } from "node:fs";
import ts from "typescript";
const source = readFileSync(
  new URL("../src/components/purchasing/fms-model.ts", import.meta.url),
  "utf8",
);
const { outputText } = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.ESNext,
    target: ts.ScriptTarget.ES2022,
  },
});
const {
  FMS_STEPS,
  currentStep,
  packageStatus,
  examplePackages,
  updateExample,
} = await import(
  `data:text/javascript;base64,${Buffer.from(outputText).toString("base64")}`
);
test("all source stages retain their five groups and distinct TAT/weight", () => {
  assert.deepEqual(
    FMS_STEPS.map((s) => s.number),
    Array.from({ length: 20 }, (_, i) => i + 1),
  );
  assert.deepEqual(
    [...new Set(FMS_STEPS.map((s) => s.phase))],
    ["WO2I", "I2LOI", "LOI2PO", "PO2INS", "INS2D"],
  );
  assert.equal(FMS_STEPS[11].tat, 10);
  assert.equal(FMS_STEPS[11].weight, 24);
});
test("completion requires current step and evidence, preserving the previous state", () => {
  const pkg = examplePackages()[0];
  assert.throws(
    () => updateExample(pkg, 5, "complete", "proof", ""),
    /current step/,
  );
  assert.throws(() => updateExample(pkg, 4, "complete", "  ", ""), /evidence/);
  const next = updateExample(pkg, 4, "complete", "CST-01", "Reviewed example");
  assert.equal(currentStep(next).number, 5);
  assert.equal(currentStep(pkg).number, 4);
  assert.equal(next.steps[3].evidence, "CST-01");
  assert.equal(next.history.length, pkg.history.length + 1);
});
test("holds override overdue and prevent progression until reasoned resolution", () => {
  const pkg = examplePackages()[0];
  assert.equal(packageStatus(pkg), "Overdue");
  assert.throws(() => updateExample(pkg, 4, "block", "", ""), /reason/);
  const held = updateExample(pkg, 4, "block", "", "Awaiting correction");
  assert.equal(packageStatus(held), "Blocked");
  assert.throws(
    () => updateExample(held, 4, "complete", "proof", ""),
    /Resolve/,
  );
  assert.throws(() => updateExample(held, 4, "resume", "", ""), /resolved/);
  const resumed = updateExample(held, 4, "resume", "", "Correction received");
  assert.equal(packageStatus(resumed), "Overdue");
  assert.equal(currentStep(resumed).number, 4);
});
test("only optional advance request can be skipped and only with a reason", () => {
  let pkg = examplePackages()[0];
  assert.throws(() => updateExample(pkg, 4, "skip", "", "Not needed"), /Only/);
  for (let n = 4; n < 11; n++)
    pkg = updateExample(pkg, n, "complete", "Example reference", "");
  assert.throws(() => updateExample(pkg, 11, "skip", "", ""), /reason/);
  pkg = updateExample(pkg, 11, "skip", "", "No advance in these example terms");
  assert.equal(pkg.steps[10].state, "not_applicable");
  assert.equal(currentStep(pkg).number, 12);
});
test("completed packages have no next action and reject further writes; examples reset independently", () => {
  const pkg = examplePackages()[4];
  assert.equal(currentStep(pkg), undefined);
  assert.equal(packageStatus(pkg), "Completed");
  assert.throws(
    () => updateExample(pkg, 20, "complete", "proof", ""),
    /current step/,
  );
  const a = examplePackages(),
    b = examplePackages();
  a[0].steps[0].evidence = "changed";
  assert.notEqual(a[0].steps[0].evidence, b[0].steps[0].evidence);
});
