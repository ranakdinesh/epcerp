"use client";
import { displayServerErrors } from "@/components/ui/server-field-errors";
import { ValidatedForm, ValidatedInput, ValidatedLabel, ValidatedSelect } from "@/components/ui/validated-controls";
import { useAuthenticatedUser } from "@/components/auth-user-provider";

import { FormEvent, useDeferredValue, useEffect, useEffectEvent, useState } from "react";

type Reference = { id: string; name: string; code: string; is_active: boolean; state_id: string | null };
type Kind = "client-types" | "industries" | "states" | "cities";
type DialogMode = "create" | "view" | "edit" | null;
type SortKey = "name" | "code" | "state" | "status";
type StatusFilter = "all" | "active" | "inactive";

const labels: Record<Kind, { title: string; singular: string }> = {
  "client-types": { title: "Client Types", singular: "client type" },
  industries: { title: "Industries", singular: "industry" },
  states: { title: "States & Union Territories", singular: "state or union territory" },
  cities: { title: "Cities", singular: "city" },
};

function Icon({ name }: { name: "close" | "edit" | "eye" | "plus" | "search" }) {
  const paths = {
    close: <path d="m6 6 12 12M18 6 6 18" />,
    edit: <><path d="m14.5 5.5 4 4M4 20l4.2-1 10.6-10.6a2.8 2.8 0 0 0-4-4L4.2 15 4 20Z" /><path d="m13.5 5.5 4 4" /></>,
    eye: <><path d="M2.5 12C4.8 7.7 8 5.5 12 5.5S19.2 7.7 21.5 12C19.2 16.3 16 18.5 12 18.5S4.8 16.3 2.5 12Z" /><circle cx="12" cy="12" r="2.5" /></>,
    plus: <path d="M12 5v14M5 12h14" />,
    search: <><circle cx="10.5" cy="10.5" r="6" /><path d="m15 15 4.5 4.5" /></>,
  };
  return <svg aria-hidden="true" className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="1.9">{paths[name]}</svg>;
}

function normalized(value: string) { return value.trim().replace(/\s+/g, " ").toLocaleLowerCase(); }

export function ClientReferenceManagement({ kind }: { kind: Kind }) {
  const user = useAuthenticatedUser();
  const copy = labels[kind];
  const writePermission = kind === "client-types" ? "settings.client_types.write" : kind === "industries" ? "settings.industries.write" : "settings.locations.write";
  const canWrite = user.permissions.includes("system:admin") || user.permissions.includes(writePermission);
  const [records, setRecords] = useState<Reference[]>([]);
  const [states, setStates] = useState<Reference[]>([]);
  const [query, setQuery] = useState("");
  const deferredQuery = useDeferredValue(query);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [sortKey, setSortKey] = useState<SortKey>("name");
  const [sortDirection, setSortDirection] = useState<"asc" | "desc">("asc");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [notice, setNotice] = useState("");
  const [loading, setLoading] = useState(true);
  const [dialogMode, setDialogMode] = useState<DialogMode>(null);
  const [selected, setSelected] = useState<Reference | null>(null);
  const [name, setName] = useState("");
  const [code, setCode] = useState("");
  const [stateId, setStateId] = useState("");
  const [isActive, setIsActive] = useState(true);
  const [formError, setFormError] = useState("");
  const [isSaving, setIsSaving] = useState(false);
  const stateName = (id: string | null) => states.find((state) => state.id === id)?.name ?? "-";

  async function load() {
    setLoading(true);
    try {
      const [referenceResponse, stateResponse] = await Promise.all([
        fetch(`/api/settings/client-references/${kind}?include_inactive=true`, { cache: "no-store" }),
        kind === "cities" ? fetch("/api/settings/client-references/states?include_inactive=true", { cache: "no-store" }) : Promise.resolve(null),
      ]);
      if (!referenceResponse.ok) throw new Error("Unable to load master data.");
      setRecords(await referenceResponse.json() as Reference[]);
      setStates(stateResponse?.ok ? await stateResponse.json() as Reference[] : []);
    } catch (error) { setNotice(error instanceof Error ? error.message : "Unable to load master data."); }
    finally { setLoading(false); }
  }

  const loadForKind = useEffectEvent(() => { void load(); });

  useEffect(() => {
    const timer = window.setTimeout(() => { loadForKind(); }, 0);
    return () => window.clearTimeout(timer);
  }, [kind]);

  const filteredRecords = records.filter((record) => (statusFilter === "all" || record.is_active === (statusFilter === "active")) && `${record.name} ${record.code} ${stateName(record.state_id)} ${record.is_active ? "active" : "inactive"}`.toLocaleLowerCase().includes(deferredQuery.trim().toLocaleLowerCase()));
  const sortedRecords = [...filteredRecords].sort((left, right) => {
    const value = (record: Reference) => sortKey === "state" ? stateName(record.state_id) : sortKey === "status" ? (record.is_active ? "active" : "inactive") : record[sortKey];
    return value(left).localeCompare(value(right), undefined, { sensitivity: "base" }) * (sortDirection === "asc" ? 1 : -1);
  });
  const pageCount = Math.max(1, Math.ceil(sortedRecords.length / pageSize));
  const safePage = Math.min(page, pageCount);
  const pageRecords = sortedRecords.slice((safePage - 1) * pageSize, safePage * pageSize);

  function changeSort(nextKey: SortKey) { setSortDirection((current) => nextKey === sortKey ? (current === "asc" ? "desc" : "asc") : "asc"); setSortKey(nextKey); setPage(1); }
  function openDialog(mode: DialogMode, record: Reference | null = null) { if (mode !== "view" && !canWrite) return; setDialogMode(mode); setSelected(record); setName(record?.name ?? ""); setCode(record?.code ?? ""); setStateId(record?.state_id ?? ""); setIsActive(record?.is_active ?? true); setFormError(""); }
  function closeDialog() { setDialogMode(null); setSelected(null); setFormError(""); }

  async function submit(event: FormEvent<HTMLFormElement>) {
    const submittedForm = event.currentTarget instanceof HTMLFormElement ? event.currentTarget : undefined;
    event.preventDefault();
    if (!canWrite) { setFormError("You do not have permission to change this master record."); return; }
    const duplicate = records.find((record) => record.id !== selected?.id && normalized(record.name) === normalized(name) && (kind !== "cities" || record.state_id === stateId));
    if (duplicate) { displayServerErrors(submittedForm, {detail: [{loc: ["body", "name"], msg: `${duplicate.name} already exists${kind === "cities" ? ` in ${stateName(duplicate.state_id)}` : ""}.`}]}); return; }
    const statusChanged = selected ? selected.is_active !== isActive : !isActive;
    if (kind === "states" && selected?.is_active && !isActive && !window.confirm(`Deactivating ${selected.name} will also deactivate its active cities. Continue?`)) return;
    setIsSaving(true); setFormError("");
    const endpoint = selected ? `/api/settings/client-references/${kind}/${selected.id}` : `/api/settings/client-references/${kind}`;
    const response = await fetch(endpoint, { method: selected ? "PATCH" : "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ name, code, ...(kind === "cities" ? { state_id: stateId || null } : {}) }) });
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) { setIsSaving(false); if (displayServerErrors(submittedForm, payload)) return;  setFormError(typeof payload.detail === "string" ? payload.detail : `Unable to save ${copy.singular}.`); return; }
    if (statusChanged) {
      const action = isActive ? "activate" : "deactivate";
      const statusResponse = await fetch(`/api/settings/client-references/${kind}/${payload.id}/${action}`, { method: "POST", headers: { "Content-Type": "application/json" } });
      if (!statusResponse.ok) {
        const errorData = await statusResponse.json().catch(() => ({}));
        setIsSaving(false);
        setSelected(payload as Reference);
        setIsActive(Boolean(payload.is_active));
        await load();
        setFormError(`The details were saved, but the status could not be changed. ${typeof errorData.detail === "string" ? errorData.detail : `Unable to ${action} ${payload.name}.`}`);
        return;
      }
    }
    setIsSaving(false);
    const action = selected ? "updated" : "created";
    const statusNotice = statusChanged ? ` and marked ${isActive ? "active" : "inactive"}` : "";
    const cascadeNotice = kind === "states" && statusChanged ? (isActive ? " Its cities remain unchanged." : " Its active cities were also deactivated.") : "";
    closeDialog(); setNotice(`${payload.name} was ${action}${statusNotice}.${cascadeNotice}`); await load();
  }

  const column = (label: string, key: SortKey) => <th className="px-5 py-3"><button type="button" onClick={() => changeSort(key)} className="inline-flex items-center gap-1 rounded text-left hover:text-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500"><span>{label}</span><span className={sortKey === key ? "text-blue-700" : "text-slate-400"}>{sortKey === key && sortDirection === "desc" ? "↓" : "↑"}</span></button></th>;

  return <section className="mx-auto w-full max-w-[1500px] space-y-5">
    <div className="flex flex-col gap-4 border-b border-slate-200 pb-5 sm:flex-row sm:items-end sm:justify-between"><div><p className="text-xs font-bold uppercase tracking-[0.16em] text-blue-700">Settings / Client reference data</p><h2 className="mt-1 text-2xl font-extrabold tracking-tight text-slate-900">{copy.title}</h2></div>{canWrite ? <button type="button" onClick={() => openDialog("create")} className="inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-blue-700 px-4 text-sm font-bold text-white shadow-lg shadow-blue-900/15 hover:bg-blue-800 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"><Icon name="plus" /> Add {copy.singular}</button> : null}</div>
    {notice ? <p role="status" className="rounded-xl border border-blue-100 bg-blue-50 px-4 py-3 text-sm font-medium text-blue-800">{notice}</p> : null}
    <div className="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-[0_14px_34px_-26px_rgba(15,23,42,0.35)]"><div className="flex flex-col gap-3 border-b border-slate-100 p-4 sm:flex-row sm:items-center sm:justify-between"><div className="relative w-full sm:max-w-md"><span className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3 text-slate-400"><Icon name="search" /></span><ValidatedInput name="query" value={query} onChange={(event) => { setQuery(event.target.value); setPage(1); }} placeholder={`Search ${copy.title.toLocaleLowerCase()}`} className="min-h-11 w-full rounded-xl border border-slate-200 bg-slate-50 pl-10 pr-3 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100" /></div><ValidatedSelect name="status_filter" aria-label="Filter by status" value={statusFilter} onChange={(event) => { setStatusFilter(event.target.value as StatusFilter); setPage(1); }} className="min-h-11 rounded-xl border border-slate-200 bg-white px-3 text-sm font-bold text-slate-700"><option value="all">All statuses</option><option value="active">Active</option><option value="inactive">Inactive</option></ValidatedSelect><ValidatedLabel className="flex min-h-11 items-center gap-2 text-sm font-semibold text-slate-600">Rows<ValidatedSelect name="page_size" value={pageSize} onChange={(event) => { setPageSize(Number(event.target.value)); setPage(1); }} className="min-h-11 rounded-xl border border-slate-200 bg-white px-3 text-sm font-bold text-slate-700"><option value={20}>20</option><option value={50}>50</option><option value={100}>100</option></ValidatedSelect></ValidatedLabel></div><div className="overflow-x-auto"><table className="vtpl-data-table w-full min-w-[760px] text-left"><thead className="bg-slate-50 text-[11px] font-bold uppercase tracking-[0.12em] text-slate-500"><tr>{column("Name", "name")}{column("Code", "code")}{kind === "cities" ? column("State / UT", "state") : null}{column("Status", "status")}<th className="px-5 py-3 text-right">Actions</th></tr></thead><tbody className="divide-y divide-slate-100">{pageRecords.map((record) => <tr key={record.id} className="hover:bg-slate-50/80"><td className="px-5 py-3 text-sm font-bold text-slate-800">{record.name}</td><td className="px-5 py-3 font-mono text-xs text-slate-500">{record.code}</td>{kind === "cities" ? <td className="px-5 py-3 text-sm text-slate-600">{stateName(record.state_id)}</td> : null}<td className="px-5 py-3"><span className={`rounded-full px-2.5 py-1 text-xs font-bold ${record.is_active ? "bg-emerald-50 text-emerald-700" : "bg-slate-100 text-slate-600"}`}>{record.is_active ? "Active" : "Inactive"}</span></td><td className="px-5 py-3"><div className="flex justify-end gap-1"><button type="button" onClick={() => openDialog("view", record)} title={`View ${record.name}`} aria-label={`View ${record.name}`} className="grid h-10 w-10 place-items-center rounded-lg text-slate-500 hover:bg-blue-50 hover:text-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500"><Icon name="eye" /></button>{canWrite ? <button type="button" onClick={() => openDialog("edit", record)} title={`Edit ${record.name}`} aria-label={`Edit ${record.name}`} className="grid h-10 w-10 place-items-center rounded-lg text-slate-500 hover:bg-blue-50 hover:text-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500"><Icon name="edit" /></button> : null}</div></td></tr>)}{!loading && !pageRecords.length ? <tr><td colSpan={kind === "cities" ? 5 : 4} className="px-5 py-12 text-center text-sm text-slate-500">No {copy.title.toLocaleLowerCase()} match this search.</td></tr> : null}</tbody></table></div><div className="flex flex-col gap-3 border-t border-slate-100 px-5 py-3 text-sm text-slate-500 sm:flex-row sm:items-center sm:justify-between"><span>Showing {sortedRecords.length ? (safePage - 1) * pageSize + 1 : 0}-{Math.min(safePage * pageSize, sortedRecords.length)} of {sortedRecords.length}</span><div className="flex gap-2"><button type="button" disabled={safePage === 1} onClick={() => setPage((current) => current - 1)} className="min-h-10 rounded-lg border border-slate-200 px-3 font-bold text-slate-700 disabled:opacity-40">Previous</button><span className="grid min-h-10 place-items-center px-2 font-semibold">{safePage} / {pageCount}</span><button type="button" disabled={safePage === pageCount} onClick={() => setPage((current) => current + 1)} className="min-h-10 rounded-lg border border-slate-200 px-3 font-bold text-slate-700 disabled:opacity-40">Next</button></div></div></div>
    {dialogMode ? <div role="dialog" aria-modal="true" aria-labelledby="reference-dialog-title" className="fixed inset-0 z-50 flex items-end bg-slate-950/45 p-0 backdrop-blur-sm sm:items-center sm:justify-center sm:p-5"><ValidatedForm onSubmit={(event) => { if (dialogMode === "view") event.preventDefault(); else void submit(event); }} className="vtpl-modal max-h-[94dvh] w-full overflow-y-auto rounded-t-3xl bg-white p-5 shadow-2xl sm:max-w-xl sm:rounded-3xl sm:p-7"><div className="flex items-start justify-between gap-4"><div><p className="text-xs font-bold uppercase tracking-[0.14em] text-blue-700">Settings / {copy.title}</p><h3 id="reference-dialog-title" className="mt-1 text-xl font-extrabold text-slate-900">{dialogMode === "create" ? `Add ${copy.singular}` : dialogMode === "edit" ? `Edit ${copy.singular}` : `${copy.singular[0].toLocaleUpperCase()}${copy.singular.slice(1)} details`}</h3></div><button type="button" onClick={closeDialog} disabled={isSaving} aria-label="Close dialog" className="grid h-10 w-10 place-items-center rounded-xl text-slate-500 hover:bg-slate-100 focus:outline-none focus:ring-2 focus:ring-blue-500"><Icon name="close" /></button></div>{formError ? <p role="alert" className="mt-5 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm font-semibold text-red-800">{formError}</p> : null}{dialogMode === "view" && selected ? <div className="mt-6 rounded-2xl border border-slate-200 bg-slate-50 p-5"><dl className="grid gap-4 text-sm sm:grid-cols-2"><div><dt className="text-xs font-bold uppercase tracking-wide text-slate-400">Name</dt><dd className="mt-1 font-bold text-slate-900">{selected.name}</dd></div><div><dt className="text-xs font-bold uppercase tracking-wide text-slate-400">Code</dt><dd className="mt-1 font-mono text-slate-700">{selected.code}</dd></div>{kind === "cities" ? <div><dt className="text-xs font-bold uppercase tracking-wide text-slate-400">State / UT</dt><dd className="mt-1 font-bold text-slate-900">{stateName(selected.state_id)}</dd></div> : null}<div><dt className="text-xs font-bold uppercase tracking-wide text-slate-400">Status</dt><dd className="mt-1 font-bold text-slate-900">{selected.is_active ? "Active" : "Inactive"}</dd></div></dl></div> : <fieldset disabled={isSaving} className="mt-6 space-y-4"><ValidatedLabel className="block text-sm font-bold text-slate-700">Name<ValidatedInput name="name" required value={name} onChange={(event) => setName(event.target.value)} className="field" /></ValidatedLabel><ValidatedLabel className="block text-sm font-bold text-slate-700">Code <span className="font-normal text-slate-400">optional</span><ValidatedInput name="code" value={code} onChange={(event) => setCode(event.target.value)} placeholder="Generated from name if blank" className="field" /></ValidatedLabel>{kind === "cities" ? <ValidatedLabel className="block text-sm font-bold text-slate-700">State / Union Territory<ValidatedSelect name="state_id" required value={stateId} onChange={(event) => setStateId(event.target.value)} className="field"><option value="">Select state or UT</option>{states.filter((state) => state.is_active || state.id === selected?.state_id).map((state) => <option key={state.id} value={state.id} disabled={!state.is_active}>{state.name}{state.is_active ? "" : " (Inactive)"}</option>)}</ValidatedSelect></ValidatedLabel> : null}<ValidatedLabel className="flex min-h-14 items-center gap-3 rounded-xl border border-slate-200 bg-white px-4 py-3 text-sm text-slate-700"><ValidatedInput name="is_active" type="checkbox" checked={isActive} onChange={(event) => setIsActive(event.target.checked)} /><span><span className="block font-bold">Active</span><span className="block text-xs font-normal text-slate-500">{isActive ? `${copy.singular[0].toLocaleUpperCase()}${copy.singular.slice(1)} is available for use.` : `${copy.singular[0].toLocaleUpperCase()}${copy.singular.slice(1)} is inactive and unavailable for new selections.`}</span>{kind === "states" && !isActive ? <span className="mt-1 block text-xs font-semibold text-amber-700">Saving will also deactivate this state’s active cities.</span> : null}</span></ValidatedLabel></fieldset>}<div className="mt-7 flex flex-col-reverse gap-2 border-t border-slate-100 pt-5 sm:flex-row sm:items-center sm:justify-end"><div className="flex flex-col-reverse gap-2 sm:flex-row"><button type="button" onClick={closeDialog} disabled={isSaving} className="min-h-11 rounded-xl border border-slate-200 px-4 text-sm font-bold text-slate-700">{dialogMode === "view" ? "Close" : "Cancel"}</button>{dialogMode !== "view" ? <button type="submit" disabled={isSaving} className="min-h-11 rounded-xl bg-blue-700 px-5 text-sm font-bold text-white shadow-lg shadow-blue-900/15 hover:bg-blue-800 disabled:opacity-60">{isSaving ? "Saving..." : selected ? "Save changes" : `Add ${copy.singular}`}</button> : null}</div></div></ValidatedForm></div> : null}
  </section>;
}
