"use client";

import { useDeferredValue, useEffect, useMemo, useState } from "react";
import {
  ValidatedInput,
  ValidatedLabel,
} from "@/components/ui/validated-controls";
import { documentPages, findPages, searchTerms } from "./document-find-model";

import { TenderPdfPreview } from "./tender-pdf-preview";

const topics = [
  ["Tender identity", "tender no|tender number|name of work|invitation"],
  ["EMD / bid security", "emd|earnest money|bid security"],
  ["Submission deadline", "last date|closing date|submission|bid opening"],
  ["Eligibility", "eligibility|qualification|experience|turnover"],
  ["Scope of work", "scope of work|description of work|specification"],
  ["BOQ / quantities", "boq|bill of quantities|schedule of quantities"],
] as const;

function Highlight({ text, query }: { text: string; query: string }) {
  const terms = searchTerms(query);
  if (!terms.length) return text;
  const escaped = terms
    .sort((a, b) => b.length - a.length)
    .map((term) => term.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"));
  const parts = text.split(new RegExp(`(${escaped.join("|")})`, "gi"));
  return parts.map((part, index) =>
    terms.includes(part.toLowerCase()) ? (
      <mark key={index} className="rounded bg-amber-200 text-slate-950">
        {part}
      </mark>
    ) : (
      part
    ),
  );
}

export function TenderDocumentFinder({ id }: { id: string }) {
  const [open, setOpen] = useState(false);
  const [markdown, setMarkdown] = useState<string>();
  const [error, setError] = useState("");
  const [retry, setRetry] = useState(0);
  const [query, setQuery] = useState("");
  const [limit, setLimit] = useState(12);
  const [previewPage, setPreviewPage] = useState<number | null>(null);
  const deferredQuery = useDeferredValue(query);
  useEffect(() => {
    if (!open) return;
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 30000);
    let active = true;
    void fetch(`/api/tenders/${id}/source/md`, {
      signal: controller.signal,
      cache: "no-store",
    })
      .then(async (response) => {
        if (!response.ok)
          throw new Error(
            response.status === 401
              ? "Session expired. Sign in again to search the document."
              : "Unable to load document text. Retry or download the original PDF.",
          );
        return response.text();
      })
      .then((text) => {
        if (active) setMarkdown(text);
      })
      .catch((error) => {
        if (active)
          setError(
            error.name === "AbortError"
              ? "Document text took too long to load. Try again."
              : error.message,
          );
      })
      .finally(() => clearTimeout(timeout));
    return () => {
      active = false;
      clearTimeout(timeout);
      controller.abort();
    };
  }, [id, open, retry]);
  const pages = useMemo(() => documentPages(markdown ?? ""), [markdown]);
  const matches = useMemo(
    () => findPages(pages, deferredQuery),
    [pages, deferredQuery],
  );
  function search(value: string) {
    setQuery(value);
    setLimit(12);
    setPreviewPage(null);
  }
  return (
    <div className="rounded-lg border border-blue-200 bg-white p-4">
      <button
        type="button"
        aria-expanded={open}
        aria-controls={`document-finder-${id}`}
        onClick={() => setOpen(!open)}
        className="min-h-11 font-bold text-blue-700"
      >
        {open ? "Hide document finder" : "Find in document"}
      </button>
      {open ? (
        <div id={`document-finder-${id}`} className="mt-3 space-y-3">
          <p className="text-sm text-slate-600">
            Find evidence across the extracted pages. Verify important values
            against the original PDF.
          </p>
          <ValidatedLabel className="block text-sm font-semibold">
            Search document text
            <ValidatedInput
              type="search"
              value={query}
              maxLength={200}
              onChange={(event) => search(event.target.value)}
              placeholder="Keyword or phrase, e.g. earnest money"
              className="field mt-1 w-full"
            />
          </ValidatedLabel>
          <div
            className="flex flex-wrap gap-2"
            aria-label="Document search topics"
          >
            {topics.map(([label, terms]) => (
              <button
                key={label}
                type="button"
                aria-pressed={query === terms}
                onClick={() => search(terms)}
                className={`min-h-11 rounded-lg border px-3 text-sm font-semibold ${query === terms ? "border-blue-700 bg-blue-700 text-white" : "border-blue-200 text-blue-700"}`}
              >
                {label}
              </button>
            ))}
          </div>
          {error ? (
            <div role="alert" className="text-sm text-red-800">
              {error}{" "}
              <button
                type="button"
                className="min-h-11 underline"
                onClick={() => {
                  setError("");
                  setRetry(retry + 1);
                }}
              >
                Retry document search
              </button>
            </div>
          ) : markdown === undefined ? (
            <p role="status">Loading document text…</p>
          ) : !query.trim() ? (
            <p className="text-sm text-slate-600">
              Choose a topic or enter a phrase. Use | between alternatives.{" "}
              {pages.length} extracted sections available.
            </p>
          ) : (
            <>
              <p role="status" className="text-sm font-semibold">
                {matches.length} matching{" "}
                {matches.length === 1 ? "page" : "pages"}
              </p>
              {!matches.length ? (
                <p className="text-sm text-slate-600">
                  Try a shorter keyword, another spelling or a topic above. If
                  text was missed during extraction, check the original PDF.
                </p>
              ) : (
                <div
                  className={
                    previewPage ? "grid min-w-0 gap-3 lg:grid-cols-2" : ""
                  }
                >
                  <div className="max-h-[28rem] min-w-0 space-y-3 overflow-y-auto">
                    {matches.slice(0, limit).map((page, index) => (
                      <article
                        key={index}
                        className="rounded-lg border border-slate-200 p-3"
                      >
                        <p className="mb-2 font-bold text-blue-800">
                          {page.number === null
                            ? "Document text · page unavailable"
                            : `Page ${page.number}`}
                        </p>
                        <p className="break-words text-sm leading-6">
                          <Highlight
                            text={page.excerpt}
                            query={deferredQuery}
                          />
                        </p>
                        {page.number !== null ? (
                          <button
                            type="button"
                            onClick={() => setPreviewPage(page.number)}
                            aria-pressed={previewPage === page.number}
                            className="min-h-11 text-sm font-semibold text-blue-700"
                          >
                            Compare original page {page.number}
                          </button>
                        ) : null}
                        <details className="mt-2">
                          <summary className="min-h-11 cursor-pointer py-3 text-sm font-semibold text-blue-700">
                            Read full extracted page
                            {page.number ? ` ${page.number}` : ""}
                          </summary>
                          <pre className="max-h-96 overflow-y-auto whitespace-pre-wrap break-words rounded bg-slate-50 p-3 font-sans text-sm leading-6">
                            <Highlight text={page.text} query={deferredQuery} />
                          </pre>
                        </details>
                      </article>
                    ))}
                    {matches.length > limit ? (
                      <button
                        type="button"
                        className="min-h-11 font-semibold text-blue-700"
                        onClick={() => setLimit(limit + 12)}
                      >
                        Show more matching pages
                      </button>
                    ) : null}
                  </div>
                  {previewPage !== null ? (
                    <TenderPdfPreview
                      key={`${id}-${previewPage}`}
                      id={id}
                      page={previewPage}
                      onClose={() => setPreviewPage(null)}
                    />
                  ) : null}
                </div>
              )}
            </>
          )}
        </div>
      ) : null}
    </div>
  );
}
