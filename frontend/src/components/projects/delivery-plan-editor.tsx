"use client";
import { ValidatedInput, ValidatedLabel, ValidatedSelect, ValidatedTextarea } from "@/components/ui/validated-controls";
import { useState } from "react";
import Link from "next/link";
import {
  DeliveryBoard,
  DeliveryNode,
  outline,
  depth,
  siblingTotal,
  planDates,
  deliveryField as field,
} from "./delivery-plan-data";
import {
  processRequest,
  processButton,
  processSecondary,
} from "./project-process-data";

export function DeliveryPlanEditor({
  projectId,
  board,
  onSaved,
  onClose,
}: {
  projectId: string;
  board: DeliveryBoard;
  onSaved: () => void;
  onClose: () => void;
}) {
  const plan = board.plan!;
  const [nodes, setNodes] = useState(board.nodes);
  const [selected, setSelected] = useState(nodes[0]?.code ?? "");
  const [manager, setManager] = useState(plan.manager_id ?? "");
  const [start, setStart] = useState(plan.start_date ?? "");
  const [finish, setFinish] = useState(plan.finish_date ?? "");
  const [reason, setReason] = useState("");
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const [rootDoer, setRootDoer] = useState("");
  const [rootResolver, setRootResolver] = useState("");
  const node = nodes.find((n) => n.code === selected);
  function update(change: Partial<DeliveryNode>) {
    setNodes((v) =>
      v.map((n) => (n.code === selected ? { ...n, ...change } : n)),
    );
  }
  async function save(activate: boolean) {
    setError("");
    if (!manager || !start || !finish || !reason.trim()) {
      setError(
        "Select a project manager, both project dates, and a review reason.",
      );
      return;
    }
    setSaving(true);
    try {
      await processRequest(`/api/projects/${projectId}/delivery-plan`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          expected_version: plan.version,
          manager_id: manager,
          start_date: start,
          finish_date: finish,
          activate,
          reason,
          nodes: nodes.map((n) => ({
            code: n.code,
            name: n.name,
            weight_bp: n.weight_bp,
            start_date: n.start_date,
            finish_date: n.finish_date,
            doer_id: n.doer_id,
            resolver_id: n.resolver_id,
            resolution_minutes: n.resolution_minutes,
            predecessors: n.predecessors,
          })),
        }),
      });
      onSaved();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setSaving(false);
    }
  }
  const memberOptions = board.members.map((m) => (
    <option key={m.id} value={m.id}>
      {m.label}
    </option>
  ));
  return (
    <section className="space-y-4 rounded-2xl border border-blue-200 bg-white p-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h3 className="text-xl font-bold">Review project task plan</h3>
          <p className="mt-1 text-sm text-slate-600">
            Weights and dates can be changed while this plan is a draft.
            Activation freezes the reviewed scope and dates.
          </p>
        </div>
        <button onClick={onClose} className={processSecondary}>
          Close review
        </button>
      </div>
      {error ? (
        <p role="alert" className="rounded-xl bg-red-50 p-4 text-red-800">
          {error}
        </p>
      ) : null}
      {!board.members.length ? (
        <p className="rounded-xl bg-amber-50 p-4 text-sm text-amber-900">
          Assign active project members before staffing this plan.{" "}
          <Link
            className="underline"
            href={`/dashboard?screen=projects&project=${projectId}&tab=team`}
          >
            Open Project team
          </Link>
        </p>
      ) : null}
      <fieldset disabled={saving} className="space-y-4">
        <div className="grid gap-3 sm:grid-cols-3">
          <ValidatedLabel className="text-sm font-bold">
            Project manager
            <ValidatedSelect name="manager"
              aria-label="Project manager"
              value={manager}
              onChange={(e) => setManager(e.target.value)}
              className={field}
            >
              <option value="">Select project member</option>
              {memberOptions}
            </ValidatedSelect>
          </ValidatedLabel>
          <ValidatedLabel className="text-sm font-bold">
            Plan start date
            <ValidatedInput name="start"
              type="date"
              value={start}
              onChange={(e) => setStart(e.target.value)}
              className={field}
            />
          </ValidatedLabel>
          <ValidatedLabel className="text-sm font-bold">
            Plan finish date
            <ValidatedInput name="finish"
              type="date"
              value={finish}
              onChange={(e) => setFinish(e.target.value)}
              className={field}
            />
          </ValidatedLabel>
        </div>
        <button
          className={processSecondary}
          onClick={() => {
            try {
              const dates = planDates(nodes, start, finish);
              setNodes((v) =>
                v.map((n) => ({
                  ...n,
                  start_date: dates.get(n.code)![0],
                  finish_date: dates.get(n.code)![1],
                })),
              );
              setError("");
            } catch (e) {
              setError((e as Error).message);
            }
          }}
        >
          Generate dates from template windows
        </button>
        <p className="text-xs text-slate-500">
          This replaces the local draft dates using the copied template windows.
          Saving records the change; project-master contract dates remain
          unchanged.
        </p>
        <div className="rounded-xl bg-slate-50 p-4">
          <h4 className="font-bold">Initial process assignments</h4>
          <p className="mt-1 text-xs text-slate-600">
            Set defaults on all 19 process roots, then override any major task
            or subtask below. Existing child overrides remain.
          </p>
          <div className="mt-3 flex flex-wrap items-end gap-3">
            <ValidatedLabel className="min-w-40 flex-1 text-sm font-bold">
              Default doer
              <ValidatedSelect name="root_doer"
                aria-label="Default doer"
                value={rootDoer}
                onChange={(e) => setRootDoer(e.target.value)}
                className={field}
              >
                <option value="">Choose doer</option>
                {memberOptions}
              </ValidatedSelect>
            </ValidatedLabel>
            <ValidatedLabel className="min-w-40 flex-1 text-sm font-bold">
              Default problem resolver
              <ValidatedSelect name="root_resolver"
                aria-label="Default problem resolver"
                value={rootResolver}
                onChange={(e) => setRootResolver(e.target.value)}
                className={field}
              >
                <option value="">Choose resolver</option>
                {memberOptions}
              </ValidatedSelect>
            </ValidatedLabel>
            <button
              className={processSecondary}
              onClick={() => {
                if (!rootDoer || !rootResolver || rootDoer === rootResolver) {
                  setError("Choose two different active project members.");
                  return;
                }
                setNodes((v) =>
                  v.map((n) =>
                    n.parent_code
                      ? n
                      : { ...n, doer_id: rootDoer, resolver_id: rootResolver },
                  ),
                );
                setError("");
              }}
            >
              Apply to all processes
            </button>
          </div>
        </div>
        <div className="grid items-start gap-4 lg:grid-cols-2">
          <div className="max-h-[600px] overflow-y-auto rounded-xl border border-slate-200">
            {outline(nodes).map((n) => (
              <button
                key={n.code}
                onClick={() => setSelected(n.code)}
                className={`block min-h-12 w-full border-b border-slate-100 py-2 pr-2 text-left ${selected === n.code ? "bg-blue-50" : "hover:bg-slate-50"}`}
                style={{ paddingLeft: 12 + depth(n.code, nodes) * 15 }}
              >
                <span className="block text-sm font-bold">
                  {n.code} · {n.name}
                </span>
                <span className="block text-xs text-slate-600">
                  {n.weight_bp / 100}% of parent · {n.start_date ?? "No start"}{" "}
                  → {n.finish_date ?? "No finish"}
                </span>
              </button>
            ))}
          </div>
          {node ? (
            <div className="space-y-3">
              <h4 className="font-bold">{node.code} · Task settings</h4>
              <ValidatedLabel className="block text-sm font-bold">
                Task name
                <ValidatedInput
                  value={node.name}
                  onChange={(e) => update({ name: e.target.value })}
                  className={field}
                />
              </ValidatedLabel>
              <div className="grid grid-cols-2 gap-3">
                <ValidatedLabel className="text-sm font-bold">
                  Contribution to parent (%)
                  <ValidatedInput
                    type="number"
                    min="0.01"
                    max="100"
                    step="0.01"
                    value={node.weight_bp / 100}
                    onChange={(e) =>
                      update({
                        weight_bp: Math.round(Number(e.target.value) * 100),
                      })
                    }
                    className={field}
                  />
                </ValidatedLabel>
                <ValidatedLabel className="text-sm font-bold">
                  Resolver allowance (hours)
                  <ValidatedInput
                    type="number"
                    min="0.0167"
                    max="720"
                    step="any"
                    value={
                      node.resolution_minutes === null
                        ? ""
                        : node.resolution_minutes / 60
                    }
                    placeholder="Inherit"
                    onChange={(e) =>
                      update({
                        resolution_minutes: e.target.value
                          ? Math.round(Number(e.target.value) * 60)
                          : null,
                      })
                    }
                    className={field}
                  />
                </ValidatedLabel>
                <ValidatedLabel className="text-sm font-bold">
                  Task start date
                  <ValidatedInput
                    type="date"
                    value={node.start_date ?? ""}
                    onChange={(e) => update({ start_date: e.target.value })}
                    className={field}
                  />
                </ValidatedLabel>
                <ValidatedLabel className="text-sm font-bold">
                  Task finish date
                  <ValidatedInput
                    type="date"
                    value={node.finish_date ?? ""}
                    onChange={(e) => update({ finish_date: e.target.value })}
                    className={field}
                  />
                </ValidatedLabel>
                <ValidatedLabel className="text-sm font-bold">
                  Doer override
                  <ValidatedSelect
                    value={node.doer_id ?? ""}
                    onChange={(e) =>
                      update({ doer_id: e.target.value || null })
                    }
                    className={field}
                  >
                    <option value="">
                      {node.parent_code ? "Inherit from parent" : "Unassigned"}
                    </option>
                    {memberOptions}
                  </ValidatedSelect>
                </ValidatedLabel>
                <ValidatedLabel className="text-sm font-bold">
                  Resolver override
                  <ValidatedSelect
                    value={node.resolver_id ?? ""}
                    onChange={(e) =>
                      update({ resolver_id: e.target.value || null })
                    }
                    className={field}
                  >
                    <option value="">
                      {node.parent_code ? "Inherit from parent" : "Unassigned"}
                    </option>
                    {memberOptions}
                  </ValidatedSelect>
                </ValidatedLabel>
              </div>
              <p
                className={`text-sm ${siblingTotal(node, nodes) === 100 ? "text-emerald-700" : "text-red-700"}`}
              >
                Sibling contribution: {siblingTotal(node, nodes)} / 100%
              </p>
              <ValidatedLabel className="block text-sm font-bold">
                Prerequisite tasks (select all that apply)
                <ValidatedSelect
                  multiple
                  size={6}
                  value={node.predecessors}
                  onChange={(e) =>
                    update({
                      predecessors: Array.from(
                        e.target.selectedOptions,
                        (o) => o.value,
                      ),
                    })
                  }
                  className={field}
                >
                  {nodes
                    .filter((n) => n.code !== node.code)
                    .map((n) => (
                      <option key={n.code} value={n.code}>
                        {n.code} · {n.name}
                      </option>
                    ))}
                </ValidatedSelect>
              </ValidatedLabel>
              <p className="text-xs text-slate-500">
                Clearing an override restores inheritance. Every process must
                have a doer and a different resolver before activation.
              </p>
            </div>
          ) : null}
        </div>
        <ValidatedLabel className="block text-sm font-bold">
          Review / change reason
          <ValidatedTextarea name="reason"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            maxLength={2000}
            className={field}
          />
        </ValidatedLabel>
        <div className="flex flex-wrap gap-3">
          <button
            disabled={saving}
            onClick={() => void save(false)}
            className={processSecondary}
          >
            Save draft plan
          </button>
          <button
            disabled={saving}
            onClick={() => void save(true)}
            className={processButton}
          >
            Activate reviewed plan
          </button>
        </div>
        <p className="text-xs text-slate-500">
          Activation confirms this execution plan, not contractual, engineering,
          quality or safety approvals.
        </p>
      </fieldset>
    </section>
  );
}
