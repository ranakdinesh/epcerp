import assert from "node:assert/strict";
import test from "node:test";
import { readFileSync } from "node:fs";
import { createHash, webcrypto } from "node:crypto";
import ts from "typescript";
const source = readFileSync(new URL("../src/lib/file-sha256.ts", import.meta.url), "utf8");
const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ES2022, target: ts.ScriptTarget.ES2022 } }).outputText;
// Resolve the lazy dependency exactly as the frontend would.
const dependency = new URL("../node_modules/@noble/hashes/sha2.js", import.meta.url).href;
const { fileSHA256 } = await import(`data:text/javascript;base64,${Buffer.from(compiled.replace('"@noble/hashes/sha2.js"', JSON.stringify(dependency))).toString("base64")}`);
test("HTTP fallback and native WebCrypto match server SHA-256 for empty, binary and multi-chunk files", async () => {
  const original = Object.getOwnPropertyDescriptor(globalThis, "crypto");
  try {
    for (const bytes of [new Uint8Array(), new TextEncoder().encode("abc"), Uint8Array.from({ length: 2 * 1024 * 1024 + 123 }, (_, i) => i % 251)]) {
      const expected = createHash("sha256").update(bytes).digest("hex");
      for (const crypto of [undefined, { getRandomValues: webcrypto.getRandomValues.bind(webcrypto) }, webcrypto]) {
        Object.defineProperty(globalThis, "crypto", { configurable: true, value: crypto });
        assert.equal(await fileSHA256(new Blob([bytes])), expected);
      }
    }
  } finally { if (original) Object.defineProperty(globalThis, "crypto", original); else delete globalThis.crypto; }
});
