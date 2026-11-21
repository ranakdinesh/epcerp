import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import ts from "typescript";

function load(relative, requireFn = () => ({})) {
  const source = readFileSync(new URL(relative, import.meta.url), "utf8");
  const { outputText } = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  });
  const mod = { exports: {} };
  new Function("require", "exports", outputText)(requireFn, mod.exports);
  return mod.exports;
}

test("tender FMS route supports flow-status and bidding-approval with same-origin enforcement", async () => {
  const draftId = "c5898643-88b8-44ce-89fc-45d7990e45b3";

  const route = load("../src/app/api/tender-fms/[[...path]]/route.ts", (name) => {
    if (name.includes("request-origin")) {
      return { hasSameOrigin: (r) => r.headers.get("origin") === new URL(r.url).origin };
    }
    return {
      proxyAuthenticatedRequest: async (r, c, resource, methods, limit) => {
        return Response.json({ ok: true });
      },
    };
  });

  // GET flow-status without origin
  const flowGet = await route.GET(
    new Request(`http://localhost/api/tender-fms/drafts/${draftId}/flow-status`),
    { params: Promise.resolve({ path: ["drafts", draftId, "flow-status"] }) }
  );
  assert.equal(flowGet.status, 200);

  // GET bidding-approval without origin
  const bidGet = await route.GET(
    new Request(`http://localhost/api/tender-fms/drafts/${draftId}/bidding-approval`),
    { params: Promise.resolve({ path: ["drafts", draftId, "bidding-approval"] }) }
  );
  assert.equal(bidGet.status, 200);

  // POST bidding-approval from foreign origin must fail 403
  const foreignPost = await route.POST(
    new Request(`http://localhost/api/tender-fms/drafts/${draftId}/bidding-approval`, {
      method: "POST",
      headers: { origin: "http://malicious.origin.com", "Content-Type": "application/json" },
      body: JSON.stringify({ decision: "authorized" }),
    }),
    { params: Promise.resolve({ path: ["drafts", draftId, "bidding-approval"] }) }
  );
  assert.equal(foreignPost.status, 403);

  // POST bidding-approval with same origin succeeds
  const validPost = await route.POST(
    new Request(`http://localhost/api/tender-fms/drafts/${draftId}/bidding-approval`, {
      method: "POST",
      headers: { origin: "http://localhost", "Content-Type": "application/json" },
      body: JSON.stringify({ decision: "authorized" }),
    }),
    { params: Promise.resolve({ path: ["drafts", draftId, "bidding-approval"] }) }
  );
  assert.equal(validPost.status, 200);
});
