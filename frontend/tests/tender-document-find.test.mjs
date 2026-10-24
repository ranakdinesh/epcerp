import assert from "node:assert/strict";
import test from "node:test";
import { readFileSync } from "node:fs";
import ts from "typescript";
const output = ts.transpileModule(
  readFileSync(
    new URL(
      "../src/components/tenders/document-find-model.ts",
      import.meta.url,
    ),
    "utf8",
  ),
  {
    compilerOptions: {
      module: ts.ModuleKind.ESNext,
      target: ts.ScriptTarget.ES2022,
    },
  },
).outputText;
const { documentPages, findPages } = await import(
  `data:text/javascript;base64,${Buffer.from(output).toString("base64")}`
);
const source =
  '# Source\n\n## Page 1\nEarnest\n money deposit INR 100.\n### Page 1 — table 1\n| EMD | 100 |\n## Page 2\nSubmission date 1 October.\n## Canonical page manifest\n{"text":"EMD duplicate"}';
test("generated pages retain tables but exclude manifest and preamble", () => {
  const pages = documentPages(source);
  assert.equal(pages.length, 2);
  assert.deepEqual(
    pages.map((p) => p.number),
    [1, 2],
  );
  assert.match(pages[0].text, /table 1/);
  assert.doesNotMatch(pages[1].text, /duplicate/);
});
test("phrases match across line breaks and repeated topic hits count one page", () => {
  const pages = documentPages(source);
  assert.equal(findPages(pages, "EARNEST MONEY|emd").length, 1);
  assert.equal(findPages(pages, "submission|closing date")[0].number, 2);
  assert.equal(findPages(pages, "missing").length, 0);
  assert.equal(findPages(pages, " | ").length, 0);
});
test("legacy content never invents a page number and literal metacharacters work", () => {
  const pages = documentPages("Offer [A+B]");
  assert.equal(pages[0].number, null);
  assert.equal(findPages(pages, "[A+B]").length, 1);
});

test("full topic phrases rank above incidental acronym references", () => {
  const matches = findPages(
    [
      { number: 1, text: "ET-03/EMD-II/2026" },
      { number: 2, text: "Earnest money deposit is required." },
    ],
    "emd|earnest money",
  );
  assert.equal(matches[0].number, 2);
});
