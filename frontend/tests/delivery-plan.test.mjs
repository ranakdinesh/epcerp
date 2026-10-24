import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import ts from "typescript";
const source = readFileSync(
  new URL("../src/components/projects/delivery-plan-data.ts", import.meta.url),
  "utf8",
);
const output = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.ESNext,
    target: ts.ScriptTarget.ES2022,
  },
}).outputText;
const { planDates, outline, siblingTotal, deliveryEdges, blockedMinutes } =
  await import(
    `data:text/javascript;base64,${Buffer.from(output).toString("base64")}`
  );
// Owned synthetic graph: a parallel purchasing branch joins site preparation.
// This tests the renderer algorithms, not an unpublished operational template.
const node=(code,parent_code,weight_bp,predecessors=[],start_bp=0,finish_bp=10000)=>({code,parent_code,weight_bp,predecessors,start_bp,finish_bp});
const nodes=[
 node("setup",null,2000),node("buy",null,4000),node("site",null,4000),
 node("setup.a","setup",5000),node("setup.b","setup",5000,[],2000,8000),
 node("buy.a","buy",5000,["setup"]),node("buy.b","buy",5000,["buy.a"]),
 node("site.a","site",5000,["setup"]),node("site.b","site",5000,["buy","site.a"]),
];
test("owned graph preserves nested allocations and parent-first outline", () => {
 for (const n of nodes) assert.equal(siblingTotal(n,nodes),100);
 assert.deepEqual(outline(nodes).map(n=>n.code),["setup","setup.a","setup.b","buy","buy.a","buy.b","site","site.a","site.b"]);
});
test("dates are bounded by parents and unrelated to progress weighting", () => {
  const dates = planDates(nodes, "2026-01-01", "2026-12-31");
  const changed = planDates(
    nodes.map((n) => ({ ...n, weight_bp: 1 })),
    "2026-01-01",
    "2026-12-31",
  );
  assert.deepEqual(dates, changed);
  for (const n of nodes) {
    const [a, b] = dates.get(n.code);
    const [pa, pb] = n.parent_code
      ? dates.get(n.parent_code)
      : ["2026-01-01", "2026-12-31"];
    assert.ok(a >= pa && b <= pb && b >= a);
  }
  assert.throws(() => planDates(nodes, "2026-12-31", "2026-01-01"));
});
test("summary prerequisites expand to exact leaf edges across parallel tracks", () => {
 const {edges,leaves}=deliveryEdges(nodes);
 assert.deepEqual(leaves("setup"),["setup.a","setup.b"]);
 assert.deepEqual(edges.filter(e=>e.to==="site.b").map(e=>e.from).sort(),["buy.a","buy.b","site.a"]);
 assert.ok(edges.some(e=>e.from==="setup.b"&&e.to==="buy.a"));
 assert.ok(!edges.some(e=>e.from==="site.a"&&e.to==="buy.a"));
});

test("summary blocked time merges overlapping child intervals instead of double counting", () => {
  const issues = [
    {
      node_id: "a",
      opened_at: "2026-01-01T00:00:00Z",
      resolved_at: "2026-01-01T02:00:00Z",
    },
    {
      node_id: "b",
      opened_at: "2026-01-01T01:00:00Z",
      resolved_at: "2026-01-01T03:00:00Z",
    },
    { node_id: "a", opened_at: "2026-01-01T04:00:00Z", resolved_at: null },
  ];
  assert.equal(
    blockedMinutes(issues, new Set(["a", "b"]), "2026-01-01T05:00:00Z"),
    240,
  );
  assert.equal(
    blockedMinutes(issues, new Set(["a"]), "2026-01-01T05:00:00Z"),
    180,
  );
});
