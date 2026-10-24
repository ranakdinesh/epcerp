"use client";
import { useState, type SubmitEvent } from "react";
import { ValidatedForm, ValidatedLabel as Label, ValidatedInput as Input, ValidatedSelect as Select, ValidatedTextarea as Textarea } from "@/components/ui/validated-controls";
import { displayServerErrors } from "@/components/ui/server-field-errors";
import type { Catalogue, CatalogueKind, CatalogueRecord } from "./category-types";
import styles from "./category-form.module.css";

export function CategoryForm({ kind, data, record, parentCategoryId, onAddUnit, onSaved }: { parentCategoryId?: string; onAddUnit?: () => void; record?: CatalogueRecord; kind: CatalogueKind; data: Catalogue; onSaved: (record: CatalogueRecord) => void }) {
  const [busy, setBusy] = useState(false), [error, setError] = useState("");
  const categoryId = parentCategoryId ?? record?.category_id;
  const category = data.categories.find(c => c.id === categoryId);
  const inherited = data.units.find(u => u.id === category?.unit_id);
  const singular = kind === "categories" ? "category" : kind === "subcategories" ? "subcategory" : "unit";
  async function submit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault(); const form = event.currentTarget; const values = new FormData(form);
    const payload = { code: record && String(values.get("code")) === record.code ? record.code : String(values.get("code") ?? "").trim().toUpperCase(), name: String(values.get("name") ?? "").trim(), reason: String(values.get("reason") ?? "").trim(), ...(record ? {version:record.version} : kind === "categories" ? { unit_id: String(values.get("unit_id")) } : kind === "subcategories" ? { category_id: categoryId } : {}) };
    setBusy(true); setError("");
    try { const response = await fetch(`/api/settings/categories/${kind}${record ? `/${record.id}` : ""}`, { method: record ? "PATCH" : "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(payload) }); const body = await response.json(); if (!response.ok) { if (!displayServerErrors(form, body)) setError(typeof body.detail === "string" ? body.detail : "Unable to save this record. Check your access or try again."); return; } onSaved(body as CatalogueRecord); }
    catch { setError("Unable to connect. Please try again."); } finally { setBusy(false); }
  }
  return <ValidatedForm className={`${styles.form} space-y-5`} onSubmit={submit}><fieldset disabled={busy} className="space-y-5">
    <Label>{kind === "subcategories" ? "Type code" : "Code"}<Input name="code" required defaultValue={record?.code} maxLength={kind === "units" ? 16 : 32} pattern="[A-Za-z0-9]+" title="Use letters and digits." /></Label>
    <Label>Name<Input name="name" defaultValue={record?.name} required maxLength={kind === "units" ? 128 : 255} /></Label>
    {kind === "categories" && !record && <div className="space-y-2"><Label>Base unit<Select name="unit_id" required defaultValue=""><option value="">Select a base unit</option>{data.units.filter(u => u.is_active).map(u => <option key={u.id} value={u.id}>{u.code} — {u.name}{u.code === "Nos" ? " (count / units)" : ""}</option>)}</Select></Label>{onAddUnit && <button type="button" className="min-h-11 text-sm font-semibold text-blue-700" onClick={onAddUnit}>Add a new base unit</button>}</div>}
    {kind === "subcategories" && <><p className="text-sm">Category: <strong>{category ? `${category.name} (${category.code})` : "Unavailable"}</strong></p><p className="text-sm">Base unit: <strong>{inherited ? `${inherited.code} — ${inherited.name}` : "Unavailable"}</strong>. Inherited from the category.</p>{!record && <p className="text-sm">Specification configuration will be pending after creation.</p>}</>}
    {kind === "units" && record && <p className="text-sm">Correcting this shared unit updates its label in every category using it; it does not convert quantities.</p>}
    {record?.unit_id && <p className="text-sm">Base unit: <strong>{data.units.find(u => u.id === record.unit_id)?.code}</strong>. Unit relationships are preserved during corrections.</p>}
    <Label>{record ? "Reason for correction" : "Reason for creation"}<Textarea name="reason" required maxLength={2000} /></Label>
    {error && <p role="alert" className="text-red-700">{error}</p>}
    <button type="submit" className="rounded-lg bg-blue-700 px-4 py-3 font-semibold text-white">{busy ? "Saving…" : record ? "Save changes" : `Create ${singular}`}</button>
  </fieldset></ValidatedForm>;
}
