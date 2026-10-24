"use client";
import { displayServerErrors } from "@/components/ui/server-field-errors";
import { ValidatedForm, ValidatedInput, ValidatedLabel, ValidatedSelect, ValidatedTextarea } from "@/components/ui/validated-controls";
import { useEffect, useState, type FormEvent } from "react";
import { useAuthenticatedUser } from "@/components/auth-user-provider";
import { DataGrid, GridAction, GridStatus } from "@/components/ui/data-grid";
import { RecordDrawer } from "@/components/ui/record-drawer";
import { ItemHistory } from "../items/item-history";
import { type Definition, type Manufacturer } from "../items/item-data";
import {
  request,
  input,
  button,
  secondary,
  type RecordRow,
  type RuleData,
} from "./purchase-data";
export function ClientPurchaseRules() {
  const user = useAuthenticatedUser(),
    write =
      user.permissions.includes("system:admin") ||
      user.permissions.includes("settings.purchase_rules.write");
  const [refs, setRefs] = useState<{
      clients: { id: string; name: string }[];
      catalogue: Definition[];
      projects: { id: string; name: string; client_id: string }[];
      manufacturers: Manufacturer[];
    }>({ clients: [], catalogue: [], projects: [], manufacturers: [] }),
    [client, setClient] = useState(""),
    [rows, setRows] = useState<RecordRow<RuleData>[]>([]),
    [edit, setEdit] = useState<RecordRow<RuleData> | "new" | null>(null),
    [error, setError] = useState(""),
    [reload, setReload] = useState(0),
    [loading, setLoading] = useState(false);
  useEffect(() => {
    let live = true;
    request<typeof refs>("/api/settings/purchase-rule-references")
      .then((r) => {
        if (live) setRefs(r);
      })
      .catch((e) => {
        if (live) setError(e.message);
      });
    return () => {
      live = false;
    };
  }, []);
  useEffect(() => {
    if (!client) return;
    let live = true;
    request<RecordRow<RuleData>[]>(
      `/api/settings/clients/${client}/purchase-rules`,
    )
      .then((r) => {
        if (live) {
          setRows(r);
          setError("");
        }
      })
      .catch((e) => {
        if (live) setError(e.message);
      })
      .finally(() => {
        if (live) setLoading(false);
      });
    return () => {
      live = false;
    };
  }, [client, reload]);
  return (
    <section className="space-y-5">
      <header>
        <h2 className="text-2xl font-bold">Client purchasing rules</h2>
        <p className="mt-2 text-sm text-slate-600">
          Manufacturer requirements inherited by client projects. Exclusions and
          approved-only lists are checked together. Unreviewed items cannot be
          released for purchase.
        </p>
      </header>
      <div className="flex flex-wrap items-end gap-4">
        <ValidatedLabel className="min-w-64">
          Client
          <ValidatedSelect name="client"
            aria-label="Client"
            className={input}
            value={client}
            onChange={(e) => {
              setLoading(Boolean(e.target.value));
              setClient(e.target.value);
              setRows([]);
              setEdit(null);
            }}
          >
            <option value="">Select client</option>
            {refs.clients.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </ValidatedSelect>
        </ValidatedLabel>
        {write && client && (
          <button className={button} onClick={() => setEdit("new")}>
            Add manufacturer rule
          </button>
        )}
      </div>
      {error && (
        <p role="alert" className="text-red-700">
          {error}
        </p>
      )}
      {client && (
        <DataGrid<RecordRow<RuleData>>
          label="Client manufacturer rules"
          rows={rows}
          loading={loading}
          columns={[
            {
              key: "scope",
              label: "Category / configuration",
              value: (r) => r.data.scope_code,
            },
            {
              key: "mode",
              label: "Policy",
              value: (r) => r.data.mode.replaceAll("_", " "),
            },
            {
              key: "allowed",
              label: "Allowed manufacturers",
              value: (r) =>
                r.data.allowed
                  .map(
                    (id) =>
                      refs.manufacturers.find((m) => m.id === id)?.name ?? id,
                  )
                  .join(", ") || "No whitelist",
            },
            {
              key: "excluded",
              label: "Excluded manufacturers",
              value: (r) =>
                r.data.excluded
                  .map(
                    (id) =>
                      refs.manufacturers.find((m) => m.id === id)?.name ?? id,
                  )
                  .join(", ") || "None",
            },
            {
              key: "project",
              label: "Project scope",
              value: (r) =>
                r.data.project_id
                  ? (refs.projects.find((p) => p.id === r.data.project_id)
                      ?.name ?? "Project unavailable")
                  : "All client projects",
            },
            {
              key: "active",
              label: "Status",
              value: (r) => (r.is_active ? "Active" : "Inactive"),
              render: (r) => (
                <GridStatus value={r.is_active ? "Active" : "Inactive"} />
              ),
            },
          ]}
          actions={(r) => (
            <GridAction
              label={`View rule ${r.data.scope_code}`}
              onClick={() => setEdit(r)}
            />
          )}
        />
      )}
      {edit && (
        <RuleEditor
          key={typeof edit === "string" ? "new" : edit.id}
          client={client}
          record={edit}
          refs={refs}
          write={write}
          close={() => setEdit(null)}
          done={() => {
            setEdit(null);
            setLoading(true);
            setReload((n) => n + 1);
          }}
        />
      )}
    </section>
  );
}
function RuleEditor({
  client,
  record,
  refs,
  write,
  close,
  done,
}: {
  client: string;
  record: RecordRow<RuleData> | "new";
  refs: {
    catalogue: Definition[];
    projects: { id: string; name: string; client_id: string }[];
    manufacturers: Manufacturer[];
  };
  write: boolean;
  close: () => void;
  done: () => void;
}) {
  const original = record === "new" ? null : record;
  const [data, setData] = useState<RuleData>(
      original?.data ?? {
        project_id: null,
        scope_code: "*",
        mode: "approved_only",
        allowed: [],
        excluded: [],
        effective_from: new Date().toISOString().slice(0, 10),
        effective_until: null,
        evidence: "",
      },
    ),
    [active, setActive] = useState(original?.is_active ?? true),
    [reason, setReason] = useState(""),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false);
  async function save(e: FormEvent) {
    const submittedForm = e.currentTarget instanceof HTMLFormElement ? e.currentTarget : undefined;
    e.preventDefault();
    setBusy(true);
    try {
      await request(
        `/api/settings/clients/${client}/purchase-rules${original ? "/" + original.id : ""}`,
        { data, is_active: active, version: original?.version ?? 1, reason },
        original ? "PATCH" : "POST",
      );
      done();
    } catch (e) {
      if (displayServerErrors(submittedForm, e)) return;
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <RecordDrawer
      title={original ? "Manufacturer policy" : "Add manufacturer rule"}
      onClose={close}
    >
      <ValidatedForm onSubmit={save} className="space-y-4">
        {error && (
          <p role="alert" className="text-red-700">
            {error}
          </p>
        )}
        <fieldset disabled={!write || busy} className="space-y-4">
          <ValidatedLabel className="block">
            Category / configuration
            <ValidatedSelect name="scope_code"
              aria-label="Category / configuration"
              className={input}
              value={data.scope_code}
              onChange={(e) => setData({ ...data, scope_code: e.target.value })}
            >
              <option value="*">All categories (client default)</option>
              {refs.catalogue
                .filter((r) => r.is_active)
                .map((r) => (
                  <option key={r.id} value={r.code}>
                    {r.name} ({r.code})
                  </option>
                ))}
            </ValidatedSelect>
          </ValidatedLabel>
          <ValidatedLabel className="block">
            Project scope
            <ValidatedSelect
              aria-label="Project scope"
              className={input}
              value={data.project_id ?? ""}
              onChange={(e) =>
                setData({ ...data, project_id: e.target.value || null })
              }
            >
              <option value="">All client projects</option>
              {refs.projects
                .filter((p) => p.client_id === client)
                .map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))}
            </ValidatedSelect>
          </ValidatedLabel>
          <ValidatedLabel className="block">
            Policy
            <ValidatedSelect name="mode"
              aria-label="Policy"
              className={input}
              value={data.mode}
              onChange={(e) => setData({ ...data, mode: e.target.value })}
            >
              <option value="approved_only">Only listed manufacturers</option>
              <option value="unrestricted">
                Any eligible manufacturer except exclusions
              </option>
            </ValidatedSelect>
          </ValidatedLabel>
          <div className="grid grid-cols-2 gap-4">
            {(["allowed", "excluded"] as const).map((kind) => (
              <fieldset key={kind}>
                <legend className="font-semibold capitalize">{kind}</legend>
                <div className="max-h-60 overflow-auto">
                  {refs.manufacturers
                    .filter((m) => m.is_active)
                    .map((m) => (
                      <ValidatedLabel
                        key={m.id}
                        className="flex min-h-10 items-center gap-2 text-sm"
                      >
                        <ValidatedInput
                          type="checkbox"
                          checked={data[kind].includes(m.id)}
                          onChange={(e) =>
                            setData({
                              ...data,
                              [kind]: e.target.checked
                                ? [...data[kind], m.id]
                                : data[kind].filter((id) => id !== m.id),
                            })
                          }
                        />
                        {m.name}
                      </ValidatedLabel>
                    ))}
                </div>
              </fieldset>
            ))}
          </div>
          <div className="grid grid-cols-2 gap-3">
            <ValidatedLabel>
              Effective from
              <ValidatedInput name="effective_from"
                type="date"
                required
                className={input}
                value={data.effective_from}
                onChange={(e) =>
                  setData({ ...data, effective_from: e.target.value })
                }
              />
            </ValidatedLabel>
            <ValidatedLabel>
              Effective until
              <ValidatedInput
                type="date"
                className={input}
                value={data.effective_until ?? ""}
                onChange={(e) =>
                  setData({ ...data, effective_until: e.target.value || null })
                }
              />
            </ValidatedLabel>
          </div>
          <ValidatedLabel className="block">
            Client instruction / evidence reference
            <ValidatedTextarea name="evidence"
              required
              className={input}
              value={data.evidence}
              onChange={(e) => setData({ ...data, evidence: e.target.value })}
            />
          </ValidatedLabel>
          <ValidatedLabel className="flex items-center gap-2">
            <ValidatedInput
              type="checkbox"
              checked={active}
              onChange={(e) => setActive(e.target.checked)}
            />
            Active
          </ValidatedLabel>
          <ValidatedLabel className="block">
            Change reason
            <ValidatedInput name="reason"
              required
              className={input}
              value={reason}
              onChange={(e) => setReason(e.target.value)}
            />
          </ValidatedLabel>
          {write && <button className={button}>Save policy</button>}
        </fieldset>
        {original && (
          <ItemHistory
            url={`/api/settings/clients/${client}/purchase-rules/${original.id}/history`}
          />
        )}
        <button type="button" className={secondary} onClick={close}>
          Close
        </button>
      </ValidatedForm>
    </RecordDrawer>
  );
}
