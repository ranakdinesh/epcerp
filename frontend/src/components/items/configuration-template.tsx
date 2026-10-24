"use client";
import { useEffect, useState, type SubmitEvent } from "react";
import { EyeIcon } from "@heroicons/react/24/outline";
import { DataGrid } from "@/components/ui/data-grid";
import { RecordDrawer } from "@/components/ui/record-drawer";
import { ValidatedForm, ValidatedLabel, ValidatedSelect } from "@/components/ui/validated-controls";
import { displayServerErrors } from "@/components/ui/server-field-errors";

export type Option = { id: string; code: string; label: string; is_active: boolean; sort_order: number };
export type Field = { id: string; field_code: string; field_name: string; input_type: string; mandatory: boolean; allow_na: boolean; use_in_description: boolean; description_order: number; description_prefix: string; description_suffix: string; use_in_duplicate_check: boolean; sort_order: number; is_active: boolean; options: Option[] };
export type Template = { id: string; template_name: string; model_required: boolean; revision: number; is_active: boolean; fields: Field[] };
const yes = (value: boolean) => value ? "Yes" : "No";
export function ConfigurationTemplate({ subcategoryId }: { subcategoryId: string }) {
  const [template, setTemplate] = useState<Template | null>(null);
  const [error, setError] = useState(""), [pending, setPending] = useState(false);
  const [field, setField] = useState<Field | null>(null);
  const [showPreview, setShowPreview] = useState(false);
  const [preview, setPreview] = useState<{ description: string; engineering_key: string } | null>(null);
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    const controller = new AbortController();
    fetch(`/api/settings/configuration-templates/${subcategoryId}`, { signal: controller.signal, cache: "no-store" }).then(async response => {
      if (response.status === 404) { setPending(true); return; }
      if (!response.ok) throw new Error();
      setTemplate(await response.json() as Template);
    }).catch(() => { if (!controller.signal.aborted) setError("Unable to load the template. Close and reopen this panel to retry."); });
    return () => controller.abort();
  }, [subcategoryId]);
  async function generate(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault(); const form = event.currentTarget;
    const values = Object.fromEntries(new FormData(form));
    setBusy(true); setError(""); setPreview(null);
    try {
      const response = await fetch(`/api/settings/configuration-templates/${subcategoryId}/preview`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ values }) });
      const body = await response.json();
      if (!response.ok) { if (!displayServerErrors(form, body)) setError(typeof body.detail === "string" ? body.detail : "Unable to preview these values."); return; }
      setPreview(body);
    } catch { setError("Unable to connect. Try again."); } finally { setBusy(false); }
  }
  if (!template) return <p role={error ? "alert" : "status"}>{error || (pending ? "Configuration template pending for this subcategory." : "Loading configuration template…")}</p>;
  return <section className="space-y-5">
    <header className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between"><div><h3 className="font-semibold">{template.template_name}</h3><p className="text-sm">Revision {template.revision} · {template.is_active ? "Active" : "Inactive"} · Model required: {yes(template.model_required)}</p></div>{template.is_active && <button type="button" className="min-h-11 rounded-xl border border-slate-200 px-4 text-sm font-semibold text-blue-700" onClick={() => { setShowPreview(value => !value); setPreview(null); }}>{showPreview ? "Hide preview" : "Preview configuration"}</button>}</header>
    <DataGrid label="Template fields" rows={template.fields} initialSort="sort_order" status={row => row.is_active ? "Active" : "Inactive"} columns={[
      { key: "sort_order", label: "Order", value: row => row.sort_order },
      { key: "field_name", label: "Field", value: row => row.field_name },
      { key: "input_type", label: "Input", value: row => row.input_type },
      { key: "mandatory", label: "Mandatory", value: row => yes(row.mandatory) },
      { key: "allow_na", label: "Allow N/A", value: row => yes(row.allow_na) },
      { key: "description", label: "In description", value: row => yes(row.use_in_description) },
      { key: "description_order", label: "Description order", value: row => row.description_order },
      { key: "prefix", label: "Prefix", value: row => row.description_prefix || "—", render: row => <span className="whitespace-pre">{row.description_prefix || "—"}</span> },
      { key: "suffix", label: "Suffix", value: row => row.description_suffix || "—" },
      { key: "duplicate", label: "Duplicate check", value: row => yes(row.use_in_duplicate_check) },
      { key: "options", label: "Options", value: row => row.options.length },
      { key: "status", label: "Status", value: row => row.is_active ? "Active" : "Inactive" },
    ]} actions={row => <button type="button" className="flex min-h-11 min-w-11 items-center justify-center text-blue-700" title={`View options for ${row.field_name}`} aria-label={`View options ${row.field_code}`} onClick={() => setField(row)}><EyeIcon className="h-5 w-5" /></button>} />
    {showPreview && <ValidatedForm onSubmit={generate} className="space-y-4 border-t border-slate-200 pt-5"><h3 className="font-semibold">Configuration preview</h3><p className="text-sm">N/A satisfies a required selection where allowed and is omitted from the description. This preview does not create an item or SKU.</p><fieldset disabled={busy} className="grid gap-4 sm:grid-cols-2">{template.fields.filter(row => row.is_active).map(row => <ValidatedLabel key={row.id}>{row.field_name}<ValidatedSelect name={row.field_code} required={row.mandatory} defaultValue="" className="mt-1 block min-h-11 w-full rounded-lg border border-slate-200 bg-white px-3"><option value="">{row.mandatory ? "Select a value" : "Not specified"}</option>{row.options.filter(option => option.is_active).map(option => <option key={option.id} value={option.code}>{option.label}</option>)}{row.allow_na && <option value="__NA__">N/A — Not applicable</option>}</ValidatedSelect></ValidatedLabel>)}</fieldset><button type="submit" disabled={busy} className="min-h-11 rounded-xl bg-blue-700 px-4 font-semibold text-white disabled:opacity-50">{busy ? "Generating…" : "Generate preview"}</button>{preview && <section role="status" className="space-y-2 rounded-xl border border-slate-200 p-4"><h4 className="font-semibold">Specification description</h4><p>{preview.description || "No description fields selected."}</p><details><summary className="min-h-11 cursor-pointer text-sm">Engineering identity preview</summary><p className="break-all font-mono text-xs">{preview.engineering_key}</p><p className="text-sm">Uses only marked fields within this subcategory. Manufacturer/model SKU matching is a later step.</p></details></section>}</ValidatedForm>}
    {error && <p role="alert" className="text-red-700">{error}</p>}
    {field && <RecordDrawer title={`${field.field_name} options`} onClose={() => setField(null)}><p className="text-sm">{field.field_code} · {field.allow_na ? "N/A is also allowed as an explicit selection." : "N/A is not allowed."}</p><DataGrid label={`${field.field_name} options`} rows={field.options} initialSort="sort_order" status={row => row.is_active ? "Active" : "Inactive"} columns={[{ key: "sort_order", label: "Order", value: row => row.sort_order },{ key: "code", label: "Code", value: row => row.code },{ key: "label", label: "Value", value: row => row.label },{ key: "status", label: "Status", value: row => row.is_active ? "Active" : "Inactive" }]} /></RecordDrawer>}
  </section>;
}
