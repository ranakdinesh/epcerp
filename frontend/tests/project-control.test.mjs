import assert from "node:assert/strict";
import test from "node:test";
import { readFileSync } from "node:fs";
import ts from "typescript";
const source = readFileSync(
  new URL(
    "../src/components/projects/project-control-model.ts",
    import.meta.url,
  ),
  "utf8",
);
const output = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.ESNext,
    target: ts.ScriptTarget.ES2022,
  },
}).outputText;
const { downstream, overdue, day, iso, controlModel, pendingDependencies } =
  await import(
    `data:text/javascript;base64,${Buffer.from(output).toString("base64")}`
  );
test("transitive impact handles branches, merges and malformed cycles without counting source", () => {
  const links = [
    ["a", "b"],
    ["a", "c"],
    ["b", "d"],
    ["c", "d"],
    ["d", "a"],
  ].map(([from, to]) => ({ from, to }));
  assert.deepEqual([...downstream("a", links)].sort(), ["b", "c", "d"]);
  assert.deepEqual([...downstream("unknown", links)], []);
});
test("date-only overruns exclude completed, future and undated work", () => {
  assert.equal(
    overdue({ status: "blocked", finish: "2026-09-01" }, "2026-09-08"),
    7,
  );
  assert.equal(
    overdue({ status: "completed", finish: "2026-09-01" }, "2026-09-08"),
    0,
  );
  assert.equal(
    overdue({ status: "not_started", finish: null }, "2026-09-08"),
    0,
  );
  assert.equal(
    overdue({ status: "not_started", finish: "2026-09-10" }, "2026-09-08"),
    0,
  );
  assert.equal(day("2026-03-09") - day("2026-03-08"), 1);
  assert.equal(iso(day("2026-09-08")), "2026-09-08");
});
test("task grouping preserves unknown process links and never invents dependency edges or progress", () => {
  const board = {
    processes: [
      {
        id: "p",
        name: "Survey",
        position: 2,
        status: "not_started",
        dependencies_confirmed: false,
      },
    ],
    owners: [],
    dependencies: [],
  };
  const task = {
    id: "t",
    process_id: "p",
    code: "A",
    name: "Inspect",
    is_active: true,
    status: "completed",
  };
  const result = controlModel(
    board,
    [task, { ...task, id: "inactive", is_active: false }],
    [],
    [],
  );
  assert.equal(result.rows.length, 2);
  assert.equal(result.rows[1].parent, "p");
  assert.equal(result.rows[0].status, "not_started");
  assert.equal(result.links.length, 0);
});

test("completed intermediate work stops pending dependency exposure", () => {
  const rows = [
    { id: "a", status: "blocked" },
    { id: "b", status: "completed" },
    { id: "c", status: "not_started" },
  ];
  const links = pendingDependencies(rows, [
    { from: "a", to: "b" },
    { from: "b", to: "c" },
  ]);
  assert.deepEqual([...downstream("a", links)], []);
});
