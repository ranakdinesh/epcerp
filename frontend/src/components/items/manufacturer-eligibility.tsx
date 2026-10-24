"use client";
import { useEffect, useState } from "react";
import { ValidatedInput, ValidatedLabel, ValidatedSelect } from "@/components/ui/validated-controls";
export type EligibilityChoice = { code: string; name: string; kind: string; parent_name: string; parent_code: string; is_active: boolean };
type Page = { rows: EligibilityChoice[]; total: number };
const input = "mt-1 block min-h-11 w-full rounded-lg border border-slate-200 bg-white px-3";
async function choices(params: Record<string, string>, signal?: AbortSignal): Promise<Page> {
  const response = await fetch(`/api/settings/manufacturers/eligibility?${new URLSearchParams(params)}`, { cache: "no-store", signal });
  if (!response.ok) throw new Error("Unable to load categories or subcategories. Please reopen the form to retry.");
  return response.json();
}
export function ManufacturerEligibility({ selected, onChange, initial }: { selected: string[]; onChange: (codes: string[]) => void; initial: EligibilityChoice[] }) {
  const [categories, setCategories] = useState(initial.filter(c => c.kind === "category"));
  const [categoryTotal, setCategoryTotal] = useState(0), [category, setCategory] = useState("");
  const [rows, setRows] = useState<EligibilityChoice[]>([]), [known, setKnown] = useState(initial);
  const [skip, setSkip] = useState(0), [total, setTotal] = useState(0), [loading, setLoading] = useState(false), [busy, setBusy] = useState(false), [error, setError] = useState("");
  const selectedKey = selected.join(",");
  useEffect(() => {
    const controller = new AbortController();
    choices({ kind: "category" }, controller.signal).then(body => { setCategories(body.rows); setCategoryTotal(body.total); }).catch(e => { if (!controller.signal.aborted) setError(e.message); });
    return () => controller.abort();
  }, []);
  useEffect(() => {
    if (!selectedKey) return;
    const controller = new AbortController();
    choices({ codes: selectedKey }, controller.signal).then(body => {
      setKnown(body.rows);
      const first = body.rows[0];
      if (first) setCategory(current => current || (first.kind === "category" ? first.code : first.parent_code));
    }).catch(e => { if (!controller.signal.aborted) setError(e.message); });
    return () => controller.abort();
  }, [selectedKey]);
  useEffect(() => {
    if (!category) return;
    const controller = new AbortController();
    choices({ kind: "subcategory", category_code: category, skip: String(skip) }, controller.signal).then(body => { setRows(body.rows); setTotal(body.total); }).catch(e => { if (!controller.signal.aborted) setError(e.message); }).finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [category, skip]);
  async function allChildren() {
    const all: EligibilityChoice[] = [];
    for (let offset = 0; ; offset += 50) {
      const body = await choices({ kind: "subcategory", category_code: category, skip: String(offset) });
      if (body.total > 200) throw new Error("A manufacturer supports up to 200 eligibility links. Choose individual subcategories.");
      all.push(...body.rows);
      if (all.length >= body.total || !body.rows.length) return all;
    }
  }
  function withoutCategory(codes: string[], all: EligibilityChoice[]) {
    const owned = new Set([category, ...all.map(c => c.code), ...known.filter(c => c.parent_code === category).map(c => c.code)]);
    return codes.filter(code => !owned.has(code));
  }
  async function selectAll(clear = false) {
    setBusy(true); setError("");
    try { const all = await allChildren(); const other = withoutCategory(selected, all); const next = clear ? other : [...other, ...all.map(c => c.code)]; if (next.length > 200) throw new Error("Select up to 200 eligibility links for one manufacturer."); onChange(next); }
    catch (e) { setError((e as Error).message); } finally { setBusy(false); }
  }
  async function toggle(code: string, checked: boolean) {
    if (!selected.includes(category)) { onChange(checked ? [...selected, code] : selected.filter(c => c !== code)); return; }
    setBusy(true); setError("");
    try { const all = await allChildren(); onChange([...withoutCategory(selected, all), ...all.filter(c => checked || c.code !== code).map(c => c.code)]); }
    catch (e) { setError((e as Error).message); } finally { setBusy(false); }
  }
  const extraCategory = category && !categories.some(c => c.code === category);
  const categoryLabel = categories.find(c => c.code === category)?.name || known.find(c => c.parent_code === category)?.parent_name || category;
  return <div className="min-w-0 space-y-4">
    <ValidatedLabel>Category<ValidatedSelect name="manufacturer_category" value={category} onChange={e => { setCategory(e.target.value); setSkip(0); setRows([]); setTotal(0); setLoading(Boolean(e.target.value)); setError(""); }} className={input}>
      <option value="">Choose category</option>{extraCategory && <option value={category}>{categoryLabel}</option>}{categories.map(c => <option key={c.code} value={c.code}>{c.name}</option>)}
    </ValidatedSelect></ValidatedLabel>
    {categories.length < categoryTotal && <button type="button" className="min-h-11 text-blue-700" onClick={async () => { try { const body = await choices({ kind: "category", skip: String(categories.length) }); setCategories(current => [...current, ...body.rows]); } catch (e) { setError((e as Error).message); } }}>Load more categories</button>}
    {category && <section aria-label={`${categoryLabel} subcategories`} className="space-y-3">
      <header className="flex flex-wrap items-center justify-between gap-2"><h4 className="font-semibold">Subcategories</h4><div className="flex gap-3"><button type="button" disabled={busy || !total} onClick={() => void selectAll()} className="min-h-11 font-semibold text-blue-700">Select all</button><button type="button" disabled={busy || !total} onClick={() => void selectAll(true)} className="min-h-11 text-blue-700">Clear</button></div></header>
      {loading ? <p role="status">Loading subcategories…</p> : <div className="grid gap-2 sm:grid-cols-2">{rows.map(c => <ValidatedLabel key={c.code} className="flex min-h-11 items-center gap-2"><ValidatedInput type="checkbox" disabled={busy} checked={selected.includes(c.code) || selected.includes(category)} onChange={e => void toggle(c.code, e.target.checked)} />{c.name}</ValidatedLabel>)}</div>}
      {!loading && !total && <p>No active subcategories in this category.</p>}
      {total > 50 && <div className="flex items-center gap-3"><button type="button" disabled={!skip || busy} className="min-h-11 text-blue-700" onClick={() => { setLoading(true); setSkip(skip - 50); }}>Previous</button><span>{skip + 1}–{Math.min(skip + 50, total)} of {total}</span><button type="button" disabled={skip + 50 >= total || busy} className="min-h-11 text-blue-700" onClick={() => { setLoading(true); setSkip(skip + 50); }}>Next</button></div>}
    </section>}
    {selected.length > 0 && <p className="text-sm">{selected.length} saved or selected links. Selections in other categories are retained.</p>}
    {error && <p role="alert" className="text-red-700">{error}</p>}
  </div>;
}
