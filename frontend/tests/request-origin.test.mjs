import assert from "node:assert/strict";
import test from "node:test";
import { readFileSync } from "node:fs";
import ts from "typescript";

const output = ts.transpileModule(
  readFileSync(new URL("../src/lib/auth/request-origin.ts", import.meta.url), "utf8"),
  { compilerOptions: { module: ts.ModuleKind.ESNext } },
).outputText;
const { hasSameOrigin } = await import(`data:text/javascript;base64,${Buffer.from(output).toString("base64")}`);
const request = (origin, host = "localhost:3001", extra = {}) => new Request(
  "http://0.0.0.0:3001/api/tender-fms/drafts",
  { headers: { host, ...(origin === undefined ? {} : { origin }), ...extra } },
);

test("accepts browser authority despite Next bind-host reconstruction", () => {
  assert.equal(hasSameOrigin(request("http://localhost:3001")), true);
  assert.equal(hasSameOrigin(request("http://127.0.0.1:3001", "127.0.0.1:3001")), true);
});
test("rejects foreign, wrong-port/protocol, absent, null and malformed origins", () => {
  for (const origin of ["https://evil.example", "http://localhost:3000", "https://localhost:3001", undefined, "null", "invalid", "http://localhost:3001/path"]) {
    assert.equal(hasSameOrigin(request(origin)), false, String(origin));
  }
  assert.equal(hasSameOrigin(request("http://127.0.0.1:3001")), false);
});
test("forwarded headers cannot authorize a foreign origin", () => {
  assert.equal(hasSameOrigin(request("https://evil.example", "localhost:3001", {
    "x-forwarded-host": "evil.example", "x-forwarded-proto": "https",
  })), false);
});
