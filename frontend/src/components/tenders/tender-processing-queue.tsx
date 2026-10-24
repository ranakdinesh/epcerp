"use client";

import { useEffect, useRef, useState } from "react";
import { DataGrid, type GridColumn } from "@/components/ui/data-grid";
import { RecordDrawer } from "@/components/ui/record-drawer";

type QueueJob = {
  id: string;
  filename: string;
  size_bytes: number;
  state: string;
  received_chunks: number;
  chunk_count: number;
  attempts: number;
  tender_id: string | null;
  error: string;
  created_at: string;
  updated_at: string;
  can_manage: boolean;
  events: { state: string; at: string }[];
};
const names: Record<string, string> = {
  uploading: "Uploading",
  queued: "Queued",
  processing: "Processing",
  completed: "Completed",
  failed: "Failed",
  cancelled: "Discarded",
};
const columns: GridColumn<QueueJob>[] = [
  {
    key: "filename",
    label: "Document",
    value: (row) => row.filename,
    render: (row) => (
      <span className="break-words font-semibold">
        {row.filename}
        <small className="block font-normal text-slate-500">
          {(row.size_bytes / 1048576).toFixed(1)} MB
        </small>
      </span>
    ),
  },
  {
    key: "state",
    label: "Status",
    value: (row) => names[row.state] || row.state,
    render: (row) => (
      <span className={`tender-job-status status-${row.state}`}>
        {names[row.state] || row.state}
      </span>
    ),
  },
  { key: "attempts", label: "Attempts", value: (row) => row.attempts },
  {
    key: "created",
    label: "Uploaded",
    value: (row) => row.created_at,
    render: (row) => new Date(row.created_at).toLocaleString(),
  },
  {
    key: "detail",
    label: "Details",
    value: (row) =>
      row.error ||
      (row.state === "uploading"
        ? `${row.received_chunks} / ${row.chunk_count} chunks received`
        : row.state === "completed"
          ? "Ready for review"
          : row.state === "queued"
            ? "Waiting for processor"
            : row.state === "processing"
              ? "Extracting document"
              : "—"),
  },
];

export function TenderProcessingQueue({
  onOpen,
  onCompleted,
}: {
  onOpen: (id: string) => void;
  onCompleted: () => void;
}) {
  const [rows, setRows] = useState<QueueJob[]>([]);
  const [total, setTotal] = useState(0);
  const limit = 50;
  const [offset, setOffset] = useState(0);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState<string | null>(null);
  const [selected, setSelected] = useState<QueueJob | null>(null);
  const [refresh, setRefresh] = useState(0);
  const completed = useRef(new Set<string>());
  const initialized = useRef(false);
  const callback = useRef(onCompleted);
  useEffect(() => {
    callback.current = onCompleted;
  }, [onCompleted]);
  useEffect(() => {
    const controller = new AbortController();
    let timer: ReturnType<typeof setTimeout>;
    async function load() {
      try {
        const response = await fetch(
          `/api/tenders/uploads?limit=${limit}&offset=${offset}`,
          {
            cache: "no-store",
            signal: controller.signal,
          },
        );
        if (!response.ok)
          throw new Error(
            response.status === 403
              ? "Your account needs tender processing queue access."
              : "Unable to load the queue.",
          );
        const page = (await response.json()) as {
          items: QueueJob[];
          total: number;
        };
        if (controller.signal.aborted) return;
        if (
          initialized.current &&
          page.items.some(
            (job) =>
              job.state === "completed" && !completed.current.has(job.id),
          )
        )
          callback.current();
        completed.current = new Set(
          page.items
            .filter((job) => job.state === "completed")
            .map((job) => job.id),
        );
        initialized.current = true;
        setRows(page.items);
        setTotal(page.total);
        setError("");
      } catch (error) {
        if (!controller.signal.aborted)
          setError(
            error instanceof Error ? error.message : "Unable to load queue.",
          );
      } finally {
        if (!controller.signal.aborted) {
          setLoading(false);
          timer = setTimeout(() => void load(), 5000);
        }
      }
    }
    void load();
    return () => {
      controller.abort();
      clearTimeout(timer);
    };
  }, [limit, offset, refresh]);
  async function action(job: QueueJob, kind: "retry" | "discard") {
    if (busy) return;
    setBusy(job.id);
    setError("");
    try {
      const response = await fetch(`/api/tenders/uploads/${job.id}/${kind}`, {
        method: "POST",
      });
      const result = await response.json();
      if (!response.ok)
        throw new Error(
          typeof result.detail === "string"
            ? result.detail
            : "Unable to update job.",
        );
      setRefresh((value) => value + 1);
    } catch (error) {
      setError(
        error instanceof Error ? error.message : "Unable to update job.",
      );
    } finally {
      setBusy(null);
    }
  }
  return (
    <section className="tender-queue space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="text-lg font-bold">Processing queue</h2>
          <p className="text-xs text-slate-500">
            Updates every 5 seconds. You can leave this page after a document is
            queued.
          </p>
        </div>
        <button
          className="tender-secondary"
          onClick={() => setRefresh((value) => value + 1)}
        >
          Refresh queue
        </button>
      </div>
      <div className="tender-queue-summary" aria-label="Queue summary">
        {["uploading", "queued", "processing", "completed", "failed"].map(
          (state) => (
            <span key={state}>
              <strong>
                {rows.filter((row) => row.state === state).length}
              </strong>{" "}
              {names[state]}
            </span>
          ),
        )}
        <small>Loaded jobs</small>
      </div>
      {error ? (
        <p role="alert" className="rounded-lg bg-red-50 p-3 text-red-800">
          {error}
        </p>
      ) : null}
      <DataGrid
        label="Processing queue"
        rows={rows}
        columns={columns}
        loading={loading}
        initialSort="created"
        onRowClick={(row) => setSelected(row)}
        actions={(row) => (
          <div className="flex flex-wrap justify-end gap-1">
            {row.tender_id ? (
              <button
                className="tender-secondary"
                onClick={() => onOpen(row.tender_id!)}
              >
                Open tender
              </button>
            ) : null}
            {row.can_manage && row.state === "failed" && row.attempts < 3 ? (
              <button
                disabled={busy === row.id}
                className="tender-secondary"
                onClick={() => void action(row, "retry")}
              >
                Retry
              </button>
            ) : null}
            {row.can_manage &&
            ["uploading", "queued", "failed"].includes(row.state) ? (
              <button
                disabled={busy === row.id}
                className="tender-secondary"
                onClick={() => void action(row, "discard")}
              >
                Discard
              </button>
            ) : null}
            <button
              className="tender-secondary"
              onClick={() => setSelected(row)}
            >
              History
            </button>
          </div>
        )}
      />
      <p role="status" className="text-xs text-slate-500">
        {rows.length} of {total} accessible jobs loaded.
      </p>
      <div className="flex gap-3">
        <button
          className="tender-secondary"
          disabled={offset === 0}
          onClick={() => setOffset((value) => Math.max(0, value - limit))}
        >
          Previous jobs
        </button>
        <button
          className="tender-secondary"
          disabled={offset + rows.length >= total}
          onClick={() => setOffset((value) => value + limit)}
        >
          Next jobs
        </button>
      </div>
      {selected ? (
        <RecordDrawer
          title={selected.filename}
          onClose={() => setSelected(null)}
        >
          <h2 className="font-bold">Processing history</h2>
          <ol className="mt-4 space-y-3">
            {selected.events.map((event, index) => (
              <li key={index} className="rounded-lg border p-3">
                <strong>{names[event.state] || event.state}</strong>
                <p className="text-sm text-slate-500">
                  {new Date(event.at).toLocaleString()}
                </p>
              </li>
            ))}
          </ol>
          {selected.error ? (
            <p role="alert" className="mt-4 text-red-700">
              {selected.error}
            </p>
          ) : null}
        </RecordDrawer>
      ) : null}
    </section>
  );
}
