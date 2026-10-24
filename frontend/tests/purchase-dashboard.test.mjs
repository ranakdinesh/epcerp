import assert from "node:assert/strict";
import test from "node:test";
import { readFileSync } from "node:fs";
import ts from "typescript";
async function load(path) {
  const output = ts.transpileModule(
    readFileSync(new URL(path, import.meta.url), "utf8"),
    { compilerOptions: { module: ts.ModuleKind.ESNext } },
  ).outputText;
  return import(
    `data:text/javascript;base64,${Buffer.from(output).toString("base64")}`
  );
}
const { dashboardLanding } = await load(
  "../src/components/dashboards/landing-model.ts",
);
const { purchaseTasks, purchaseTaskHref, purchaseProjectAction } = await load(
  "../src/components/purchasing/purchase-dashboard-model.ts",
);
test("landing follows capabilities, distinguishing purchase operator, manager, executive and admin", () => {
  assert.equal(
    dashboardLanding([
      "projects.purchase_workflow.view",
      "projects.purchase_workflow.execute",
    ]),
    "purchase",
  );
  assert.equal(
    dashboardLanding([
      "projects.purchase_workflow.view",
      "projects.purchase_workflow.write",
    ]),
    "purchase",
  );
  assert.equal(
    dashboardLanding([
      "projects.purchase_workflow.read",
      "projects.dashboard.read",
    ]),
    "portfolio",
  );
  assert.equal(
    dashboardLanding([
      "projects.portfolio.write",
      "projects.purchase_workflow.view",
      "projects.purchase_workflow.write",
    ]),
    "manager",
  );
  assert.equal(dashboardLanding(["system:admin"]), "portfolio");
  assert.equal(dashboardLanding([]), "portfolio");
});
test("queue uses recorded dates, excludes terminal/source-linked steps and resolves captured blocker ownership", () => {
  const step = {
    number: 3,
    phase: "I2LOI",
    title: "Invite quotations",
    state: "not_started",
    planned_date: "2026-09-19",
    doer_id: "worker",
    resolver_id: "resolver",
    blocked_resolver_id: null,
  };
  const source = {
    project: { id: "project" },
    data: {
      server_time: "2026-09-20T08:00:00Z",
      members: [
        { id: "worker", label: "Purchase user" },
        { id: "resolver", label: "Resolver" },
      ],
      packages: [
        {
          id: "pkg",
          steps: [
            step,
            {
              ...step,
              number: 4,
              state: "blocked",
              doer_id: "other",
              resolver_id: "new",
              blocked_resolver_id: "worker",
              planned_date: "2026-09-20",
            },
            { ...step, number: 5, doer_id: null, planned_date: null },
            { ...step, number: 1, state: "source_linked" },
            { ...step, number: 6, state: "completed" },
            { ...step, number: 11, state: "not_applicable" },
          ],
        },
      ],
    },
  };
  const rows = purchaseTasks([source], "worker");
  assert.equal(rows.length, 3);
  assert.deepEqual(
    rows.map((r) => r.overdue),
    [true, false, false],
  );
  assert.deepEqual(
    rows.map((r) => r.mine),
    [true, true, false],
  );
  assert.equal(rows[0].doer, "Purchase user");
  assert.equal(rows[1].resolver, "Purchase user");
  assert.equal(rows[2].doer, "Unassigned");
});
test("empty sources stay empty and task links preserve exact package and step", () => {
  assert.deepEqual(purchaseTasks([], "worker"), []);
  const task = new URL(
    purchaseTaskHref("p", "package", 12),
    "http://localhost",
  );
  assert.equal(task.searchParams.get("purchaseStep"), "12");
  assert.equal(task.searchParams.get("purchase"), "package");
  assert.equal(task.searchParams.get("tab"), "purchasing");
  const start = new URL(purchaseTaskHref("p"), "http://localhost");
  assert.equal(start.searchParams.get("purchaseView"), "bo");
  assert.equal(start.searchParams.has("purchase"), false);
});

const { matchesPurchaseProject, purchaseProjectSignal } = await load(
  "../src/components/purchasing/purchase-dashboard-model.ts",
);
test("project lifecycle and weighted completion filters stay independent and absent progress is not zero", () => {
  const source = {
    project: { lifecycle_status: "closed" },
    progress: { progress_percent: 25, packages: [{ signal: "delayed" }] },
  };
  assert.equal(matchesPurchaseProject(source, "closed", "in_progress"), true);
  assert.equal(matchesPurchaseProject(source, "closed", "complete"), false);
  assert.equal(matchesPurchaseProject(source, "active", ""), false);
  assert.equal(
    matchesPurchaseProject(
      { ...source, project: { lifecycle_status: "draft" } },
      "active",
      "",
    ),
    true,
  );
  const absent = { ...source, progress: null };
  assert.equal(matchesPurchaseProject(absent, "", "not_started"), false);
  assert.equal(matchesPurchaseProject(absent, "", "unavailable"), true);
  assert.equal(purchaseProjectSignal(absent), "unavailable");
  assert.equal(
    purchaseProjectSignal({
      ...source,
      progress: { progress_percent: null, packages: [] },
    }),
    "unrecorded",
  );
  assert.equal(purchaseProjectSignal(source), "delayed");
  assert.equal(
    purchaseProjectSignal({
      ...source,
      progress: { progress_percent: 100, packages: [{ signal: "complete" }] },
    }),
    "complete",
  );
  assert.equal(
    purchaseProjectSignal({
      ...source,
      progress: {
        progress_percent: 25,
        packages: [{ signal: "blocked" }, { signal: "delayed" }],
      },
    }),
    "blocked",
  );
});

const progressSource = (packages) => ({ project: { id: "qa-project" }, progress: { packages } });
const bo = (id, states) => ({ bo_id: id, steps: states.map((state, i) => ({ number: i+1, title: ["BO item list", "Purchase indent", "Receive vendor quotations", "Price comparison"][i], state })) });
test("project action advances with saved completion and returns to reopened work", () => {
 const source = progressSource([bo("bo-a", ["completed", "completed", "in_progress", "not_started"])]);
 let action = purchaseProjectAction(source);
 assert.equal(action.label, "Step 3 · Receive vendor quotations");
 let query = new URL(action.href, "http://localhost").searchParams;
 assert.equal(query.get("bo"), "bo-a"); assert.equal(query.get("purchaseStep"), "3"); assert.equal(query.get("purchaseView"), "bo");
 source.progress.packages[0].steps[2].state="completed";
 assert.equal(purchaseProjectAction(source).label,"Step 4 · Price comparison");
 source.progress.packages[0].steps[1].state="in_progress";
 assert.equal(purchaseProjectAction(source).label,"Step 2 · Purchase indent");
});
test("project action selects the earliest unfinished BO deterministically and respects view access", () => {
 const source=progressSource([bo("bo-z",["completed","completed","in_progress"]),bo("bo-b",["completed","in_progress","not_started"]),bo("bo-a",["completed","in_progress","not_started"])]);
 assert.equal(new URL(purchaseProjectAction(source).href,"http://localhost").searchParams.get("bo"),"bo-a");
 assert.equal(new URL(purchaseProjectAction(source,n=>n===3).href,"http://localhost").searchParams.get("purchaseStep"),"3");
 assert.equal(purchaseProjectAction(source,()=>false).label,"Open purchase workspace");
});
test("complete, not-applicable, empty and unavailable reports do not invent pending progress", () => {
 assert.equal(purchaseProjectAction(progressSource([bo("bo-a",["completed","not_applicable"])] )).label,"Purchase complete · View records");
 assert.equal(purchaseProjectAction(progressSource([])).label,"Step 1 · Prepare BO item list");
 assert.equal(purchaseProjectAction({project:{id:"qa-project"},progress:null}).label,"Open purchase workspace");
});
