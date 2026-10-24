"use client";
import { FieldValidationError, displayServerErrors } from "@/components/ui/server-field-errors";
import { RequiredSelection, ValidatedForm, ValidatedInput, ValidatedLabel, ValidatedTextarea } from "@/components/ui/validated-controls";
import { useEffect, useState, type SubmitEvent } from "react";
import { useAuthenticatedUser } from "@/components/auth-user-provider";
import {
  type Manufacturer,
  request,
  input,
  button,
  secondary,
  panel,
} from "./item-data";
import { DataGrid, GridAction, GridStatus } from "@/components/ui/data-grid";
import { RecordDrawer } from "@/components/ui/record-drawer";
import { PlusIcon } from "@heroicons/react/24/outline";
import { ManufacturerEligibility, type EligibilityChoice } from "./manufacturer-eligibility";
import { ItemHistory } from "./item-history";
export function ManufacturerBank() {
  const user = useAuthenticatedUser();
  const write =
    user.permissions.includes("system:admin") ||
    user.permissions.includes("settings.manufacturers.write");
  const [records, setRecords] = useState<Manufacturer[]>([]);
  const [catalogue, setCatalogue] = useState<EligibilityChoice[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [reload, setReload] = useState(0);
  const [editing, setEditing] = useState<Manufacturer | null | undefined>();
  const [viewId, setViewId] = useState<string | null>(null);
  useEffect(() => {
    let live = true;
    Promise.all([
      request<Manufacturer[]>("/api/settings/manufacturers"),
      request<{rows:EligibilityChoice[]}>("/api/settings/manufacturers/eligibility"),
    ])
      .then(([r, c]) => {
        if (live) {
          setRecords(r);
          setCatalogue(c.rows);
          setLoading(false);
        }
      })
      .catch((e) => {
        if (live) {
          setError(e.message);
          setLoading(false);
        }
      });
    return () => {
      live = false;
    };
  }, [reload]);
  return (
    <section className="space-y-5">
      <header className="flex flex-col gap-4 border-b border-slate-200 pb-5 sm:flex-row sm:items-end sm:justify-between"><div><h1 className="text-2xl font-bold">Manufacturers</h1><p className="mt-1 text-sm">Manage manufacturers and the categories or subcategories they make.</p></div>{write&&<button type="button" className="inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-blue-700 px-4 text-sm font-bold text-white" onClick={()=>setEditing(null)}><PlusIcon className="h-5 w-5"/>Add manufacturer</button>}</header>
      {error && (
        <p role="alert" className="text-red-700">
          {error}
        </p>
      )}
      {loading && <p>Loading manufacturers…</p>}
      {editing !== undefined && (
        <RecordDrawer
          title={editing ? "Edit manufacturer" : "Add manufacturer"}
          onClose={() => setEditing(undefined)}
        >
          <ManufacturerEditor
            key={editing?.id ?? "new"}
            source={editing ?? undefined}
            catalogue={catalogue}
            done={() => {
              setEditing(undefined);
              setReload((r) => r + 1);
            }}
            close={() => setEditing(undefined)}
          />
        </RecordDrawer>
      )}
      <DataGrid<Manufacturer>
        label="Manufacturers"
        rows={records}
        loading={loading}
        initialSort="name"
        status={(r) => (r.is_active ? "Active" : "Inactive")}
        columns={[
          {
            key: "code",
            label: "Code",
            value: (r) => r.code,
            className: "font-mono text-xs",
          },
          {
            key: "name",
            label: "Name",
            value: (r) => r.name,
            className: "font-semibold text-slate-800",
          },
          {
            key: "eligible",
            label: "Eligible subcategories",
            value: (r) =>
              r.data.eligible
                .map(
                  (code) =>
                    catalogue.find((c) => c.code === code)?.name ?? code,
                )
                .join(", "),
            className: "max-w-lg",
          },
          {
            key: "status",
            label: "Status",
            value: (r) => (r.is_active ? "Active" : "Inactive"),
            render: (r) => (
              <GridStatus value={r.is_active ? "Active" : "Inactive"} />
            ),
          },
        ]}
        actions={(r) => (
          <>
            <GridAction
              label={`View ${r.name}`}
              onClick={() => setViewId(r.id)}
            />
            {write && (
              <GridAction
                kind="edit"
                label={`Edit ${r.name}`}
                onClick={() => setEditing(r)}
              />
            )}
          </>
        )}
      />
      {viewId && (
        <RecordDrawer
          title="Manufacturer details"
          onClose={() => setViewId(null)}
        >
          {" "}
          <div className="space-y-4">
            {records
              .filter((r) => r.id === viewId)
              .map((r) => (
                <article className={panel} key={r.id}>
                  <h3 className="text-lg font-bold">
                    {r.name}{" "}
                    <span className="text-sm text-slate-500">
                      {r.code} · {r.is_active ? "Active" : "Inactive"}
                    </span>
                  </h3>
                  <div className="flex flex-wrap gap-2">
                    {r.data.eligible.map((code) => (
                      <span
                        key={code}
                        className="rounded-full bg-blue-50 px-3 py-1 text-xs text-blue-800"
                      >
                        {catalogue.find((c) => c.code === code)?.name ?? code}
                      </span>
                    ))}
                  </div>
                  <p className="text-sm text-slate-500">{r.data.notes}</p>
                  {write && (
                    <button
                      className={secondary}
                      onClick={() => {
                        setViewId(null);
                        setEditing(r);
                      }}
                    >
                      Edit eligibility
                    </button>
                  )}
                  <ItemHistory
                    url={`/api/settings/manufacturers/${r.id}/history`}
                  />
                </article>
              ))}
          </div>
        </RecordDrawer>
      )}
      {!loading && !records.length && <p>No manufacturers yet.</p>}
    </section>
  );
}
function ManufacturerEditor({
  source,
  catalogue,
  done,
  close,
}: {
  source?: Manufacturer;
  catalogue: EligibilityChoice[];
  done: () => void;
  close: () => void;
}) {
  const [code, setCode] = useState(source?.code ?? "");
  const [name, setName] = useState(source?.name ?? "");
  const [eligible, setEligible] = useState(source?.data.eligible ?? []);
  const [eligibilityError, setEligibilityError] = useState("");
  const [notes, setNotes] = useState(source?.data.notes ?? "");
  const [reason, setReason] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  async function save(e: SubmitEvent<HTMLFormElement>) {
    const submittedForm = e.currentTarget instanceof HTMLFormElement ? e.currentTarget : undefined;
    e.preventDefault();
    if (!eligible.length) { setEligibilityError("Select at least one subcategory."); return; }
    setBusy(true);
    try {
      await request(
        `/api/settings/manufacturers${source ? `/${source.id}/profile` : ""}`,
        {
          ...(source ? { version: source.version } : { code }),
          name,
          data: { eligible, notes },
          reason,
        },
        source ? "PATCH" : "POST",
      );
      done();
    } catch (e) {
      if (e instanceof FieldValidationError && Array.isArray(e.detail)) {
        const issue = e.detail.find((issue: {loc?: string[]}) => issue.loc?.at(-1) === "eligible");
        if (issue && typeof issue.msg === "string") { setEligibilityError(issue.msg); return; }
      }
      if (displayServerErrors(submittedForm, e)) return;
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <ValidatedForm onSubmit={save} className={panel}>
      <h3 className="font-bold">
        {source ? "Edit manufacturer" : "New manufacturer"}
      </h3>
      {error && (
        <p role="alert" className="text-red-700">
          {error}
        </p>
      )}
      <fieldset disabled={busy} className="min-w-0 space-y-4">
        <div className="grid gap-3 sm:grid-cols-2">
          <ValidatedLabel>
            Manufacturer code
            <ValidatedInput name="code"
              className={input}
              disabled={!!source}
              required
              pattern="[A-Z0-9]{1,16}"
              value={code}
              onChange={(e) => setCode(e.target.value.toUpperCase())}
            />
          </ValidatedLabel>
          <ValidatedLabel>
            Manufacturer name
            <ValidatedInput name="name"
              className={input}
              required
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </ValidatedLabel>
        </div>
        <RequiredSelection selectionCount={eligible.length} className="min-w-0" onInvalid={() => setEligibilityError("Select at least one subcategory.")}>
          <legend className="font-semibold">
            Eligible subcategories<span aria-hidden="true" className="text-red-600"> *</span>
          </legend>
          <ManufacturerEligibility selected={eligible} onChange={values=>{setEligibilityError("");setEligible(values)}} initial={catalogue}/>
          {eligibilityError ? <p role="alert" className="mt-1 text-sm text-red-600">{eligibilityError}</p> : null}
        </RequiredSelection>
        <ValidatedLabel className="block">
          Eligibility notes
          <ValidatedTextarea name="notes"
            className={input}
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
          />
        </ValidatedLabel>
        <ValidatedLabel className="block">
          Change reason
          <ValidatedInput name="reason"
            className={input}
            required
            value={reason}
            onChange={(e) => setReason(e.target.value)}
          />
        </ValidatedLabel>
        <div className="flex flex-wrap gap-2">
          <button type="submit" className={button}>{busy ? "Saving…" : source ? "Save changes" : "Create manufacturer"}</button>
          {source && (
            <button
              type="button"
              className={secondary}
              disabled={!reason.trim()}
              onClick={async () => {
                setBusy(true);
                try {
                  await request(
                    `/api/settings/manufacturers/${source.id}`,
                    {
                      version: source.version,
                      is_active: !source.is_active,
                      reason,
                    },
                    "PATCH",
                  );
                  done();
                } catch (e) {
                  setError((e as Error).message);
                } finally {
                  setBusy(false);
                }
              }}
            >
              {source.is_active ? "Deactivate" : "Reactivate"}
            </button>
          )}
          <button type="button" className={secondary} onClick={close}>
            Cancel
          </button>
        </div>
      </fieldset>
    </ValidatedForm>
  );
}
