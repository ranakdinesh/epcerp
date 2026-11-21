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

test("company profile route enforces same-origin protection on mutations", async () => {
  const route = load("../src/app/api/settings/company-profile/[[...path]]/route.ts", (name) => {
    if (name.includes("request-origin")) {
      return { hasSameOrigin: (r) => r.headers.get("origin") === new URL(r.url).origin };
    }
    return {
      proxyAuthenticatedRequest: async (r, c, resource, methods, limit) => {
        return Response.json({ ok: true });
      },
    };
  });

  // GET is permitted without origin header
  const getRes = await route.GET(new Request("http://localhost/api/settings/company-profile"), {
    params: Promise.resolve({ path: [] }),
  });
  assert.equal(getRes.status, 200);

  // PATCH with foreign origin must fail with 403
  const foreignPatch = await route.PATCH(
    new Request("http://localhost/api/settings/company-profile", {
      method: "PATCH",
      headers: { origin: "http://malicious.example.com" },
    }),
    { params: Promise.resolve({ path: [] }) }
  );
  assert.equal(foreignPatch.status, 403);

  // Same origin PATCH must succeed
  const validPatch = await route.PATCH(
    new Request("http://localhost/api/settings/company-profile", {
      method: "PATCH",
      headers: { origin: "http://localhost" },
    }),
    { params: Promise.resolve({ path: [] }) }
  );
  assert.equal(validPatch.status, 200);

  // Sub-resource POST (e.g. financials, licences) with same origin must succeed
  const validPost = await route.POST(
    new Request("http://localhost/api/settings/company-profile/financials", {
      method: "POST",
      headers: { origin: "http://localhost" },
    }),
    { params: Promise.resolve({ path: ["financials"] }) }
  );
  assert.equal(validPost.status, 200);

  // DELETE with foreign origin must fail with 403
  const foreignDelete = await route.DELETE(
    new Request("http://localhost/api/settings/company-profile/financials/123", {
      method: "DELETE",
      headers: { origin: "http://evil.com" },
    }),
    { params: Promise.resolve({ path: ["financials", "123"] }) }
  );
  assert.equal(foreignDelete.status, 403);
});

test("tender FMS route correctly allows eligibility and verification sub-paths", async () => {
  const route = load("../src/app/api/tender-fms/[[...path]]/route.ts", (name) => {
    if (name.includes("request-origin")) {
      return { hasSameOrigin: (r) => r.headers.get("origin") === new URL(r.url).origin };
    }
    return {
      proxyAuthenticatedRequest: async (r, c, resource, methods, limit) => {
        return Response.json({ ok: true, resource });
      },
    };
  });

  const draftId = "10000000-0000-4000-8000-000000000001";
  const jobId = "20000000-0000-4000-8000-000000000002";

  // GET eligibility
  const getElig = await route.GET(
    new Request(`http://localhost/api/tender-fms/drafts/${draftId}/tsc/${jobId}/eligibility`),
    { params: Promise.resolve({ path: ["drafts", draftId, "tsc", jobId, "eligibility"] }) }
  );
  assert.equal(getElig.status, 200);

  // POST eligibility evaluation (same origin)
  const postElig = await route.POST(
    new Request(`http://localhost/api/tender-fms/drafts/${draftId}/tsc/${jobId}/eligibility`, {
      method: "POST",
      headers: { origin: "http://localhost" },
    }),
    { params: Promise.resolve({ path: ["drafts", draftId, "tsc", jobId, "eligibility"] }) }
  );
  assert.equal(postElig.status, 200);

  // POST eligibility verification (same origin)
  const postVerify = await route.POST(
    new Request(`http://localhost/api/tender-fms/drafts/${draftId}/tsc/${jobId}/eligibility/verify`, {
      method: "POST",
      headers: { origin: "http://localhost" },
    }),
    { params: Promise.resolve({ path: ["drafts", draftId, "tsc", jobId, "eligibility", "verify"] }) }
  );
  assert.equal(postVerify.status, 200);
});
