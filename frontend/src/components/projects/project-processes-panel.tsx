"use client";

import { useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { ProjectProcessEditor } from "@/components/projects/project-process-editor";

import {
  type ProjectProcess,
  type ProcessBoard,
  processSecondary,
  processRequest,
  processStatus,
} from "@/components/projects/project-process-data";

export function ProjectProcessesPanel({
  projectId,
  onOpenWorkspace,
}: {
  projectId: string;
  onOpenWorkspace: (workspace: string) => void;
}) {
  const router = useRouter();
  const params = useSearchParams();
  const selectedId = params.get("process");
  const [data, setData] = useState<ProcessBoard | null>(null);
  const [error, setError] = useState("");
  const [reload, setReload] = useState(0);
  const [notice, setNotice] = useState("");
  useEffect(() => {
    const controller = new AbortController();
    void processRequest<ProcessBoard>(`/api/projects/${projectId}/processes`, {
      signal: controller.signal,
    })
      .then((result) => {
        if (!controller.signal.aborted) {
          setData(result);
          setError("");
        }
      })
      .catch((reason: unknown) => {
        if (!controller.signal.aborted)
          setError(
            reason instanceof Error
              ? reason.message
              : "Unable to load processes.",
          );
      });
    return () => controller.abort();
  }, [projectId, reload]);
  function select(id: string | null) {
    const query = new URLSearchParams({
      screen: "projects",
      project: projectId,
      tab: "processes",
    });
    if (id) query.set("process", id);
    router.push(`/dashboard?${query}`, { scroll: false });
    setNotice("");
  }
  if (!data)
    return (
      <section className="rounded-2xl border border-slate-200 bg-white p-6">
        <h3 className="text-xl font-bold">Project delivery processes</h3>
        {error ? (
          <div role="alert" className="mt-4 text-red-800">
            {error}
            <button
              className={processSecondary + " ml-3"}
              onClick={() => setReload((value) => value + 1)}
            >
              Retry
            </button>
          </div>
        ) : (
          <p role="status" className="mt-4 text-slate-600">
            Loading the project’s 19 delivery processes…
          </p>
        )}
      </section>
    );
  const selected = data.processes.find((item) => item.id === selectedId);
  const byId = new Map(data.processes.map((item) => [item.id, item]));
  const owners = new Map(data.owners.map((item) => [item.id, item.label]));
  const prerequisites = (id: string) =>
    data.dependencies
      .filter((edge) => edge.successor_id === id)
      .map((edge) => byId.get(edge.predecessor_id))
      .filter((item): item is ProjectProcess => !!item);
  const pendingCount = (id: string) =>
    prerequisites(id).filter(
      (item) => !["completed", "bypassed"].includes(item.status),
    ).length;
  const counts = [
    { label: "Delivery processes", value: data.processes.length },
    {
      label: "Completed",
      value: data.processes.filter((item) => item.status === "completed")
        .length,
    },
    {
      label: "In progress",
      value: data.processes.filter((item) => item.status === "in_progress")
        .length,
    },
    {
      label: "Waiting on prerequisites",
      value: data.processes.filter((item) => pendingCount(item.id) > 0).length,
    },
    {
      label: "Dependency maps to review",
      value: data.processes.filter((item) => !item.dependencies_confirmed)
        .length,
    },
  ];
  return (
    <section className="space-y-5">
      {error ? (
        <p role="alert" className="rounded-xl bg-red-50 p-4 text-red-800">
          {error}
          <button
            className="ml-3 underline"
            onClick={() => setReload((value) => value + 1)}
          >
            Retry
          </button>
        </p>
      ) : null}
      <p role="status" className="text-sm text-green-800">
        {notice}
      </p>
      {selectedId && !selected ? (
        <p role="alert">
          This process does not belong to the current project.
          <button className={processSecondary} onClick={() => select(null)}>
            All processes
          </button>
        </p>
      ) : selected ? (
        <>
          <button
            type="button"
            className={processSecondary}
            onClick={() => select(null)}
          >
            ← All 19 processes
          </button>
          <ProjectProcessEditor
            key={`${selected.id}-${selected.version}`}
            projectId={projectId}
            process={selected}
            board={data}
            onOpenWorkspace={onOpenWorkspace}
            onSaved={(record, predecessorIds) => {
              setData((current) =>
                current
                  ? {
                      ...current,
                      processes: current.processes.map((item) =>
                        item.id === record.id ? record : item,
                      ),
                      dependencies: [
                        ...current.dependencies.filter(
                          (edge) => edge.successor_id !== record.id,
                        ),
                        ...predecessorIds.map((id) => ({
                          predecessor_id: id,
                          successor_id: record.id,
                        })),
                      ],
                    }
                  : current,
              );
              setNotice(
                "Process saved. Dependencies and history have been updated.",
              );
              setReload((value) => value + 1);
            }}
          />
        </>
      ) : (
        <>
          <div className="rounded-2xl border border-slate-200 bg-white p-5 sm:p-6">
            <p className="text-xs font-bold uppercase tracking-[0.14em] text-blue-700">
              Project delivery
            </p>
            <h3 className="mt-2 text-2xl font-extrabold text-slate-900">
              19 processes. One project.
            </h3>
            <p className="mt-2 max-w-3xl text-sm leading-6 text-slate-600">
              Open a process to assign accountability, plan dates, map its
              prerequisites, and record its outcome. Dependencies belong to this
              project; processes can run in parallel.
            </p>
            <p className="mt-3 rounded-xl bg-blue-50 px-4 py-3 text-sm text-blue-900">
              Procurement runs alongside contractor work after BOM and
              quantity/item approval, targeting material delivery in process 16.
            </p>
          </div>
          <div className="grid grid-cols-2 gap-3 lg:grid-cols-5">
            {counts.map((item) => (
              <div
                key={item.label}
                className="rounded-xl border border-slate-200 bg-white p-4"
              >
                <p className="text-xs font-semibold text-slate-600">
                  {item.label}
                </p>
                <p className="mt-2 text-2xl font-extrabold text-slate-900">
                  {item.value}
                </p>
              </div>
            ))}
          </div>
          {!data.processes.length ? (
            <p
              role="alert"
              className="rounded-xl bg-amber-50 p-4 text-amber-900"
            >
              The process catalogue has not been initialized for this project.
              Contact the project administrator.
            </p>
          ) : null}
          <ol className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
            {data.processes.map((item) => {
              const required = prerequisites(item.id),
                pending = pendingCount(item.id);
              return (
                <li key={item.id}>
                  <button
                    type="button"
                    onClick={() => select(item.id)}
                    className="flex h-full w-full flex-col rounded-2xl border border-slate-200 bg-white p-5 text-left shadow-sm transition hover:border-blue-400 hover:shadow-md focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-600"
                  >
                    <div className="flex w-full items-center justify-between gap-3">
                      <span className={`flex h-9 w-9 items-center justify-center rounded-xl font-mono text-sm font-bold ${item.status === "completed" ? "bg-emerald-100 text-emerald-800 ring-2 ring-emerald-500/20" : "bg-blue-50 text-blue-800"}`}>
                        {String(item.position).padStart(2, "0")}
                      </span>
                      <span
                        className={`inline-flex items-center gap-1 rounded-full px-3 py-1 text-xs font-semibold capitalize ${item.status === "completed" ? "bg-emerald-50 text-emerald-800 border border-emerald-200" : item.status === "blocked" ? "bg-red-50 text-red-800" : item.status === "in_progress" ? "bg-blue-50 text-blue-800" : "bg-slate-100 text-slate-600"}`}
                      >
                        {item.status === "completed" && <span className="font-bold text-emerald-600">✓</span>}
                        {processStatus(item.status)}
                      </span>
                    </div>
                    <h4 className="mt-4 text-base font-bold leading-6 text-slate-900">
                      {item.name}
                    </h4>
                    <p className="mt-2 text-sm leading-6 text-slate-600">
                      {item.purpose}
                    </p>
                    <div className="mt-auto w-full pt-4 text-xs leading-5 text-slate-600">
                      <p>
                        <span className="font-semibold">Owner:</span>{" "}
                        {item.owner_user_id
                          ? (owners.get(item.owner_user_id) ??
                            "Inactive member")
                          : "Not assigned"}
                      </p>
                      <p>
                        <span className="font-semibold">Target:</span>{" "}
                        {item.planned_finish_date ?? "Not planned"}
                      </p>
                      <p
                        className={`mt-2 border-t border-slate-100 pt-3 ${!item.dependencies_confirmed || pending ? "text-amber-800" : "text-green-800"}`}
                      >
                        {!item.dependencies_confirmed
                          ? "Prerequisites not reviewed"
                          : pending
                            ? `${pending} prerequisite${pending === 1 ? "" : "s"} pending`
                            : required.length
                              ? "Prerequisites completed or bypassed"
                              : "No prerequisites · confirmed"}
                      </p>
                      {required.length ? (
                        <p className="mt-1 text-slate-500">
                          Depends on:{" "}
                          {required
                            .map((row) => `${row.position}. ${row.name}`)
                            .join(" · ")}
                        </p>
                      ) : null}
                    </div>
                    <span className="mt-4 text-sm font-bold text-blue-700">
                      Open process →
                    </span>
                  </button>
                </li>
              );
            })}
          </ol>
        </>
      )}
    </section>
  );
}
