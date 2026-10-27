// Browser plugin not available: use regular Playwright with synthetic responses.
// Runs only the existing disposable preview route; never writes operational data.
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const playwright = process.env.EPCERP_PLAYWRIGHT_MODULE
  ? await import(pathToFileURL(path.join(process.env.EPCERP_PLAYWRIGHT_MODULE, "index.mjs")).href)
  : await import("playwright");
const { chromium } = playwright;

(async () => {
  const base = process.env.EPCERP_TSC_UI_URL || "http://127.0.0.1:9093";
  assert.ok(["localhost", "127.0.0.1"].includes(new URL(base).hostname), "Local preview only");
  const root = path.resolve(__dirname, "../..");
  const registry = JSON.parse(fs.readFileSync(path.join(root, "docs/GO_TSC_QUESTION_SOURCE_MAP.json"), "utf8").replace(/^\uFEFF/, ""));
  const browser = await chromium.launch({ executablePath: process.env.EPCERP_BROWSER_EXECUTABLE || "C:/Program Files/Google/Chrome/Application/chrome.exe", headless: true });
  const page = await browser.newPage({ viewport: { width: 1440, height: 1000 }, acceptDownloads: true });
  const errors = [];
  page.on("pageerror", e => errors.push(e.message));
  const output = process.env.EPCERP_TSC_PROOF_DIR || path.join(root, "tmp/tsc-proof");
  fs.mkdirSync(output, { recursive: true });
  try {
    await page.goto(`${base}/tf102-preview`, { waitUntil: "networkidle" });
    await page.getByRole("button", { name: "Add tender draft", exact: true }).waitFor();
    await page.evaluate(questions => {
      const original = window.fetch;
      const id = "abcd0000-0000-4000-8000-000000000001", jobID = "abcd0000-0000-4000-8000-000000000002", sourceID = "abcd0000-0000-4000-8000-000000000003";
      const draft = { id, name: "TSC browser fixture", version: 1, created_at: "2026-10-25T00:00:00Z", updated_at: "2026-10-25T00:00:00Z", batches: [{ id: "abcd0000-0000-4000-8000-000000000004", revision: "fixture-revision", state: "consumed", attempts: 1, error: "", files: [], manifest: [], sources: [{ id: sourceID, member_id: "synthetic-member", path: "synthetic-tender.pdf", filename: "synthetic-tender.pdf", sha256: "synthetic-hash", status: "ready", error: "", version: 1, attempts: 1, segments: 1, expected_pages: 1, expected_sheets: 0, warnings: [], updated_at: "2026-10-25T00:00:00Z" }] }] };
      const passage = { id: "source:1:1", document_id: sourceID, filename: "synthetic-tender.pdf", locator: "page 1 / part 1", text: "Synthetic Department of Electrical Works. EMD INR 500000.00." };
      let job = null, polls = 0, writes = 0;
      const response = (body, status = 200) => new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
      window.__tscFixture = { stale() { draft.version++; }, writes() { return writes; } };
      window.fetch = async (input, init) => {
        const url = new URL(typeof input === "string" ? input : input.url, location.href), p = url.pathname.replace("/api/tender-fms/", ""), method = init?.method || "GET";
        if (!p.startsWith("drafts")) return original(input, init);
        if (p.endsWith("/tsc/options")) return response({ providers: [{ mode: "manual", available: true, cloud: false }, { mode: "ollama", model: "synthetic-local-model", available: true, cloud: false, allowance_paise: 0 }, { mode: "gemini", model: "gemini-3.1-flash-lite", available: true, cloud: true, allowance_paise: 150 }, { mode: "openai", available: false, cloud: true, allowance_paise: 60 }], limit_paise: 10000, reserved_paise: 0 });
        if (p.endsWith("/research")) { const body = JSON.parse(init.body); return response({ question_id: "particulars.q01", passages: [passage], warning: "Synthetic source snapshot only.", ...(body.mode !== "search" ? { proposal: { question_id: "particulars.q01", status: "answered", answer: "Synthetic Department of Electrical Works", citations: [{ source_id: passage.id, quote: passage.text }], origin: "synthetic" } } : {}) }); }
        if (p === "drafts") return response({ items: [draft], total: 1 });
        if (p === `drafts/${id}`) return response(draft);
        if (p === `drafts/${id}/batches` && method === "POST") return new Response(null, { status: 204 });
        if (p === `drafts/${id}/tsc` && method === "POST") {
          const body = JSON.parse(init.body); if (body.expected_version !== draft.version || !new Headers(init.headers).get("Idempotency-Key")) return response({ detail: "Invalid generation context" }, 409);
          job = { id: jobID, draft_id: id, draft_version: draft.version, state: "queued", error: "", version: 1, mode: body.mode, model: body.mode === "ollama" ? "synthetic-local-model" : "", attempts: 0, created_at: "2026-10-25T00:00:00Z", stale: false, input: { questions, passages: [passage, { ...passage, id: "source:2:1", document_id: "abcd0000-0000-4000-8000-000000000009", filename: "synthetic-conditions.pdf", locator: "page 2", text: "Synthetic second document payment conditions. <script>window.evidenceExecuted = true</script>" }], coverage: { version: "document-evidence-v1", documents: 1, segments: 1, empty_segments: 0, blocks: 1, suppressed: 0, low_confidence: 0 } }, result: { answers: [], warnings: ["Synthetic browser evidence only; no operational approval."], diagnostics: questions.map(q => ({ question_id: q.id, candidates: 1, documents: 1, outcome: "manual_draft:needs_review" })) } };
          return response(job, 202);
        }
        if (p === `drafts/${id}/tsc`) return response({ items: job ? [job] : [] });
        if (method === "PATCH") {
          const body = JSON.parse(init.body); writes++;
          if (body.expected_version !== job.version || body.citations[0]?.quote !== passage.text || "approved" in body || "created_by_id" in body || "reason" in body) return response({ detail: "Invalid correction fixture" }, 422);
          const question = p.split("/").at(-1); job.result.answers = job.result.answers.map(a => a.question_id === question ? { ...a, status: body.status, answer: body.answer, citations: body.citations, accepted: body.accepted, origin: "manual" } : a); job.version++; return response(job);
        }
        if (p === `drafts/${id}/tsc/${jobID}` && job) {
          if (polls++ > 0) { job.state = "ready"; job.attempts = 1; if (!job.result.answers.length) job.result.answers = questions.map(q => q.id === "particulars.q05" ? { question_id:q.id, status:"answered", answer:"The tender value without GST is INR 3,74,62,212.00. This is stated in the document (S1). The bid value is INR 37462212.", citations:[{source_id:passage.id,quote:passage.text}], origin:"ollama_proposal" } : q.id === "particulars.q06" ? { question_id:q.id, status:"answered", answer:"Reverse Auction (RA) is applicable to the first 50% of technically qualified bidders, arranged in order of prices from lowest to highest. If the number is odd, round up. For example, 7 bids means 4 bidders.", citations:[{source_id:passage.id,quote:passage.text}], origin:"ollama_proposal" } : q.id === "particulars.q09" ? {question_id:q.id,status:"answered",answer:"answered",citations:[{source_id:passage.id,quote:passage.text}],origin:"ollama_proposal"} : ({ question_id: q.id, status: "needs_review", answer: "The local model proposal failed source-evidence validation. Review the original documents and complete this answer with citations.", citations: [], origin: "rejected_ollama_proposal" })); } else job.state = "processing";
          job.stale = job.draft_version !== draft.version; return response(job);
        }
        return response({ detail: "Unsupported synthetic route" }, 404);
      };
    }, registry.questions);
    await page.getByRole("button", { name: "Retry", exact: true }).first().click();
    await page.getByRole("button", { name: "Open TSC browser fixture", exact: true }).click();
    const panel = page.getByRole("dialog", { name: "TSC browser fixture", exact: true });
    assert.equal(await panel.getByRole("button", { name: "Generate TSC draft", exact: true }).count(), 0);
    const selection = panel.getByLabel("Tender document selection", { exact: true });
    const addFile = async (name, text) => {
      const chooser = page.waitForEvent("filechooser");
      await selection.getByRole("button", { name: "Add documents", exact: true }).click();
      await (await chooser).setFiles({ name, mimeType: "application/octet-stream", buffer: Buffer.from(text) });
    };
    await addFile("notice.pdf", "%PDF-synthetic notice only");
    await addFile("conditions.docx", "synthetic document fixture only");
    assert.equal(await selection.getByRole("list", { name: "Selected documents" }).getByRole("listitem").count(), 2);
    await selection.getByRole("button", { name: "Choose documents or drop files", exact: true }).evaluate(element => {
      const transfer = new DataTransfer(); transfer.items.add(new File(["Synthetic BOQ fixture"], "schedule.xlsx"));
      element.dispatchEvent(new DragEvent("drop", { bubbles: true, cancelable: true, dataTransfer: transfer }));
    });
    assert.equal(await selection.getByRole("listitem").count(), 3);
    await selection.getByRole("button", { name: "Remove conditions.docx", exact: true }).click();
    await addFile("notice.pdf", "%PDF-synthetic duplicate");
    await selection.getByText("Rename duplicate filenames before uploading.", { exact: true }).waitFor();
    assert.equal(await selection.getByRole("listitem").count(), 2);
    await addFile("notes.exe", "unsupported synthetic file");
    await selection.getByText(/Use PDF, DOCX, XLSX or ZIP/).waitFor();
    assert.equal(await selection.getByRole("listitem").count(), 2);
    await addFile("extra.pdf", "%PDF-synthetic additional source");
    await page.screenshot({ path: path.join(output, "tender-upload-desktop.png") });
    await page.setViewportSize({ width: 390, height: 844 });
    await page.evaluate(() => { document.documentElement.classList.add("dark"); document.documentElement.dataset.theme = "dark"; });
    assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), "Upload page overflow on mobile");
    await page.screenshot({ path: path.join(output, "tender-upload-mobile.png") });
    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.evaluate(() => { document.documentElement.classList.remove("dark"); document.documentElement.dataset.theme = "light"; });
    assert.equal(await panel.getByRole("button", { name: "Upload and read", exact: true }).count(), 0);
    await panel.getByRole("button", { name: "Upload", exact: true }).click();
    await selection.getByText("No documents selected. Use Add documents to start.", { exact: true }).waitFor();
    await addFile("next-batch.pdf", "%PDF-synthetic next batch");
    await selection.getByRole("button", { name: "Remove next-batch.pdf", exact: true }).click();

    await panel.getByRole("button", { name: "2. Generate TSC", exact: true }).click();
    await panel.getByLabel("Draft preparation", { exact: true }).selectOption("ollama");
    await panel.getByRole("button", { name: "Generate TSC draft", exact: true }).click();
    await panel.getByText(/Generation: (queued|processing)/).waitFor();
    await panel.getByRole("button", { name: "Close panel", exact: true }).click();
    await page.getByRole("button", { name: "Open TSC browser fixture", exact: true }).click();
    await panel.getByRole("button", { name: "2. Generate TSC", exact: true }).click();
    await panel.getByText(/Generation: ready/).waitFor({ timeout: 15000 });
    const editor = panel.getByRole("region", { name: "TSC split review", exact: true });
    const checklist = editor.getByRole("region", { name: "Editable TSC", exact: true });
    const reader = editor.getByRole("complementary", { name: "Consolidated Markdown reader" });
    assert.equal(await checklist.locator("form").count(), 70, "Entire checklist is immediately editable");
    assert.equal(await reader.getByLabel("Consolidated Markdown document").count(), 1, "One continuous Markdown document");
    assert.equal(await editor.getByRole("button", { name: /Edit TSC question/ }).count(), 0);
    assert.ok((await reader.getByLabel("Consolidated Markdown document").innerText()).includes("Synthetic second document payment conditions."));
    assert.equal(await page.evaluate(() => window.evidenceExecuted), undefined, "Source markup is inert");
    await reader.getByLabel("Search consolidated Markdown").fill("no such synthetic phrase");
    await reader.getByText("No source text matches these filters.").waitFor();
    await reader.getByLabel("Search consolidated Markdown").fill("");
    await reader.getByRole("combobox", { name: "Filter source document", exact: true }).selectOption({ index: 1 });
    await checklist.getByLabel("Filter TSC questions").fill("Tender Due");
    assert.equal(await checklist.getByRole("textbox", {name:/^Answer/}).inputValue(), "", "Historical answered status is not answer content");
    await checklist.getByRole("checkbox", {name:"Accepted",exact:true}).click();
    await checklist.getByRole("alert").getByText("Enter an answer before accepting.", {exact:true}).waitFor();
    assert.equal(await page.evaluate(() => window.__tscFixture.writes()), 0);
    await checklist.getByLabel("Filter TSC questions").fill("Tender value without GST");
    assert.equal(await checklist.getByRole("textbox", {name:/^Answer/}).inputValue(), "INR 3,74,62,212.00");
    await checklist.getByRole("textbox", {name:/^Answer/}).scrollIntoViewIfNeeded();
    await page.screenshot({path:path.join(output,"tsc-one-line-value.png")});
    await checklist.getByLabel("Filter TSC questions").fill("Reverse Auction");
    assert.equal(await checklist.getByRole("textbox", {name:/^Answer/}).inputValue(), "Yes; the first 50% of technically qualified bidders, ranked by price (lowest first)");
    await checklist.getByRole("textbox", {name:/^Answer/}).scrollIntoViewIfNeeded();
    await page.screenshot({path:path.join(output,"tsc-one-line-auction.png")});
    await checklist.getByLabel("Filter TSC questions").fill("Name of Department");
    const question = checklist.getByRole("form", { name: "TSC question particulars.q01", exact: true });
    assert.equal(await checklist.getByRole("combobox", { name: "Answer state" }).count(), 0);
    await question.getByRole("button", { name: "Search sources for this question", exact: true }).click();
    await question.getByRole("button", { name: "Search sources · no cloud cost", exact: true }).click();
    await question.getByText("1 relevant excerpts", { exact: true }).waitFor();
    await question.getByLabel("Proposal provider", { exact: true }).selectOption("gemini");
    await question.getByRole("button", { name: "Request revised proposal", exact: true }).click();
    await question.getByRole("button", { name: "Use proposal in answer", exact: true }).waitFor();
    await page.screenshot({ path: path.join(output, "tsc-research-desktop.png") });
    await question.getByRole("button", { name: "Use proposal in answer", exact: true }).scrollIntoViewIfNeeded();
    await page.screenshot({ path: path.join(output, "tsc-proposal-desktop.png") });
    await question.getByRole("button", { name: "Use proposal in answer", exact: true }).click();
    assert.equal(await question.getByRole("textbox", { name: /^Answer/ }).inputValue(), "Synthetic Department of Electrical Works");
    assert.equal(await page.evaluate(() => window.__tscFixture.writes()), 0);
    await question.getByText("Supporting evidence (1)", { exact: true }).click();
    await question.getByRole("button", { name: "Remove citation", exact: true }).click();
    await question.getByRole("textbox", { name: /^Answer/ }).fill("Synthetic Department of Electrical Works");
    assert.equal(await checklist.getByLabel("Correction reason").count(), 0);
    assert.equal(await checklist.getByRole("button", { name: "Save answer", exact: true }).count(), 0);
    await question.getByRole("checkbox", { name: "Accepted", exact: true }).click();
    await question.getByRole("alert").waitFor();
    assert.equal(await page.evaluate(() => window.__tscFixture.writes()), 0);
    await reader.getByLabel("Consolidated Markdown document").evaluate(el => {
      const text = "Synthetic Department of Electrical Works. EMD INR 500000.00.";
      const node = el.firstChild;
      const start = node.textContent.indexOf(text);
      const range = document.createRange(); range.setStart(node, start); range.setEnd(node, start + text.length);
      const selection = window.getSelection(); selection.removeAllRanges(); selection.addRange(range);
    });
    await reader.getByRole("button", { name: "Attach selected text to active question", exact: true }).click();
    await reader.getByText("Selected source text attached to the active question.").waitFor();
    await page.screenshot({ path: path.join(output, "tsc-split-desktop.png") });
    const leftBox = await checklist.boundingBox(), rightBox = await reader.boundingBox();
    assert.ok(leftBox.x < rightBox.x && Math.abs(leftBox.y - rightBox.y) < 2, "Persistent equal-height desktop columns");
    await page.setViewportSize({ width: 390, height: 844 });
    assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), "Split reader overflow on mobile");
    await page.screenshot({ path: path.join(output, "tsc-split-mobile.png") });
    await page.setViewportSize({ width: 1440, height: 1000 });
    await question.getByRole("checkbox", { name: "Accepted", exact: true }).click();
    await reader.getByText("Answer accepted.", { exact: true }).waitFor();
    assert.ok(await question.getByRole("textbox", { name: /^Answer/ }).isDisabled());
    assert.ok(await question.getByRole("button", { name: "Search sources for this question", exact: true }).isDisabled());
    await question.getByRole("checkbox", { name: "Accepted", exact: true }).click();
    await reader.getByText("Answer reopened.", { exact: true }).waitFor();
    assert.ok(await question.getByRole("textbox", { name: /^Answer/ }).isEnabled());
    await question.getByRole("checkbox", { name: "Accepted", exact: true }).click();
    await reader.getByText("Answer accepted.", { exact: true }).waitFor();
    assert.equal(await page.evaluate(() => window.__tscFixture.writes()), 3);
    assert.equal(await question.getByRole("textbox", { name: /^Answer/ }).inputValue(), "Synthetic Department of Electrical Works");
    await checklist.getByLabel("Filter TSC questions").fill("Location where");
    assert.equal(await checklist.getByRole("textbox", { name: /^Answer/ }).first().inputValue(), "", "Historical failure descriptions leave answer blank");
    await checklist.getByLabel("Filter TSC questions").fill("");
    await checklist.getByLabel("Answer filter", { exact: true }).selectOption("accepted");
    assert.equal(await checklist.locator("form").count(), 1);
    await checklist.getByLabel("Answer filter", { exact: true }).selectOption("answered");
    assert.equal(await checklist.locator("form").count(), 4);
    await checklist.getByLabel("Answer filter", { exact: true }).selectOption("");
    assert.equal(await checklist.locator("form").count(), 70);
    const assertContrast = async () => {
      const values = await editor.locator('h3, h4, label, textarea, pre, input[type="text"]').evaluateAll(nodes => nodes.map(el => {
        const rgb = color => (color.match(/[0-9.]+/g) || []).slice(0, 3).map(Number);
        const lum = c => rgb(c).map(v => { v /= 255; return v <= .04045 ? v / 12.92 : ((v + .055) / 1.055) ** 2.4; }).reduce((n,v,i) => n + v * [.2126,.7152,.0722][i],0);
        let parent = el, bg;
        while (parent) { bg = getComputedStyle(parent).backgroundColor; if (bg !== "rgba(0, 0, 0, 0)" && bg !== "transparent") break; parent = parent.parentElement; }
        const a = lum(getComputedStyle(el).color), b = lum(bg || "rgb(255,255,255)");
        return { tag: el.tagName, ratio: (Math.max(a,b)+.05)/(Math.min(a,b)+.05) };
      }));
      assert.ok(values.every(v => v.ratio >= 4.5), JSON.stringify(values.filter(v => v.ratio < 4.5)));
    };
    await page.emulateMedia({ colorScheme: "dark" });
    await page.evaluate(() => { document.documentElement.classList.remove("dark"); document.documentElement.dataset.theme = "light"; });
    await assertContrast();
    await page.screenshot({ path: path.join(output, "tsc-accepted-light.png") });
    await page.emulateMedia({ colorScheme: "light" });
    await page.evaluate(() => { document.documentElement.dataset.theme = "dark"; });
    await assertContrast();
    await page.screenshot({ path: path.join(output, "tsc-accepted-dark.png") });
    await page.evaluate(() => { document.documentElement.dataset.theme = "light"; });
    await assertContrast();
    await panel.getByText("Source evidence and processing details", { exact: true }).click();
    await panel.getByLabel("Evidence processing coverage").getByText("Source preparation", { exact: true }).waitFor();
    assert.equal(await panel.getByRole("columnheader", { name: "Evidence retrieved", exact: true }).count(), 0);
    await page.route("**/api/tender-fms/drafts/*/tsc/*/evidence", route => route.fulfill({ status: 200, contentType: "text/markdown", headers: { "Content-Disposition": 'attachment; filename="TSC-source-evidence.md"' }, body: "# Synthetic source evidence\n\nSynthetic fixture only." }));
    const evidenceDownload = page.waitForEvent("download");
    await panel.getByRole("link", { name: "Download source evidence MD", exact: true }).click();
    assert.equal((await evidenceDownload).suggestedFilename(), "TSC-source-evidence.md");
    await panel.getByText("Source evidence and processing details", { exact: true }).click();
    await page.route("**/api/tender-fms/drafts/*/tsc/*/document", route => route.fulfill({ status: 200, contentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document", headers: { "Content-Disposition": 'attachment; filename="TSC-draft.docx"' }, body: fs.readFileSync(path.join(output, "synthetic-tsc-draft.docx")) }));
    const downloadPromise = page.waitForEvent("download");
    await panel.getByRole("link", { name: "Download draft DOCX", exact: true }).click();
    assert.equal((await downloadPromise).suggestedFilename(), "TSC-draft.docx");
    await page.screenshot({ path: path.join(output, "tsc-desktop.png") });
    await page.setViewportSize({ width: 390, height: 844 });
    await page.evaluate(() => { document.documentElement.classList.add("dark"); document.documentElement.dataset.theme = "dark"; });
    assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), "Page overflow on mobile");
    await page.screenshot({ path: path.join(output, "tsc-mobile-dark.png") });
    await page.evaluate(() => window.__tscFixture.stale());
    await panel.getByText(/Sources changed\. This snapshot is retained/).waitFor({ timeout: 15000 });
    assert.equal(await panel.getByRole("link", { name: "Download draft DOCX", exact: true }).count(), 0);
    assert.equal(await panel.getByRole("link", { name: "Download source evidence MD", exact: true }).count(), 0);
    assert.equal(errors.length, 0, errors.join("\n"));
    console.log("PASS: visible upload area, additive one-by-one selection, drag/drop, remove, duplicate/unsupported rejection, Upload and next-batch selection; full editable 70-question TSC, provider availability/allowance, free question search, revised proposal adoption without automatic save, persistent split, one consolidated Markdown document with both sources, source and answer filters, inert source HTML, text-selection citation, accept/reopen with automatic audit, blank failed answers, concise amount/applicability answers, application-controlled light/dark contrast, exports, stale-source guards, desktop/mobile. Synthetic UI only.");
  } catch (error) { await page.screenshot({ path: path.join(output, "tsc-ui-failure.png") }); fs.writeFileSync(path.join(output, "tsc-ui-failure.txt"), await page.locator("body").innerText()); throw error; } finally { await browser.close(); }
})().catch(error => { console.error(error.message); process.exitCode = 1; });
