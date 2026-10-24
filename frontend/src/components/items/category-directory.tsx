"use client";

import { useCallback, useEffect, useState } from "react";
import { ArrowLeftIcon, EyeIcon, PencilSquareIcon, PlusIcon } from "@heroicons/react/24/outline";
import { useAuthenticatedUser } from "@/components/auth-user-provider";
import { DataGrid, type GridQuery } from "@/components/ui/data-grid";
import { RecordDrawer } from "@/components/ui/record-drawer";
import { ConfigurationEditor } from "./template-editor";
import { ConfigurationTemplate } from "./configuration-template";
import { CategoryForm } from "./category-form";
import type { Catalogue, CatalogueKind, CatalogueRecord as Record } from "./category-types";

const primaryAction = "inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-blue-700 px-4 text-sm font-bold text-white shadow-lg shadow-blue-900/15 hover:bg-blue-800";
const iconAction = "flex min-h-11 min-w-11 items-center justify-center text-blue-700";
type Editor = { kind: CatalogueKind; record?: Record; parentCategoryId?: string };
type Page = { rows: Record[]; total: number };
const initialQuery: GridQuery = { search: "", status: "", sort: "name", direction: "asc", page: 1, size: 20 };
function unitName(row: Record) { return row.unit_code === row.unit_name ? row.unit_name : `${row.unit_code} — ${row.unit_name}`; }
function rowUnit(row: Record): Record { return { id: row.unit_id!, code: row.unit_code!, name: row.unit_name!, is_active: true, version: 1 }; }

export function CategoryDirectory() {
  const user = useAuthenticatedUser();
  const canRead = user.permissions.includes("system:admin") || user.permissions.includes("settings.categories.read");
  const canReadTemplates = user.permissions.includes("system:admin") || user.permissions.includes("settings.configuration_templates.read");
  const canConfigure = user.permissions.includes("system:admin") || user.permissions.includes("settings.configuration_templates.write");
  const [configurationEditor,setConfigurationEditor] = useState<Record|null>(null);
  const canWrite = user.permissions.includes("system:admin") || user.permissions.includes("settings.categories.write");
  const [editor, setEditor] = useState<Editor | null>(null);
  const [addingUnit, setAddingUnit] = useState(false);
  const [formData, setFormData] = useState<Catalogue>({ categories: [], subcategories: [], units: [] });
  const [result, setResult] = useState<Page>({ rows: [], total: 0 });
  const [loading, setLoading] = useState(true), [error, setError] = useState("");
  const [selected, setSelected] = useState<Record | null>(null);
  const [configured, setConfigured] = useState<Record | null>(null);
  const [query, setQuery] = useState<GridQuery>(initialQuery);
  const [reload, setReload] = useState(0);
  const parentId = selected?.id;
  useEffect(() => {
    if (!canRead) return;
    const controller = new AbortController();
    const params = new URLSearchParams({ kind: parentId ? "subcategory" : "category", search: query.search, status: query.status, sort: query.sort, direction: query.direction, page: String(query.page), size: String(query.size) });
    if (parentId) params.set("category_id", parentId);
    fetch(`/api/settings/categories/browse?${params}`, { signal: controller.signal, cache: "no-store" }).then(async response => {
      if (!response.ok) throw new Error();
      setResult(await response.json() as Page);
    }).catch(() => { if (!controller.signal.aborted) { setResult({ rows: [], total: 0 }); setError("Unable to load catalogue records. Try again."); } })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [canRead, parentId, query, reload]);
  const changeQuery = useCallback((next: GridQuery) => { setLoading(true); setError(""); setQuery(next); }, []);
  function refresh() { setLoading(true); setError(""); setReload(n => n + 1); }
  function openCategory(row: Record | null) { setLoading(true); setError(""); setResult({ rows: [], total: 0 }); setQuery(initialQuery); setSelected(row); }
  async function openEditor(next: Editor) {
    setError("");
    const category = next.kind === "categories" ? next.record : selected;
    setFormData({ categories: category ? [category] : [], subcategories: [], units: category?.unit_id ? [rowUnit(category)] : [] });
    if (next.kind === "categories" && !next.record) {
      try {
        const response = await fetch("/api/settings/categories/browse?kind=unit&size=100&status=Active", { cache: "no-store" });
        if (!response.ok) throw new Error();
        const units = await response.json() as Page;
        setFormData({ categories: [], subcategories: [], units: units.rows });
      } catch { setError("Unable to load base units. Try again."); return; }
    }
    setEditor(next);
  }
  async function detail(kind: string, id: string) {
    const response = await fetch(`/api/settings/categories/detail/${kind}/${id}`, { cache: "no-store" });
    if (!response.ok) throw new Error();
    return await response.json() as Record;
  }
  async function viewConfiguration(row: Record) {
    setConfigured(row);
  }
  async function editUnit() {
    if (!selected?.unit_id) return;
    try { await openEditor({ kind: "units", record: await detail("unit", selected.unit_id) }); } catch { setError("Unable to load the unit. Try again."); }
  }
  const kind = selected ? "subcategories" : "categories";
  if (!canRead) return <p role="alert">Category read permission is required.</p>;
  return <section className="space-y-5">
    {selected && <nav aria-label="Catalogue breadcrumb" className="flex flex-wrap items-center gap-2 text-sm"><button type="button" className="inline-flex min-h-11 items-center gap-2 font-semibold text-blue-700" onClick={() => openCategory(null)}><ArrowLeftIcon className="h-4 w-4" />Categories</button><span aria-hidden="true">/</span><span aria-current="page">{selected.name}</span></nav>}
    <header className="flex flex-col gap-4 border-b border-slate-200 pb-5 sm:flex-row sm:items-end sm:justify-between"><div><h1 className="text-2xl font-bold">{selected ? selected.name : "Categories"}</h1><p className="mt-1 text-sm">{selected ? `${selected.code} · Base unit: ${unitName(selected)} · ${selected.is_active ? "Active" : "Inactive"}` : "Select a category to manage its subcategories."}</p></div>{canWrite && (!selected || selected.is_active) && <button type="button" className={primaryAction} onClick={() => void openEditor({ kind, parentCategoryId: selected?.id })}><PlusIcon className="h-5 w-5" />Add {selected ? "subcategory" : "category"}</button>}</header>
    {error && <p role="alert" className="text-red-700">{error} <button type="button" className="min-h-11 underline" onClick={refresh}>Retry</button></p>}
    {selected && <h2 className="text-lg font-semibold">Subcategories</h2>}
    <DataGrid key={parentId ?? "categories"} label={selected ? "Subcategories" : "Categories"} stateKey={selected ? `catalogue_category_${selected.id}` : "catalogue_categories"} rows={result.rows} loading={loading} initialSort="name" remote={{ total: result.total, onChange: changeQuery }} status={row => row.is_active ? "Active" : "Inactive"} columns={[
      { key: "code", label: selected ? "Type code" : "Code", value: row => row.code },
      { key: "name", label: "Name", value: row => row.name, render: selected ? undefined : row => <button type="button" className="min-h-11 text-left font-semibold text-blue-700 underline-offset-4 hover:underline" onClick={() => openCategory(row)}>{row.name}</button> },
      { key: "unit", label: "Base unit", value: row => unitName(row) ?? "—" },
      ...(!selected ? [{ key: "types", label: "Subcategories", value: (row: Record) => row.child_count ?? 0, render: (row: Record) => <button type="button" className="min-h-11 min-w-11 font-semibold text-blue-700 underline underline-offset-4" aria-label={`Open subcategories for ${row.name}`} onClick={() => openCategory(row)}>{row.child_count ?? 0}</button> }] : []),
      { key: "status", label: "Status", value: row => row.is_active ? "Active" : "Inactive" },
    ]} actions={row => <div className="flex gap-1"><button type="button" title={`View ${row.name}`} aria-label={`View ${row.code}`} onClick={() => selected ? void viewConfiguration(row) : openCategory(row)} className={iconAction}><EyeIcon className="h-5 w-5" /></button>{selected && canConfigure && <button type="button" className="min-h-11 px-2 font-semibold text-blue-700" aria-label={`Configure ${row.code}`} onClick={()=>setConfigurationEditor(row)}>Configuration</button>}{canWrite && <button type="button" title={`Edit ${row.name}`} aria-label={`Edit ${row.code}`} onClick={() => void openEditor({ kind, record: row })} className={iconAction}><PencilSquareIcon className="h-5 w-5" /></button>}</div>} />
    {selected && canWrite && <button type="button" className="min-h-11 text-sm font-semibold text-blue-700" onClick={() => void editUnit()}>Correct base unit name/code</button>}
    {configurationEditor && <RecordDrawer title={`Configuration — ${configurationEditor.name}`} onClose={()=>setConfigurationEditor(null)}><ConfigurationEditor id={configurationEditor.id} code={configurationEditor.code} onSaved={()=>{setConfigurationEditor(null);setConfigured(null);refresh()}}/></RecordDrawer>}
    {configured && <RecordDrawer title={configured.name} onClose={() => setConfigured(null)}>
      <p className="text-sm">{selected?.name} / {configured.name}</p><dl className="grid grid-cols-2 gap-3"><dt>Type code</dt><dd>{configured.code}</dd><dt>Base unit</dt><dd>{unitName(configured)}</dd></dl>
      {canReadTemplates ? <ConfigurationTemplate key={configured.id} subcategoryId={configured.id} /> : <p>Configuration template read permission is required.</p>}
    </RecordDrawer>}
    {editor && <RecordDrawer title={`${editor.record ? "Edit" : "Add"} ${editor.kind === "categories" ? "category" : editor.kind === "subcategories" ? "subcategory" : "unit"}`} onClose={() => setEditor(null)}><CategoryForm key={`${editor.kind}_${editor.record?.id ?? "new"}`} kind={editor.kind} data={formData} record={editor.record} parentCategoryId={editor.parentCategoryId} onAddUnit={() => setAddingUnit(true)} onSaved={async record => {
      setEditor(null); refresh();
      if (selected && (editor.kind === "units" || (editor.kind === "categories" && record.id === selected.id))) {
        try { setSelected(await detail("category", selected.id)); } catch { setError("Saved. Reload the category to refresh its details."); }
      }
    }} /></RecordDrawer>}
    {addingUnit && <RecordDrawer title="Add base unit" onClose={() => setAddingUnit(false)}><CategoryForm kind="units" data={formData} onSaved={record => { setFormData(current => ({ ...current, units: [...current.units, record] })); setAddingUnit(false); }} /></RecordDrawer>}
  </section>;
}
