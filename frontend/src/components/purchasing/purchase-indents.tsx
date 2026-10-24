"use client";
import {PlusIcon} from "@heroicons/react/24/outline";
import { ValidatedForm, ValidatedInput, ValidatedLabel, ValidatedSelect, ValidatedTextarea } from "@/components/ui/validated-controls";
import { useState, useRef, type FormEvent } from "react";
import { DataGrid, GridAction, GridStatus } from "@/components/ui/data-grid";
import { RecordDrawer } from "@/components/ui/record-drawer";
import {FieldValidationError,displayServerErrors} from "@/components/ui/server-field-errors";
import { ItemHistory } from "../items/item-history";
import {
  request,
  input,
  button,
  secondary,
  type RecordRow,
  type IndentData,
  type Line,
  type Overview,
} from "./purchase-data";
export type IndentOverview = Pick<Overview,"lines"|"members"|"permissions">;
export function PurchaseIndents({
  project,
  overview,
  rows,
  reload,
  currency = "INR",
}: {
  project: string;
  currency?: string;
  overview: IndentOverview;
  rows: RecordRow<IndentData>[];
  reload: () => void;
}) {
  const [edit, setEdit] = useState<{record:RecordRow<IndentData>|"new";editing:boolean}|null>(null);
  return (
    <section className="space-y-3">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <h3 className="text-base font-bold">Purchase indents</h3>
        {overview.permissions["purchase_indents.write"] && (
          <button
            className={`${button} inline-flex w-full items-center justify-center gap-2 sm:w-auto`}
            disabled={overview.lines.length === 0}
            onClick={() => setEdit({record:"new",editing:true})}
          >
            <PlusIcon aria-hidden="true" className="h-4 w-4"/>Create purchase indent
          </button>
        )}
      </div>
      <DataGrid<RecordRow<IndentData>>
        label="Purchase indents"
        rows={rows}
        columns={[
          { key: "number", label: "Indent", value: (r) => r.data.number },
          {
            key: "state",
            label: "State",
            value: (r) => r.data.state,
            render: (r) => (
              <GridStatus value={r.data.state.replaceAll("_", " ")} />
            ),
          },
          {
            key: "lines",
            label: "Requirements",
            value: (r) => r.data.lines.length,
          },
          {
            key: "released",
            label: "Released lines",
            value: (r) => r.data.released.length,
          },
          {
            key: "budget",
            label: "Total budget",
            value: (r) =>
              `${r.data.currency} ${totalBudget(r.data.lines)}`,
          },
          {
            key: "doer",
            label: "Procurement doer",
            value: (r) =>
              overview.members.find((m) => m.id === r.data.doer_id)?.label ??
              "Previous assignee",
          },
        ]}
        actions={(r) => (
          <><GridAction label={`View indent ${r.data.number}`} onClick={() => setEdit({record:r,editing:false})}/>{overview.permissions["purchase_indents.write"]&&r.data.state==="draft"&&<GridAction kind="edit" label={`Edit indent ${r.data.number}`} onClick={()=>setEdit({record:r,editing:true})}/>}</>
        )}
      />
      {edit && (
        <IndentEditor
          key={typeof edit.record === "string" ? "new" : `${edit.record.id}:${edit.editing}`}
          project={project}
          overview={overview}
          record={edit.record}
          editing={edit.editing}
          currency={currency}
          close={() => setEdit(null)}
          done={() => {
            setEdit(null);
            reload();
          }}
        />
      )}
    </section>
  );
}
function IndentEditor({
  project,
  overview,
  record,
  editing,
  currency: projectCurrency,
  close,
  done,
}: {
  project: string;
  overview: IndentOverview;
  record: RecordRow<IndentData> | "new";
  editing: boolean;
  currency: string;
  close: () => void;
  done: () => void;
}) {
  const old = record === "new" ? null : record;
  const formRef=useRef<HTMLFormElement>(null);
  const editable =
    editing && (!old || old.data.state === "draft") &&
    overview.permissions["purchase_indents.write"];
  const [lines, setLines] = useState<Line[]>(old?.data.lines ?? []),
    [currency, setCurrency] = useState(old?.data.currency ?? projectCurrency),
    [doer, setDoer] = useState(old?.data.doer_id ?? ""),
    [resolver, setResolver] = useState(old?.data.resolver_id ?? ""),
    [documents, setDocuments] = useState(old?.data.documents ?? ""),
    [reason, setReason] = useState(""),
    [evidence, setEvidence] = useState(""),
    [selected, setSelected] = useState<string[]>([]),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false);
  async function save(e: FormEvent) {
    e.preventDefault();
    if(!editable||busy)return;
    if (doer === resolver) {
      setError("Procurement doer and problem resolver must be different people.");
      return;
    }
    await run(() =>
      request(
        `/api/projects/${project}/purchase-indents${old ? "/" + old.id : ""}`,
        {
          version: old?.version ?? 1,
          reason,
          data: {
            currency,
            doer_id: doer,
            resolver_id: resolver,
            documents,
            lines,
          },
        },
        old ? "PATCH" : "POST",
      ),
    );
  }
  async function run(fn: () => Promise<unknown>) {
    setBusy(true);
    setError("");
    try {
      await fn();
      done();
    } catch (e) {
      if(!(e instanceof FieldValidationError&&displayServerErrors(formRef.current??undefined,e)))setError(e instanceof Error?e.message:"The indent could not be saved. Reload before retrying.");
    } finally {
      setBusy(false);
    }
  }
  function action(action: string) {
    void run(() =>
      request(`/api/projects/${project}/purchase-indents/${old!.id}/actions`, {
        version: old!.version,
        action,
        line_ids: action === "release" ? selected : [],
        reason,
        evidence,
      }),
    );
  }
  const actions = old && !editing
    ? [
        ...(old.data.state === "draft" &&
        overview.permissions["purchase_indents.write"]
          ? ["submit"]
          : []),
        ...(old.data.state === "submitted" &&
        overview.permissions["purchase_indents.review"]
          ? ["review", "return"]
          : []),
        ...(old.data.state === "reviewed" &&
        overview.permissions["purchase_indents.review"]
          ? ["return"]
          : []),
        ...(["reviewed", "part_released"].includes(old.data.state) &&
        overview.permissions["purchase_indents.release"]
          ? ["release"]
          : []),
        ...(!["released", "withdrawn"].includes(old.data.state) &&
        overview.permissions["purchase_indents.write"]
          ? ["withdraw"]
          : []),
      ]
    : [];
  return (
    <RecordDrawer
      title={
        old
          ? `${old.data.number} · ${old.data.state.replaceAll("_", " ")}`
          : "Create purchase indent"
      }
      onClose={close}
    >
      <ValidatedForm ref={formRef} onSubmit={save} className="space-y-4">
        {error && (
          <p role="alert" className="text-red-700">
            {error}
          </p>
        )}
        <fieldset disabled={!editable || busy} className="space-y-4">
          <ValidatedLabel className="block">
            Currency
            <ValidatedInput name="currency" aria-label="Currency"
              required
              pattern="[A-Z]{3}"
              className={input}
              value={currency}
              onChange={(e) => setCurrency(e.target.value.toUpperCase())}
            />
          </ValidatedLabel>
          {[
            ["Procurement doer", doer, setDoer],
            ["Problem resolver", resolver, setResolver],
          ].map(([label, value, set]) => (
            <ValidatedLabel className="block" key={label as string}>
              {label as string}
              <ValidatedSelect
                aria-label={label as string}
                name={label === "Procurement doer" ? "doer_id" : "resolver_id"}
                className={input}
                required
                value={value as string}
                onChange={(e) => (set as (v: string) => void)(e.target.value)}
              >
                <option value="">Select active user</option>
                {overview.members.map((m) => (
                  <option key={m.id} value={m.id} disabled={label === "Problem resolver" && m.id === doer}>
                    {m.label}
                  </option>
                ))}
              </ValidatedSelect>
            </ValidatedLabel>
          ))}
          <ValidatedLabel className="block">
            Technical requirements / supporting document references
            <ValidatedTextarea name="documents"
              required
              className={input}
              value={documents}
              onChange={(e) => setDocuments(e.target.value)}
            />
          </ValidatedLabel>
          {editable && (
            <ValidatedLabel className="block">
              Add project requirement
              <ValidatedSelect
                aria-label="Add project requirement"
                className={input}
                value=""
                onChange={(e) => {
                  const source = overview.lines.find(
                    (l) => l.id === e.target.value,
                  );
                  if (source)
                    setLines([
                      ...lines,
                      {
                        demand_id: source.id,
                        quantity: source.remaining_quantity ?? source.quantity,
                        budget: "",
                        full_kit: "",
                      },
                    ]);
                }}
              >
                <option value="">Choose requirement</option>
                {overview.lines
                  .filter(
                    (l) =>
                      l.active &&
                      Number(l.remaining_quantity ?? l.quantity) > 0 &&
                      !lines.some((x) => x.demand_id === l.id),
                  )
                  .map((l) => (
                    <option key={l.id} value={l.id}>
                      {l.item.description}
                    </option>
                  ))}
              </ValidatedSelect>
            </ValidatedLabel>
          )}
        </fieldset>
        <div className="space-y-3">
          {lines.map((line, i) => {
            const source = overview.lines.find((l) => l.id === line.demand_id);
            const released = old?.data.released.some(
              (l) => l.demand_id === line.demand_id,
            );
            const patch = (field: keyof Line, value: string) =>
              setLines(
                lines.map((l, j) => (i === j ? { ...l, [field]: value } : l)),
              );
            return (
              <fieldset
                key={line.demand_id}
                className="rounded-xl border border-slate-200 p-4 space-y-3"
              >
                <legend className="text-sm font-bold">
                  {source?.item.description ?? line.demand_id}
                </legend>
                <p className="text-xs text-slate-600">
                  Required {source?.required_date} · {source?.item.uom} · Make:{" "}
                  {source?.policy.status ?? "Unavailable"} ·{" "}
                  {source?.holds.holds.length
                    ? "CLIENT HOLD"
                    : "No active client hold"}
                </p>
                <div className="grid grid-cols-2 gap-3">
                  <ValidatedLabel>
                    Quantity
                    <ValidatedInput
                      required
                      type="number"
                      name={`data.lines.${i}.quantity`} aria-label="Quantity"
                      min="0.001"
                      step="0.001"
                      disabled={!editable || busy}
                      className={input}
                      value={line.quantity}
                      onChange={(e) => patch("quantity", e.target.value)}
                    />
                  </ValidatedLabel>
                  <ValidatedLabel>
                    Total line budget
                    <ValidatedInput
                      required
                      type="number"
                      name={`data.lines.${i}.budget`} aria-label="Total line budget"
                      min="0"
                      step="0.01"
                      disabled={!editable || busy}
                      className={input}
                      value={line.budget}
                      onChange={(e) => patch("budget", e.target.value)}
                    />
                  </ValidatedLabel>
                </div>
                <ValidatedLabel className="block">
                  Full-kit / configuration details
                  <ValidatedTextarea
                    aria-label="Full-kit / configuration details" name={`data.lines.${i}.full_kit`}
                    required
                    disabled={!editable || busy}
                    className={input}
                    value={line.full_kit}
                    onChange={(e) => patch("full_kit", e.target.value)}
                  />
                </ValidatedLabel>
                {editable && (
                  <button
                    type="button"
                    className="text-sm text-red-700"
                    onClick={() => setLines(lines.filter((_, j) => j !== i))}
                  >
                    Remove line
                  </button>
                )}
                {actions.includes("release") && (
                  <ValidatedLabel className="flex min-h-11 items-center gap-2">
                    <ValidatedInput
                      type="checkbox"
                      disabled={released || busy}
                      checked={selected.includes(line.demand_id)}
                      onChange={(e) =>
                        setSelected(
                          e.target.checked
                            ? [...selected, line.demand_id]
                            : selected.filter((id) => id !== line.demand_id),
                        )
                      }
                    />
                    {released
                      ? "Already released"
                      : "Select for release (server rechecks all controls)"}
                  </ValidatedLabel>
                )}
              </fieldset>
            );
          })}
        </div>
        {(editable || actions.length > 0) && (
          <ValidatedLabel className="block">
            Change / decision reason
            <ValidatedInput name="reason" aria-label="Change / decision reason"
              required
              className={input}
              value={reason}
              onChange={(e) => setReason(e.target.value)}
            />
          </ValidatedLabel>
        )}
        {editable && (
          <button disabled={busy || !lines.length} className={button}>
            Save draft
          </button>
        )}
        {actions.length > 0 && (
          <>
            <ValidatedLabel className="block">
              Review / decision evidence reference
              <ValidatedTextarea name="evidence" aria-label="Review / decision evidence reference"
                className={input}
                value={evidence}
                onChange={(e) => setEvidence(e.target.value)}
              />
            </ValidatedLabel>
            <p className="text-xs text-slate-500">
              Review records this revision’s quantities, budget details and technical
              readiness. Release issues only selected lines; remaining lines
              stay unreleased. This does not approve a project budget or authorise a purchase order.
            </p>
            <div className="flex flex-wrap gap-2">
              {actions.map((a) => (
                <button
                  type="button"
                  key={a}
                  disabled={
                    busy ||
                    !reason.trim() ||
                    !evidence.trim() ||
                    (a === "release" && !selected.length)
                  }
                  className={secondary}
                  onClick={() => action(a)}
                >
                  {
                    (
                      {
                        submit: "Submit for review",
                        review: "Confirm demand & budget",
                        return: "Return for correction",
                        release: "Release selected lines",
                        withdraw: "Withdraw remaining lines",
                      } as Record<string, string>
                    )[a]
                  }
                </button>
              ))}
            </div>
          </>
        )}
        {old && (
          <ItemHistory
            url={`/api/projects/${project}/purchase-indents/${old.id}/history`}
          />
        )}
        <button type="button" className={secondary} onClick={close}>
          Close
        </button>
      </ValidatedForm>
    </RecordDrawer>
  );
}

function totalBudget(lines:Line[]){const cents=lines.reduce((sum,line)=>{const [whole,fraction=""]=line.budget.split(".");return sum+BigInt(whole||"0")*BigInt(100)+BigInt(fraction.padEnd(2,"0"))},BigInt(0));return `${(cents/BigInt(100)).toLocaleString("en-IN")}.${(cents%BigInt(100)).toString().padStart(2,"0")}`}
