import assert from "node:assert/strict";
import test from "node:test";
import { readFileSync } from "node:fs";
import { Readable } from "node:stream";
import { createHash } from "node:crypto";
import ts from "typescript";
import { getCloneableBody } from "next/dist/server/body-streams.js";

const source = readFileSync(new URL("../next.config.ts", import.meta.url), "utf8");
const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext } }).outputText;
const { default: config } = await import(`data:text/javascript;base64,${Buffer.from(compiled).toString("base64")}`);

test("Next proxy preserves complete tender uploads above 10 MiB and at the 64 MiB ZIP limit", async () => {
  for (const size of [18 * 1024 * 1024 + 123, 64 * 1024 * 1024]) {
    const chunk = Buffer.alloc(64 * 1024, 0xa5), expected = createHash("sha256");
    const request = Readable.from((function* () {
      for (let at = 0; at < size; at += chunk.length) {
        const part = chunk.subarray(0, Math.min(chunk.length, size - at));
        expected.update(part);
        yield part;
      }
    })());
    const body = getCloneableBody(request, config.experimental?.proxyClientMaxBodySize);
    const actual = createHash("sha256");
    let received = 0;
    for await (const bytes of body.cloneBodyStream()) {
      received += bytes.length;
      actual.update(bytes);
    }
    await body.finalize();
    assert.equal(received, size);
    assert.equal(actual.digest("hex"), expected.digest("hex"));
  }
});
