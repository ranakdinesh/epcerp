import assert from "node:assert/strict";
import test from "node:test";
import { readFileSync } from "node:fs";
import ts from "typescript";
const source = readFileSync(new URL("../src/lib/auth/renew-session.ts", import.meta.url), "utf8");
const { outputText } = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 } });
const { renewSession, safeReturnPath } = await import(`data:text/javascript;base64,${Buffer.from(outputText).toString("base64")}`);

test("simultaneous idle requests rotate once and a later request can renew again", async () => {
  let count = 0;
  const fetcher = async (url, options) => {
    count++;
    assert.equal(url, "/api/auth/refresh");
    assert.equal(options.method, "POST");
    return new Response(null, { status: 200 });
  };
  const results = await Promise.all([renewSession(fetcher), renewSession(fetcher), renewSession(fetcher)]);
  assert.equal(count, 1);
  assert.ok(results.every(result => result.ok));
  await renewSession(fetcher);
  assert.equal(count, 2);
});
test("network failures allow a later retry", async () => {
  await assert.rejects(renewSession(async () => { throw new Error("offline"); }));
  assert.equal((await renewSession(async () => new Response(null, { status: 503 }))).status, 503);
});
test("return destinations stay within approved workspaces", () => {
  for (const value of [null, "https://evil.invalid", "//evil.invalid", "/dashboard/\\evil", "/api/auth/logout", "/tenders-evil", "/tenders/\\evil"]) assert.equal(safeReturnPath(value), "/dashboard");
  assert.equal(safeReturnPath("/dashboard/projects?tab=plan"), "/dashboard/projects?tab=plan");
  assert.equal(safeReturnPath("/tenders"), "/tenders");
  assert.equal(safeReturnPath("/tenders?q=solar"), "/tenders?q=solar");
});
