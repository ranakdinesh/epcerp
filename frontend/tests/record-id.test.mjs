import assert from "node:assert/strict";
import test from "node:test";
import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";
import { webcrypto } from "node:crypto";
import ts from "typescript";

const source = readFileSync(new URL("../src/lib/record-id.ts", import.meta.url), "utf8");
const { outputText } = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
});
function generator(crypto) {
  const exports = {};
  runInNewContext(outputText, { exports, crypto });
  return exports.createRecordId;
}

test("uses native UUID generation when available", () => {
  const expected = "acb5517d-125a-4bd1-a052-557598d8b167";
  assert.equal(generator({ randomUUID: () => expected })(), expected);
});
test("HTTP fallback creates RFC UUID v4 with correct version and variant", () => {
  const create = generator({ getRandomValues(bytes) {
    for (let index = 0; index < bytes.length; index++) bytes[index] = index;
    return bytes;
  } });
  assert.equal(create(), "00010203-0405-4607-8809-0a0b0c0d0e0f");
});
test("HTTP fallback uses fresh cryptographic randomness for each record", () => {
  const create = generator({ getRandomValues: webcrypto.getRandomValues.bind(webcrypto) });
  const ids = Array.from({ length: 100 }, create);
  assert.equal(new Set(ids).size, ids.length);
  for (const id of ids) assert.match(id, /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/);
});
test("does not silently substitute weak randomness", () => {
  assert.throws(generator(undefined), /Secure random number generation is unavailable/);
});
