"use client";
import { useEffect, useState, type FormEvent } from "react";
import {
  ValidatedForm,
  ValidatedInput,
  ValidatedLabel,
} from "@/components/ui/validated-controls";

import { TenderDocumentFinder } from "./tender-document-finder";

type Source = {
  tender_id: string;
  filename: string;
  sha256: string;
  page_count: number;
  state: string;
  warnings: string[];
  fields: {
    boq: unknown[];
    header: Record<string, { value: string; page: number; label: string }>;
  };
};

export function TenderPdfUpload({ onQueued }: { onQueued: () => void }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [progress, setProgress] = useState("");
  async function request(path: string, options: RequestInit) {
    let response = await fetch(`/api/tenders/uploads${path}`, options);
    for (let attempt = 0; response.status === 429 && attempt < 3; attempt++) {
      const seconds = Math.min(
        60,
        Math.max(1, Number(response.headers.get("Retry-After")) || 60),
      );
      setProgress(
        `Upload paused for ${seconds} seconds; it will resume automatically.`,
      );
      await new Promise((resolve) => setTimeout(resolve, seconds * 1000));
      response = await fetch(`/api/tenders/uploads${path}`, options);
    }
    const result = await response.json();
    if (!response.ok)
      throw new Error(
        typeof result.detail === "string"
          ? result.detail
          : "Unable to upload document.",
      );
    return result;
  }
  async function upload(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    const form = event.currentTarget;
    const files = new FormData(form)
      .getAll("file")
      .filter((file): file is File => file instanceof File && file.size > 0);
    if (
      !files.length ||
      files.some(
        (file) =>
          file.size > 104857600 || !file.name.toLowerCase().endsWith(".pdf"),
      )
    ) {
      setError("Choose PDF files up to 100 MB each.");
      return;
    }
    setError("");
    setBusy(true);
    const failures: string[] = [];
    let queued = 0;
    for (let index = 0; index < files.length; index++) {
      const file = files[index];
      let jobId: string | null = null;
      try {
        const job = await request("", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ filename: file.name, size_bytes: file.size }),
        });
        jobId = job.id;
        for (
          let start = 0, number = 0;
          start < file.size;
          start += 524288, number++
        ) {
          setProgress(
            `Uploading ${index + 1} of ${files.length}: ${file.name} · ${Math.round((start / file.size) * 100)}%`,
          );
          await request(`/${job.id}/chunks/${number}`, {
            method: "PUT",
            headers: { "Content-Type": "application/octet-stream" },
            body: file.slice(start, start + 524288),
          });
        }
        await request(`/${job.id}/enqueue`, { method: "POST" });
        queued++;
      } catch (error) {
        failures.push(
          `${file.name}: ${error instanceof Error ? error.message : "Upload failed"}`,
        );
        // A partially received document cannot create a tender. Keep uncertain jobs visible.
        if (jobId)
          failures.push(
            "Check the queue; discard an incomplete upload before trying again.",
          );
      }
    }
    setBusy(false);
    setProgress(`${queued} of ${files.length} documents queued.`);
    if (failures.length) setError(failures.join(" "));
    else {
      form.reset();
      onQueued();
    }
  }
  return (
    <div className="rounded-lg border border-blue-200 bg-white p-4">
      <h2 className="text-lg font-bold">Upload tender PDFs</h2>
      <p className="mt-1 text-sm text-slate-600">
        Select one or more documents. They will be processed in the background
        and appear in the register when ready.
      </p>
      <ValidatedForm
        onSubmit={(event) => void upload(event)}
        className="mt-4 flex flex-wrap items-end gap-3"
      >
        <ValidatedLabel className="min-w-0 flex-1">
          Tender PDFs
          <ValidatedInput
            name="file"
            type="file"
            accept="application/pdf,.pdf"
            multiple
            required
            disabled={busy}
            className="field max-w-full"
          />
        </ValidatedLabel>
        <button disabled={busy} className="tender-primary">
          {busy ? "Uploading…" : "Upload to queue"}
        </button>
      </ValidatedForm>
      <p className="mt-2 text-xs text-slate-600">
        Up to 100 MB per PDF. Keep this page open during upload; processing
        continues after it is queued. Extracted values need review.
      </p>
      {progress ? (
        <p role="status" className="mt-3 text-sm text-blue-800">
          {progress}
        </p>
      ) : null}
      {error ? (
        <p role="alert" className="mt-3 rounded-lg bg-red-50 p-3 text-red-800">
          {error}
        </p>
      ) : null}
    </div>
  );
}

export function TenderPdfSource({ id }: { id: string }) {
  const [source, setSource] = useState<Source | null>();
  const [error, setError] = useState("");
  useEffect(() => {
    const controller = new AbortController();
    void fetch(`/api/tenders/${id}/source`, {
      signal: controller.signal,
      cache: "no-store",
    })
      .then(async (response) => {
        if (!response.ok) throw new Error("Unable to load PDF extraction.");
        return response.json();
      })
      .then((result) => {
        if (!controller.signal.aborted) setSource(result);
      })
      .catch((error) => {
        if (!controller.signal.aborted) setError(error.message);
      });
    return () => controller.abort();
  }, [id]);
  if (error) return <p role="alert">{error}</p>;
  if (source === undefined)
    return <p role="status">Loading extracted source…</p>;
  if (source === null)
    return (
      <p className="text-sm text-amber-800">
        Legacy manual record: no uploaded source PDF.
      </p>
    );
  return (
    <section className="space-y-3 rounded-xl border border-blue-200 bg-blue-50 p-4">
      <h3 className="text-lg font-bold">Extracted from PDF · Needs review</h3>
      <p className="break-words text-sm">
        {source.filename} · {source.page_count} pages processed ·{" "}
        {source.fields.boq.length} BOQ rows extracted
      </p>
      <div className="flex flex-wrap gap-3">
        <a
          className="min-h-11 rounded-lg border bg-white px-3 py-3 font-bold text-blue-700"
          href={`/api/tenders/${id}/source/md`}
        >
          Download complete Markdown
        </a>
        <a
          className="min-h-11 rounded-lg border bg-white px-3 py-3 font-bold text-blue-700"
          href={`/api/tenders/${id}/source/pdf`}
        >
          Download original PDF
        </a>
      </div>
      <TenderDocumentFinder key={id} id={id} />
      <dl className="space-y-3">
        {Object.entries(source.fields.header).map(([key, entry]) => (
          <div key={key}>
            <dt className="font-bold capitalize">{key.replaceAll("_", " ")}</dt>
            <dd className="whitespace-pre-wrap break-words text-sm">
              {entry.value}
              <span className="ml-2 text-xs text-slate-600">
                (page {entry.page})
              </span>
            </dd>
          </div>
        ))}
      </dl>
      <ul className="list-disc space-y-2 pl-5 text-sm text-amber-900">
        {source.warnings.map((warning, index) => (
          <li key={index}>{warning}</li>
        ))}
      </ul>
      <p className="break-all text-xs text-slate-600">
        PDF SHA-256: {source.sha256}
      </p>
    </section>
  );
}
