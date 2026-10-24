"use client";
import { displayServerErrors } from "@/components/ui/server-field-errors";
import { ValidatedForm, ValidatedInput, ValidatedLabel, ValidatedSelect, ValidatedTextarea } from "@/components/ui/validated-controls";
import { useState, type FormEvent } from "react";
import { DataGrid, GridAction, GridStatus } from "@/components/ui/data-grid";
import { RecordDrawer } from "@/components/ui/record-drawer";
import { ItemHistory } from "../items/item-history";
import {
  request,
  input,
  button,
  secondary,
  type RecordRow,
  type HoldData,
  type Overview,
} from "./purchase-data";
export function PurchaseHolds({
  project,
  overview,
  rows,
  reload,
}: {
  project: string;
  overview: Overview;
  rows: RecordRow<HoldData>[];
  reload: () => void;
}) {
  const [edit, setEdit] = useState<RecordRow<HoldData> | "new" | null>(null);
  return (
    <section className="space-y-3">
      <div className="flex justify-between gap-3">
        <h3 className="text-lg font-bold">Client purchase holds</h3>
        {overview.permissions["purchase_holds.write"] && (
          <button className={button} onClick={() => setEdit("new")}>
            Record client hold
          </button>
        )}
      </div>
      <DataGrid<RecordRow<HoldData>>
        label="Client purchase holds"
        rows={rows}
        columns={[
          {
            key: "reason",
            label: "Client instruction",
            value: (r) => r.data.reason,
          },
          {
            key: "scope",
            label: "Scope",
            value: (r) =>
              r.data.scope === "category"
                ? r.data.category_code
                : r.data.scope === "items"
                  ? `${r.data.demand_ids.length} items`
                  : "Whole project",
          },
          {
            key: "start",
            label: "Hold started",
            value: (r) => new Date(r.data.started_at).toLocaleString(),
          },
          {
            key: "expected",
            label: "Review / expected release",
            value: (r) =>
              r.data.expected_release_at
                ? new Date(r.data.expected_release_at).toLocaleString()
                : "Not specified",
          },
          {
            key: "doer",
            label: "Doer",
            value: (r) =>
              overview.members.find((m) => m.id === r.data.doer_id)?.label ??
              "Former team member",
          },
          {
            key: "resolver",
            label: "Resolver",
            value: (r) =>
              overview.members.find((m) => m.id === r.data.resolver_id)
                ?.label ?? "Former team member",
          },
          {
            key: "state",
            label: "State",
            value: (r) => (r.is_active ? "On hold" : "Released"),
            render: (r) => (
              <GridStatus value={r.is_active ? "On hold" : "Released"} />
            ),
          },
        ]}
        actions={(r) => (
          <GridAction
            label={`View hold ${r.data.reason}`}
            onClick={() => setEdit(r)}
          />
        )}
      />
      {edit && (
        <HoldEditor
          key={typeof edit === "string" ? "new" : edit.id}
          project={project}
          overview={overview}
          record={edit}
          done={() => {
            setEdit(null);
            reload();
          }}
          close={() => setEdit(null)}
        />
      )}
    </section>
  );
}
function HoldEditor({
  project,
  overview,
  record,
  close,
  done,
}: {
  project: string;
  overview: Overview;
  record: RecordRow<HoldData> | "new";
  close: () => void;
  done: () => void;
}) {
  const old = record === "new" ? null : record;
  const now = new Date();
  now.setMinutes(now.getMinutes() - now.getTimezoneOffset());
  const [scope, setScope] = useState("project"),
    [category, setCategory] = useState(""),
    [ids, setIds] = useState<string[]>([]),
    [reason, setReason] = useState(""),
    [evidence, setEvidence] = useState(""),
    [start, setStart] = useState(now.toISOString().slice(0, 16)),
    [expected, setExpected] = useState(""),
    [doer, setDoer] = useState(""),
    [resolver, setResolver] = useState(""),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  async function save(e: FormEvent) {
    const submittedForm = e.currentTarget instanceof HTMLFormElement ? e.currentTarget : undefined;
    e.preventDefault();
    setBusy(true);
    try {
      await request(
        `/api/projects/${project}/purchase-holds${old ? "/" + old.id + "/release" : ""}`,
        old
          ? { version: old.version, reason, evidence }
          : {
              scope,
              category_code: scope === "category" ? category : "",
              demand_ids: scope === "items" ? ids : [],
              reason,
              evidence,
              started_at: new Date(start).toISOString(),
              expected_release_at: expected
                ? new Date(expected).toISOString()
                : null,
              doer_id: doer,
              resolver_id: resolver,
            },
        old ? "PATCH" : "POST",
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
      title={old ? "Client hold detail" : "Record client purchase hold"}
      onClose={close}
    >
      <ValidatedForm className="space-y-4" onSubmit={save}>
        {error && (
          <p role="alert" className="text-red-700">
            {error}
          </p>
        )}
        {old && (
          <div className="rounded-xl bg-orange-50 p-4">
            <p className="font-bold">{old.data.reason}</p>
            <p className="mt-2 text-sm">Instruction: {old.data.evidence}</p>
            <p className="mt-2 text-sm">
              {old.is_active
                ? "Hold remains active until explicitly released."
                : `Released ${new Date(old.data.released_at!).toLocaleString()}`}
            </p>
          </div>
        )}
        <fieldset disabled={busy} className="space-y-4">
          {!old && (
            <>
              <ValidatedLabel className="block">
                Hold scope
                <ValidatedSelect name="scope"
                  aria-label="Hold scope"
                  className={input}
                  value={scope}
                  onChange={(e) => setScope(e.target.value)}
                >
                  <option value="project">Whole project</option>
                  <option value="category">Category in this project</option>
                  <option value="items">Selected project items</option>
                </ValidatedSelect>
              </ValidatedLabel>
              {scope === "category" && (
                <ValidatedLabel className="block">
                  Category
                  <ValidatedSelect name="category"
                    aria-label="Category"
                    required
                    className={input}
                    value={category}
                    onChange={(e) => setCategory(e.target.value)}
                  >
                    <option value="">Select category</option>
                    {overview.categories.map((c) => (
                      <option key={c.code} value={c.code}>
                        {c.name}
                      </option>
                    ))}
                  </ValidatedSelect>
                </ValidatedLabel>
              )}
              {scope === "items" && (
                <fieldset>
                  <legend>Project requirements</legend>
                  {overview.lines.map((l) => (
                    <ValidatedLabel key={l.id} className="flex min-h-11 gap-2 text-sm">
                      <ValidatedInput
                        type="checkbox"
                        checked={ids.includes(l.id)}
                        onChange={(e) =>
                          setIds(
                            e.target.checked
                              ? [...ids, l.id]
                              : ids.filter((id) => id !== l.id),
                          )
                        }
                      />
                      {l.item.description}
                    </ValidatedLabel>
                  ))}
                </fieldset>
              )}
              <ValidatedLabel className="block">
                Hold start
                <ValidatedInput name="start"
                  type="datetime-local"
                  required
                  className={input}
                  value={start}
                  onChange={(e) => setStart(e.target.value)}
                />
              </ValidatedLabel>
              <ValidatedLabel className="block">
                Expected release / review date (optional)
                <ValidatedInput name="expected"
                  type="datetime-local"
                  className={input}
                  value={expected}
                  onChange={(e) => setExpected(e.target.value)}
                />
              </ValidatedLabel>
              {[
                ["Doer", doer, setDoer],
                ["Problem resolver", resolver, setResolver],
              ].map(([label, value, set]) => (
                <ValidatedLabel className="block" key={label as string}>
                  {label as string}
                  <ValidatedSelect
                    aria-label={label as string}
                    required
                    className={input}
                    value={value as string}
                    onChange={(e) =>
                      (set as (v: string) => void)(e.target.value)
                    }
                  >
                    <option value="">Select project team member</option>
                    {overview.members.map((m) => (
                      <option key={m.id} value={m.id}>
                        {m.label}
                      </option>
                    ))}
                  </ValidatedSelect>
                </ValidatedLabel>
              ))}
            </>
          )}
          {(!old ||
            (old.is_active &&
              overview.permissions["purchase_holds.release"])) && (
            <>
              <ValidatedLabel className="block">
                {old ? "Release reason" : "Client hold reason"}
                <ValidatedTextarea name="reason"
                  required
                  className={input}
                  value={reason}
                  onChange={(e) => setReason(e.target.value)}
                />
              </ValidatedLabel>
              <ValidatedLabel className="block">
                {old
                  ? "Release instruction / evidence reference"
                  : "Client instruction / evidence reference"}
                <ValidatedTextarea name="evidence"
                  required
                  className={input}
                  value={evidence}
                  onChange={(e) => setEvidence(e.target.value)}
                />
              </ValidatedLabel>
              <button className={button}>
                {old ? "Release hold" : "Save client hold"}
              </button>
            </>
          )}
        </fieldset>
        {old && (
          <ItemHistory
            url={`/api/projects/${project}/purchase-holds/${old.id}/history`}
          />
        )}
        <button type="button" className={secondary} onClick={close}>
          Close
        </button>
      </ValidatedForm>
    </RecordDrawer>
  );
}
