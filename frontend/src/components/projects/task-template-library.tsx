"use client";
import { ValidatedInput, ValidatedLabel, ValidatedSelect, ValidatedTextarea } from "@/components/ui/validated-controls";
import { useEffect, useState } from "react";
import { useAuthenticatedUser } from "@/components/auth-user-provider";
import {
  processRequest,
  processButton,
  processSecondary,
} from "./project-process-data";
import {
  TaskTemplate,
  TemplateNode,
  outline,
  depth,
  siblingTotal,
  deliveryField as field,
} from "./delivery-plan-data";

export function TaskTemplateLibrary() {
  const user = useAuthenticatedUser();
  const canWrite =
    user.permissions.includes("system:admin") ||
    user.permissions.includes("settings.task_templates.write");
  const [templates, setTemplates] = useState<TaskTemplate[]>([]);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [draft, setDraft] = useState<TaskTemplate | null>(null);
  const [selected, setSelected] = useState("");
  const [saving, setSaving] = useState(false);
  useEffect(() => {
    void processRequest<TaskTemplate[]>("/api/settings/task-templates")
      .then(setTemplates)
      .catch((e) => setError(e.message));
  }, []);
  function edit(t: TaskTemplate) {
    setDraft(structuredClone(t));
    setSelected(t.nodes[0].code);
    setError("");
    setNotice(
      "Editing a local copy. Publish creates a new immutable revision.",
    );
  }
  function update(values: Partial<TemplateNode>) {
    setDraft((d) =>
      d
        ? {
            ...d,
            nodes: d.nodes.map((n) =>
              n.code === selected ? { ...n, ...values } : n,
            ),
          }
        : d,
    );
  }
  async function publish(separate = false) {
    if (!draft) return;
    setSaving(true);
    setError("");
    try {
      const t = await processRequest<TaskTemplate>(
        "/api/settings/task-templates",
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            name: draft.name,
            description: draft.description,
            based_on_id: separate ? null : draft.id,
            nodes: draft.nodes,
          }),
        },
      );
      setTemplates((v) => [t, ...v]);
      setDraft(null);
      setNotice(
        `Published revision ${t.revision}. Existing projects retain their copied plans.`,
      );
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setSaving(false);
    }
  }
  async function availability(t: TaskTemplate) {
    setSaving(true);
    try {
      const next = await processRequest<TaskTemplate>(
        `/api/settings/task-templates/${t.id}`,
        {
          method: "PATCH",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            is_active: !t.is_active,
            reason: t.is_active
              ? "Retired from future project selection"
              : "Restored for future project selection",
          }),
        },
      );
      setTemplates((v) => v.map((t) => (t.id === next.id ? next : t)));
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setSaving(false);
    }
  }
  const node = draft?.nodes.find((n) => n.code === selected);
  return (
    <section className="space-y-4">
      <header className="rounded-2xl border border-slate-200 bg-white p-5">
        <h3 className="text-xl font-extrabold">Standard task templates</h3>
        <p className="mt-2 text-sm text-slate-600">
          Reusable process → task → subtask lists. Weights total 100% within
          every parent. Date windows are separate percentages of the parent date
          span. People are assigned only in projects.
        </p>
        <p className="mt-2 text-xs text-amber-800">
          The EPC starter is a suggested planning reference. Review its scope,
          weights, timing and approval evidence for your project.
        </p>
      </header>
      {error ? (
        <p role="alert" className="rounded-xl bg-red-50 p-4 text-red-800">
          {error}
        </p>
      ) : null}
      {notice ? (
        <p role="status" className="text-sm text-blue-800">
          {notice}
        </p>
      ) : null}
      {draft && node ? (
        <div className="space-y-4 rounded-2xl border border-slate-200 bg-white p-5">
          <div className="flex flex-wrap items-end gap-3">
            <ValidatedLabel className="min-w-0 flex-1 text-sm font-bold">
              Template name
              <ValidatedInput
                value={draft.name}
                onChange={(e) => setDraft({ ...draft, name: e.target.value })}
                className={field}
              />
            </ValidatedLabel>
            <button
              disabled={saving}
              onClick={() => void publish()}
              className={processButton}
            >
              Publish new revision
            </button>
            <button
              disabled={saving}
              onClick={() => void publish(true)}
              className={processSecondary}
            >
              Publish separate template
            </button>
            <button onClick={() => setDraft(null)} className={processSecondary}>
              Discard local changes
            </button>
          </div>
          <ValidatedLabel className="block text-sm font-bold">
            Scope and usage notes
            <ValidatedTextarea
              value={draft.description}
              maxLength={4000}
              onChange={(e) =>
                setDraft({ ...draft, description: e.target.value })
              }
              className={field}
            />
          </ValidatedLabel>
          <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_minmax(300px,1fr)]">
            <div className="max-h-[650px] overflow-auto rounded-xl border border-slate-200">
              {outline(draft.nodes).map((n) => (
                <button
                  key={n.code}
                  onClick={() => setSelected(n.code)}
                  className={`block min-h-12 w-full border-b border-slate-100 py-2 pr-3 text-left text-sm ${n.code === selected ? "bg-blue-50 text-blue-800" : "hover:bg-slate-50"}`}
                  style={{ paddingLeft: 12 + depth(n.code, draft.nodes) * 16 }}
                >
                  <span className="font-semibold">
                    {n.code} · {n.name}
                  </span>
                  <span className="block text-xs text-slate-500">
                    {n.weight_bp / 100}% of parent · siblings total{" "}
                    {siblingTotal(n, draft.nodes)}%
                  </span>
                </button>
              ))}
            </div>
            <div className="space-y-4">
              <h4 className="font-bold">{node.code} · Task definition</h4>
              <ValidatedLabel className="block text-sm font-bold">
                Name
                <ValidatedInput
                  value={node.name}
                  maxLength={255}
                  onChange={(e) => update({ name: e.target.value })}
                  className={field}
                />
              </ValidatedLabel>
              <div className="grid grid-cols-2 gap-3">
                <ValidatedLabel className="text-sm font-bold">
                  Weight in parent (%)
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
                    placeholder="Inherit"
                    value={
                      node.resolution_minutes === null
                        ? ""
                        : node.resolution_minutes / 60
                    }
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
                  Start in parent span (%)
                  <ValidatedInput
                    type="number"
                    min="0"
                    max="100"
                    step="0.01"
                    value={node.start_bp / 100}
                    onChange={(e) =>
                      update({
                        start_bp: Math.round(Number(e.target.value) * 100),
                      })
                    }
                    className={field}
                  />
                </ValidatedLabel>
                <ValidatedLabel className="text-sm font-bold">
                  Finish in parent span (%)
                  <ValidatedInput
                    type="number"
                    min="0"
                    max="100"
                    step="0.01"
                    value={node.finish_bp / 100}
                    onChange={(e) =>
                      update({
                        finish_bp: Math.round(Number(e.target.value) * 100),
                      })
                    }
                    className={field}
                  />
                </ValidatedLabel>
              </div>
              <p
                className={`text-sm ${siblingTotal(node, draft.nodes) === 100 ? "text-emerald-700" : "text-red-700"}`}
              >
                Sibling allocation: {siblingTotal(node, draft.nodes)} / 100%
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
                  {draft.nodes
                    .filter((n) => n.code !== node.code)
                    .map((n) => (
                      <option key={n.code} value={n.code}>
                        {n.code} · {n.name}
                      </option>
                    ))}
                </ValidatedSelect>
              </ValidatedLabel>
              <p className="text-xs text-slate-500">
                Parent prerequisites apply to all descendants. A prerequisite
                summary requires every outcome beneath it.
              </p>
              <div className="flex flex-wrap gap-2">
                <button
                  className={processSecondary}
                  onClick={() => {
                    const code = `${node.code}.${Date.now().toString().slice(-6)}`;
                    setDraft({
                      ...draft,
                      nodes: [
                        ...draft.nodes,
                        {
                          ...node,
                          code,
                          parent_code: node.code,
                          name: "New subtask",
                          weight_bp: 10000,
                          start_bp: 0,
                          finish_bp: 10000,
                          resolution_minutes: null,
                          predecessors: [],
                        },
                      ],
                    });
                    setSelected(code);
                  }}
                >
                  Add subtask
                </button>
                {node.parent_code ? (
                  <button
                    className={processSecondary}
                    onClick={() => {
                      const remove = new Set([node.code]);
                      let changed = true;
                      while (changed) {
                        changed = false;
                        for (const n of draft.nodes)
                          if (
                            n.parent_code &&
                            remove.has(n.parent_code) &&
                            !remove.has(n.code)
                          ) {
                            remove.add(n.code);
                            changed = true;
                          }
                      }
                      setDraft({
                        ...draft,
                        nodes: draft.nodes
                          .filter((n) => !remove.has(n.code))
                          .map((n) => ({
                            ...n,
                            predecessors: n.predecessors.filter(
                              (p) => !remove.has(p),
                            ),
                          })),
                      });
                      setSelected(node.parent_code!);
                    }}
                  >
                    Remove from draft
                  </button>
                ) : null}
              </div>
              <p className="text-xs text-slate-500">
                After adding or removing subtasks, rebalance sibling weights
                before publishing.
              </p>
            </div>
          </div>
        </div>
      ) : (
        <div className="grid gap-3 lg:grid-cols-2">
          {templates.map((t) => (
            <article
              key={t.id}
              className="rounded-2xl border border-slate-200 bg-white p-5"
            >
              <h4 className="font-bold">{t.name}</h4>
              <p className="mt-1 text-sm text-slate-500">
                Revision {t.revision} · {t.is_active ? "Available" : "Inactive"}{" "}
                · {t.nodes.length} nodes
              </p>
              <p className="mt-3 text-sm text-slate-600">{t.description}</p>
              <div className="mt-4 flex flex-wrap gap-2">
                {canWrite ? (
                  <>
                    <button className={processButton} onClick={() => edit(t)}>
                      Edit a new revision
                    </button>
                    <button
                      disabled={saving}
                      className={processSecondary}
                      onClick={() => void availability(t)}
                    >
                      {t.is_active
                        ? "Deactivate revision"
                        : "Reactivate revision"}
                    </button>
                  </>
                ) : (
                  <details>
                    <summary className="min-h-11 cursor-pointer text-sm font-bold text-blue-700">
                      View task list
                    </summary>
                    <ol className="max-h-96 overflow-y-auto text-sm">
                      {outline(t.nodes).map((n) => (
                        <li key={n.code} className="py-1">
                          {n.code} · {n.name} · {n.weight_bp / 100}%
                        </li>
                      ))}
                    </ol>
                  </details>
                )}
              </div>
            </article>
          ))}
        </div>
      )}
    </section>
  );
}
