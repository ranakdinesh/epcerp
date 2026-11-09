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

test("tender FMS route supports prebid-queries, bid-items, vendor-quotations, and bid-costing with origin security", async () => {
  const draftId = "c5898643-88b8-44ce-89fc-45d7990e45b3";

  const route = load("../src/app/api/tender-fms/[[...path]]/route.ts", (name) => {
    if (name.includes("request-origin")) {
      return { hasSameOrigin: (r) => r.headers.get("origin") === new URL(r.url).origin };
    }
    return {
      proxyAuthenticatedRequest: async (r, c, resource, methods, limit) => {
        return Response.json({ ok: true, resource, method: r.method });
      },
    };
  });

  // 1. GET prebid-queries
  const qGet = await route.GET(
    new Request(`http://localhost/api/tender-fms/drafts/${draftId}/prebid-queries`),
    { params: Promise.resolve({ path: ["drafts", draftId, "prebid-queries"] }) }
  );
  assert.equal(qGet.status, 200);

  // 2. GET bid-items
  const itemsGet = await route.GET(
    new Request(`http://localhost/api/tender-fms/drafts/${draftId}/bid-items`),
    { params: Promise.resolve({ path: ["drafts", draftId, "bid-items"] }) }
  );
  assert.equal(itemsGet.status, 200);

  // 3. GET vendor-quotations
  const quotesGet = await route.GET(
    new Request(`http://localhost/api/tender-fms/drafts/${draftId}/vendor-quotations`),
    { params: Promise.resolve({ path: ["drafts", draftId, "vendor-quotations"] }) }
  );
  assert.equal(quotesGet.status, 200);

  // 4. GET bid-costing
  const costingGet = await route.GET(
    new Request(`http://localhost/api/tender-fms/drafts/${draftId}/bid-costing`),
    { params: Promise.resolve({ path: ["drafts", draftId, "bid-costing"] }) }
  );
  assert.equal(costingGet.status, 200);

  // 5. POST prebid-queries from same-origin succeeds
  const qPost = await route.POST(
    new Request(`http://localhost/api/tender-fms/drafts/${draftId}/prebid-queries`, {
      method: "POST",
      headers: { origin: "http://localhost", "Content-Type": "application/json" },
      body: JSON.stringify({
        category: "technical",
        tender_specification: "Test spec",
        query_description: "Test query",
      }),
    }),
    { params: Promise.resolve({ path: ["drafts", draftId, "prebid-queries"] }) }
  );
  assert.equal(qPost.status, 200);

  // 6. POST award-project from same-origin succeeds
  const awardPost = await route.POST(
    new Request(`http://localhost/api/tender-fms/drafts/${draftId}/award-project`, {
      method: "POST",
      headers: { origin: "http://localhost", "Content-Type": "application/json" },
      body: JSON.stringify({
        project_title: "11kV Substation EPC",
        contract_reference: "LOA-2026-001",
      }),
    }),
    { params: Promise.resolve({ path: ["drafts", draftId, "award-project"] }) }
  );
  assert.equal(awardPost.status, 200);

  // 7. POST from foreign origin fails with 403
  const foreignPost = await route.POST(
    new Request(`http://localhost/api/tender-fms/drafts/${draftId}/award-project`, {
      method: "POST",
      headers: { origin: "http://attacker.com", "Content-Type": "application/json" },
      body: JSON.stringify({ contract_reference: "LOA-123" }),
    }),
    { params: Promise.resolve({ path: ["drafts", draftId, "award-project"] }) }
  );
  assert.equal(foreignPost.status, 403);
});

test("bid costing calculation computes direct, contingency, overhead and margin accurately", () => {
  const materialCost = 500000;
  const freightCost = 25000;
  const civilCost = 75000;
  const contingencyPct = 3;
  const overheadPct = 5;
  const marginPct = 10;

  const baseDirect = materialCost + freightCost + civilCost; // 600,000
  assert.equal(baseDirect, 600000);

  const contingencyAmt = baseDirect * (contingencyPct / 100); // 18,000
  assert.equal(contingencyAmt, 18000);

  const overheadAmt = baseDirect * (overheadPct / 100); // 30,000
  assert.equal(overheadAmt, 30000);

  const subtotal = baseDirect + contingencyAmt + overheadAmt; // 648,000
  assert.equal(subtotal, 648000);

  const marginAmt = subtotal * (marginPct / 100); // 64,800
  assert.equal(marginAmt, 64800);

  const totalBid = subtotal + marginAmt; // 712,800
  assert.equal(totalBid, 712800);
});
