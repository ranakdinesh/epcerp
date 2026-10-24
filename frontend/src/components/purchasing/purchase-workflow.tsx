"use client";
import { ValidatedForm, ValidatedInput, ValidatedLabel, ValidatedSelect, ValidatedTextarea } from "@/components/ui/validated-controls";
import { useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { DataGrid, GridAction, GridStatus } from "@/components/ui/data-grid";
import { RecordDrawer } from "@/components/ui/record-drawer";
import { request, input, button, secondary } from "./purchase-data";
import { PurchaseTaskWorkspace } from "./purchase-task-workspace";
import {
  workflowStatus,
  type WorkflowList,
  type PurchasePackage,
} from "./workflow-data";

export function PurchaseWorkflow({ project }: { project: string }) {
  const params = useSearchParams(),
    router = useRouter();
  const selected = params.get("purchase");
  const [data, setData] = useState<WorkflowList | null>(null),
    [error, setError] = useState("");
  const [reload, setReload] = useState(0),
    [mine, setMine] = useState(false),
    [create, setCreate] = useState(false);
  const [source, setSource] = useState(""),
    [reason, setReason] = useState(""),
    [busy, setBusy] = useState(false);
  useEffect(() => {
    let live = true;
    request<WorkflowList>(`/api/projects/${project}/purchase-workflow`)
      .then((d) => {
        if (live) {
          setData(d);
          setError("");
        }
      })
      .catch((e) => {
        if (live) {
          setData(null);
          setError(e.message);
        }
      });
    return () => {
      live = false;
    };
  }, [project, reload]);
  function open(id: string | null) {
    const q = new URLSearchParams(params.toString());
    if (id) q.set("purchase", id);
    else q.delete("purchase");
    q.delete("purchaseStep");
    q.set("purchaseView", "workflow");
    router.replace(`/dashboard?${q.toString()}`, { scroll: false });
  }
  if (selected)
    return (
      <PurchaseTaskWorkspace
        key={selected}
        project={project}
        identity={selected}
        back={() => {
          open(null);
          setReload((n) => n + 1);
        }}
      />
    );
  return (
    <section className="min-w-0 space-y-4">
      <div className="flex flex-wrap justify-between gap-3">
        <div>
          <h3 className="text-xl font-bold">Purchase workspaces</h3>
          <p className="mt-1 text-sm text-slate-600">
            Released requirements, accountable people and saved preparation in
            one place.
          </p>
        </div>
        {data?.can_create && (
          <button
            className={button}
            onClick={() => {
              setCreate(true);
              setError("");
            }}
          >
            Create from released indent
          </button>
        )}
      </div>
      <p className="rounded-xl border border-blue-200 bg-blue-50 p-3 text-sm text-blue-900">
        Assignments, notes and blockers are saved. Steps 3–5 include RFQs,
        quotation comparison, independent selection approval and recorded LOI
        release. Technical approvals, purchase orders and downstream execution
        remain pending.
      </p>
      {error && (
        <div
          role="alert"
          className="rounded-lg bg-red-50 p-3 text-sm text-red-800"
        >
          {error}
          <button
            className={`${secondary} ml-3`}
            onClick={() => setReload((n) => n + 1)}
          >
            Retry
          </button>
        </div>
      )}
      {!data && !error && <p role="status">Loading purchase workspaces…</p>}
      {data && (
        <>
          <ValidatedLabel className="flex min-h-11 items-center gap-2 text-sm">
            <ValidatedInput
              type="checkbox"
              checked={mine}
              onChange={(e) => setMine(e.target.checked)}
            />
            My assigned preparation and blocker resolutions
          </ValidatedLabel>
          <DataGrid<PurchasePackage>
            label="Purchase workspaces"
            rows={data.packages.filter(
              (p) =>
                !mine ||
                p.steps.some(
                  (s) =>
                    s.doer_id === data.actor_id ||
                    s.blocked_resolver_id === data.actor_id,
                ),
            )}
            status={workflowStatus}
            columns={[
              { key: "code", label: "Package", value: (r) => r.code },
              {
                key: "item",
                label: "Material requirement",
                value: (r) => r.description,
              },
              {
                key: "state",
                label: "Coordination status",
                value: workflowStatus,
                render: (r) => <GridStatus value={workflowStatus(r)} />,
              },
              {
                key: "assigned",
                label: "Assigned steps",
                value: (r) =>
                  `${r.steps.filter((s) => s.number > 2 && s.doer_id && s.resolver_id && s.planned_date).length} / 18`,
              },
              {
                key: "blocked",
                label: "Open blockers",
                value: (r) =>
                  r.steps.filter((s) => s.state === "blocked").length,
              },
            ]}
            actions={(r) => (
              <GridAction label={`Open ${r.code}`} onClick={() => open(r.id)} />
            )}
          />
          {!data.packages.length && (
            <p className="text-sm text-slate-600">
              No packages have been created. Release an indent line using the
              live controls, then create its workspace.
            </p>
          )}
        </>
      )}
      {create && data && (
        <RecordDrawer
          title="Create purchase workspace"
          onClose={() => {
            if (!busy) setCreate(false);
          }}
        >
          <ValidatedForm
            className="space-y-4"
            onSubmit={async (e) => {
              e.preventDefault();
              const candidate = data.candidates[Number(source)];
              if (source === "" || !candidate) return;
              setBusy(true);
              try {
                const result = await request<{ package: PurchasePackage }>(
                  `/api/projects/${project}/purchase-workflow`,
                  {
                    indent_id: candidate.indent_id,
                    demand_id: candidate.demand_id,
                    reason,
                  },
                );
                setCreate(false);
                open(result.package.id);
              } catch (e) {
                setError((e as Error).message);
              } finally {
                setBusy(false);
              }
            }}
          >
            <p className="text-sm text-slate-600">
              Uses the existing reviewed/released quantity and full-kit
              reference. No supplier order is created. Each released line can be
              linked once.
            </p>
            <ValidatedLabel className="block text-sm font-semibold">
              Released indent line
              <ValidatedSelect name="source"
                required
                className={input}
                value={source}
                onChange={(e) => setSource(e.target.value)}
              >
                <option value="">Select released requirement</option>
                {data.candidates.map((c, i) => (
                  <option
                    key={`${c.indent_id}-${c.demand_id}`}
                    value={i}
                    disabled={data.packages.some(
                      (p) =>
                        p.indent_id === c.indent_id &&
                        p.demand_id === c.demand_id,
                    )}
                  >
                    {c.indent_number} · {c.description} · {c.quantity}
                  </option>
                ))}
              </ValidatedSelect>
            </ValidatedLabel>
            <ValidatedLabel className="block text-sm font-semibold">
              Creation reason
              <ValidatedTextarea name="reason"
                required
                maxLength={2000}
                className={input}
                value={reason}
                onChange={(e) => setReason(e.target.value)}
              />
            </ValidatedLabel>
            {error && (
              <p role="alert" className="text-sm text-red-700">
                {error}
              </p>
            )}
            <button disabled={busy || !source} className={button}>
              {busy ? "Creating…" : "Create workspace"}
            </button>
          </ValidatedForm>
        </RecordDrawer>
      )}
    </section>
  );
}
