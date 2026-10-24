"use client";
import { useEffect, useState } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { useProjectUser } from "@/components/projects/project-permissions";
import { permitted, canOpen, purchaseStepAllowed } from "@/components/projects/workspace-permissions";
import { PurchaseBOWorkspace } from "./purchase-bo-workspace";
import { PurchaseWorkflow } from "./purchase-workflow";
import { DataGrid, GridStatus } from "@/components/ui/data-grid";
import { PurchaseHolds } from "./purchase-holds";
import { PurchaseIndents } from "./purchase-indents";
import {
  request,
  type Overview,
  type RecordRow,
  type HoldData,
  type IndentData,
  type DemandSource,
} from "./purchase-data";
export function ProjectPurchasing({
  project,
  scopeSummary,
  projectName,
  forceLive = false,
}: {
  project: string;
  scopeSummary?: string;
  projectName?: string;
  forceLive?: boolean;
}) {
  const user = useProjectUser();
  const params = useSearchParams(),
    router = useRouter();
  const canViewWorkflow = canOpen(user.permissions,"projects.purchase_workflow");
  const canViewLive = ["purchase_indents","purchase_holds"].some(area=>canOpen(user.permissions,"projects."+area));
  const canViewBO = permitted(user.permissions,"projects.purchase_bo.read") && Array.from({length:20},(_,i)=>i+1).some(step=>purchaseStepAllowed(user.permissions,step));
  const requestedMode = params.get("purchaseView");
  const mode = forceLive ? "live" : requestedMode || (canViewLive ? "live" : canViewBO ? "bo" : "workflow");
  const modeAllowed = mode==="bo" ? canViewBO : mode==="workflow" ? canViewWorkflow : canViewLive;
  function setMode(value: string) {
    const q = new URLSearchParams(params.toString());
    q.set("purchaseView", value);
    q.delete("purchase");
    q.delete("purchaseStep");
    router.replace(`/dashboard?${q}`, { scroll: false });
  }
  const [overview, setOverview] = useState<Overview | null>(null),
    [holds, setHolds] = useState<RecordRow<HoldData>[]>([]),
    [indents, setIndents] = useState<RecordRow<IndentData>[]>([]),
    [error, setError] = useState(""),
    [reload, setReload] = useState(0);
  useEffect(() => {
    if (mode !== "live" || !modeAllowed) return;
    let live = true;
    async function load() {
      try {
        const o = await request<Overview>(
          `/api/projects/${project}/purchase-overview`,
        );
        const [h, i] = await Promise.all([
          o.permissions["purchase_holds.read"]
            ? request<RecordRow<HoldData>[]>(
                `/api/projects/${project}/purchase-holds`,
              )
            : Promise.resolve([]),
          o.permissions["purchase_indents.read"]
            ? request<RecordRow<IndentData>[]>(
                `/api/projects/${project}/purchase-indents`,
              )
            : Promise.resolve([]),
        ]);
        if (live) {
          setOverview(o);
          setHolds(h);
          setIndents(i);
          setError("");
        }
      } catch (e) {
        if (live) setError((e as Error).message);
      }
    }
    void load();
    return () => {
      live = false;
    };
  }, [project, reload, mode, modeAllowed]);
  if (!modeAllowed) return <p role="status" className="p-6">You do not have permission to open this purchasing view.</p>;
  if (mode === "bo")
    return (
      <PurchaseBOWorkspace
        project={project}
        projectName={projectName}
        scopeSummary={scopeSummary}
        openPackages={() => setMode("workflow")}
      />
    );
  return (
    <section className="space-y-6">
      <header>
        <h2 className="text-2xl font-bold">Purchase FMS</h2>
        <p className="mt-2 text-sm text-slate-600">
          Client make policy, purchase holds and reviewed demand govern release.
          A released indent authorises procurement preparation; it is not a
          purchase order.
        </p>
        {scopeSummary && (
          <p className="mt-2 text-sm text-slate-700">{scopeSummary}</p>
        )}
      </header>
      <div
        className="flex flex-wrap gap-2 border-b border-slate-200 pb-3"
        aria-label="Purchasing view"
      >
        {!forceLive &&
          (user.permissions.includes("system:admin") ||
            (purchaseStepAllowed(user.permissions,1) || purchaseStepAllowed(user.permissions,2) || purchaseStepAllowed(user.permissions,3) || purchaseStepAllowed(user.permissions,4))) && (
            <button
              className="min-h-11 rounded-lg border border-slate-300 px-4 text-sm font-bold"
              onClick={() => setMode("bo")}
            >
              1 · BO list workspace
            </button>
          )}
        {canViewLive && <button
          className={`min-h-11 rounded-lg px-4 text-sm font-bold ${mode === "live" ? "bg-slate-900 text-white" : "border border-slate-300 bg-white text-slate-700"}`}
          aria-pressed={mode === "live"}
          onClick={() => setMode("live")}
        >
          1–2 · Requirements & indent
        </button>}
        {canViewWorkflow && (
          <button
            className={`min-h-11 rounded-lg px-4 text-sm font-bold ${mode === "workflow" ? "bg-slate-900 text-white" : "border border-slate-300 bg-white text-slate-700"}`}
            aria-pressed={mode === "workflow"}
            onClick={() => setMode("workflow")}
          >
            3–20 · Purchase workspaces
          </button>
        )}
      </div>
      {mode === "workflow" ? (
        <PurchaseWorkflow project={project} />
      ) : (
        <>
          {error && (
            <p role="alert" className="text-red-700">
              {error}
            </p>
          )}
          {overview && overview.lines.length === 0 && (
            <div className="rounded-xl border border-blue-200 bg-blue-50 p-4">
              <h3 className="font-bold">Start with the material requirement</h3>
              <p className="mt-1 text-sm">
                Confirm the item specification, quantity and unit from the
                source indent, then add the project material. The purchase
                indent follows this review.
              </p>
              <button
                className="mt-3 min-h-11 rounded-lg bg-blue-700 px-4 text-sm font-bold text-white"
                onClick={() => {
                  const q = new URLSearchParams(params.toString());
                  q.set("tab", "items");
                  q.delete("purchaseView");
                  router.push(`/dashboard?${q}`);
                }}
              >
                Define material requirements
              </button>
            </div>
          )}
          {overview ? (
            <>
              <DataGrid<DemandSource>
                label="Purchase readiness"
                actions={() => null}
                rows={overview.lines}
                columns={[
                  {
                    key: "name",
                    label: "Item / configuration",
                    value: (r) => r.item.description,
                  },
                  {
                    key: "make",
                    label: "Manufacturer",
                    value: (r) => r.item.manufacturer,
                  },
                  {
                    key: "qty",
                    label: "Required quantity",
                    value: (r) => `${r.quantity} ${r.item.uom}`,
                  },
                  {
                    key: "released",
                    label: "Released quantity",
                    value: (r) => r.released_quantity ?? "Unavailable",
                  },
                  {
                    key: "remaining",
                    label: "Remaining quantity",
                    value: (r) => r.remaining_quantity ?? "Unavailable",
                  },
                  {
                    key: "date",
                    label: "Required on site",
                    value: (r) => r.required_date,
                  },
                  {
                    key: "policy",
                    label: "Client make policy",
                    value: (r) => r.policy.status,
                    render: (r) => (
                      <div>
                        <GridStatus
                          value={r.policy.status.replaceAll("_", " ")}
                        />
                        <p className="max-w-xs text-xs text-slate-500">
                          {r.policy.reasons.join("; ")}
                        </p>
                      </div>
                    ),
                  },
                  {
                    key: "hold",
                    label: "Client hold",
                    value: (r) =>
                      r.holds.holds.map((h) => h.reason).join("; ") || "None",
                  },
                  {
                    key: "duration",
                    label: "Recorded hold time",
                    value: (r) => `${(r.holds.hold_minutes / 60).toFixed(1)} h`,
                  },
                ]}
              />
              {overview.permissions["purchase_holds.read"] ? (
                <PurchaseHolds
                  project={project}
                  overview={overview}
                  rows={holds}
                  reload={() => setReload((n) => n + 1)}
                />
              ) : (
                <p className="text-sm text-slate-500">
                  Hold details require purchase-hold read access; release gates
                  still apply.
                </p>
              )}
              {overview.permissions["purchase_indents.read"] && (
                <PurchaseIndents
                  project={project}
                  overview={overview}
                  rows={indents}
                  reload={() => setReload((n) => n + 1)}
                />
              )}
            </>
          ) : (
            !error && <p role="status">Loading purchase controls…</p>
          )}
        </>
      )}
    </section>
  );
}
