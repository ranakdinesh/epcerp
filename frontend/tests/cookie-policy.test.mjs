import assert from "node:assert/strict";
import test from "node:test";
import { readFileSync } from "node:fs";
import ts from "typescript";

const source = readFileSync(new URL("../src/lib/auth/cookie-policy.ts", import.meta.url), "utf8");
const { outputText } = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 },
});
const { authCookieOptions } = await import(
  `data:text/javascript;base64,${Buffer.from(outputText).toString("base64")}`
);

test("production builds retain secure cookies by default, including development deployments", () => {
  for (const value of [undefined, "development", "staging", "production"]) {
    assert.deepEqual(authCookieOptions({ NODE_ENV: "production", EPCERP_ENVIRONMENT: value }), {
      httpOnly: true, sameSite: "lax", secure: true,
    });
  }
});
test("explicit development HTTP works with a production-compiled frontend", () => {
  assert.deepEqual(authCookieOptions({
    NODE_ENV: "production", EPCERP_ENVIRONMENT: "development", EPCERP_DEVELOPMENT_HTTP: "true",
  }), { httpOnly: true, sameSite: "lax", secure: false });
});
test("insecure HTTP cannot be enabled for staging, production or an unspecified environment", () => {
  for (const value of [undefined, "staging", "production"]) {
    assert.throws(() => authCookieOptions({
      NODE_ENV: "production", EPCERP_ENVIRONMENT: value, EPCERP_DEVELOPMENT_HTTP: "true",
    }), /only in the development environment/);
  }
});
test("local next dev retains its existing cookie behaviour", () => {
  assert.equal(authCookieOptions({ NODE_ENV: "development" }).secure, false);
});
