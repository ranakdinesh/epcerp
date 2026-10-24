"use client";
import { useEffect, useId, useState } from "react";
import { DataGrid } from "@/components/ui/data-grid";
import { RecordDrawer } from "@/components/ui/record-drawer";
import { ValidatedForm, ValidatedInput, ValidatedLabel } from "@/components/ui/validated-controls";
import { displayServerErrors } from "@/components/ui/server-field-errors";
import { decimalUnits, itemKey, moneyText, unitBudget } from "./purchase-budget-calculations";

import {BudgetDraftEditor} from "./budget-draft-editor";
import {type BudgetData as Data,type BudgetOwner} from "./budget-contract";
import {UserIdentity} from "@/components/personal-profile";
type Item = { bo_id: string; bo_version: number; line_id: string; bo_name: string; quantity: string; item_snapshot: { description?: string; sku?: string; uom?: string } };
type Workspace = {
  record: { version: number; revision: number; state: string; data: Data; approved: { revision: number; data: Data } | null } | null;
  project: { name:string; currency: string; contract_value: string | null };
  owners: BudgetOwner[]; items: Item[]; can_write: boolean; can_approve: boolean;
  report: { purchase_total: string; allocated_heads: string; unallocated: string } | null;
  history: { id:string; action: string; reason: string; created_at: string; created_by_id:string; before:unknown; after:unknown }[];
};
const button = "min-h-11 rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold disabled:opacity-50";
const input = "min-h-11 w-full rounded-lg border border-slate-300 bg-white px-3 py-2";

export function ProjectBudgetPanel({ projectId,initialView="summary",onSaved }: { projectId: string;initialView?:"summary"|"items";onSaved?:()=>void }) {
  const [ws, setWs] = useState<Workspace | null>(null), [error, setError] = useState(""), [busy, setBusy] = useState(false), [reload, setReload] = useState(0);
  const [showItems, setShowItems] = useState(initialView==="items"), [rates, setRates] = useState<Record<string, string>>({});
  const priceFormID=useId();
  const [editDraft,setEditDraft]=useState(false),[notice,setNotice]=useState("");
  const [action, setAction] = useState<"approve" | "revise" | null>(null);
  const url = `/api/projects/${projectId}/budget`;
  useEffect(() => {
    const controller = new AbortController();
    fetch(url, { cache: "no-store", signal: controller.signal }).then(async response => {
      const payload = await response.json();
      if (!response.ok) throw Error(typeof payload.detail === "string" ? payload.detail : "Cannot load project budget");
      setWs(payload); setError("");
    }).catch(error => { if (error.name !== "AbortError") setError(error.message); });
    return () => controller.abort();
  }, [url, reload]);
  async function command(payload: { action: string; [key: string]: unknown }, form: HTMLFormElement) {
    setBusy(true); setError("");
    try {
      const response = await fetch(url, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ ...payload, version: ws?.record?.version ?? 0 }) });
      const result = await response.json();
      if (!response.ok) {
        if (displayServerErrors(form, result)) return false;
        throw Error(typeof result.detail === "string" ? result.detail : Array.isArray(result.detail) ? result.detail.map((issue:{msg?:string})=>issue.msg ?? "Check budget inputs").join("; ") : "Check the item prices and reload if the purchase list changed");
      }
      setWs(result); onSaved?.(); setNotice(payload.action === "approve" ? "Budget approved. Director notification recorded." : payload.action === "revise" ? "Budget revision started. The previous approved baseline remains effective." : "Budget draft saved."); return true;
    } catch (error) { setError(error instanceof Error ? error.message : "Budget save failed"); return false; }
    finally { setBusy(false); }
  }
  if (!ws) return <section><p role={error ? "alert" : "status"}>{error || "Loading project budget…"}</p><button className={button} onClick={() => {setWs(null);setError("");setReload(value => value + 1)}}>Reload budget</button></section>;
  const currency = ws.project.currency;
  const main = decimalUnits(ws.project.contract_value ?? "0", 2) ?? BigInt(0);
  const allocations = new Map(ws.record?.data.allocations.filter(row => row.is_active).map(row => [itemKey(row), row]));
  const otherHeads = ws.record?.data.heads.filter(row => row.is_active && row.kind !== "purchase" && !row.parent_id) ?? [];
  const other = otherHeads.reduce((sum, head) => sum + (decimalUnits(head.amount, 2) ?? BigInt(0)), BigInt(0));
  const editable = ws.can_write && ws.record?.state !== "approved" && ws.project.contract_value !== null;
  const rows = ws.items.map(item => {
    const key = itemKey(item), allocation = allocations.get(key);
    const changed = Object.hasOwn(rates, key), rate = changed ? rates[key] : allocation?.unit_price ?? "";
    const amount = ws.record?.state === "approved" ? (allocation ? decimalUnits(allocation.amount, 2) : null) : rate ? unitBudget(item.quantity, rate) : changed ? null : allocation ? decimalUnits(allocation.amount, 2) : null;
    return { ...item, id: key, rate, amount, allocation, changed };
  });
  const total = rows.reduce((sum, row) => sum + (row.amount ?? BigInt(0)), BigInt(0));
  const remaining = main - other - total;
  const savedPurchase = decimalUnits(ws.report?.purchase_total ?? "0", 2) ?? BigInt(0);
  const priced = rows.filter(row => row.amount !== null).length;
  const pendingQuantities=rows.some(row=>(decimalUnits(row.quantity,4) ?? BigInt(0))<=BigInt(0));
  const metric = (label: string, value: bigint | null) => <div><p className="text-sm text-slate-600">{label}</p><p className={`mt-1 text-lg font-bold tabular-nums ${value !== null && value < BigInt(0) ? "text-red-700" : "text-slate-900"}`}>{value===null?"Pending":moneyText(value, currency)}</p></div>;
  return <section className="space-y-5">
    <header className="rounded-xl border bg-white p-5"><div className="flex flex-wrap items-start justify-between gap-3"><h2 className="text-xl font-bold">Project budget</h2>{editable&&<button className="min-h-11 rounded-xl bg-blue-700 px-4 py-2 font-semibold text-white" disabled={busy} onClick={()=>{setError("");setEditDraft(true)}}>Edit budget draft</button>}</div><p className="mt-2 text-sm text-slate-600">Open Item purchase to price the project’s Purchase FMS items. Each unit price calculates the item budget and reduces the amount remaining from the main project budget.</p><p className="mt-2 font-semibold">{ws.project.name} · Revision {ws.record?.revision ?? 1} · {ws.record?.state ?? "Not yet saved"}</p>
      <div className="mt-4 grid gap-4 sm:grid-cols-3">{metric("Main project budget", ws.project.contract_value===null?null:main)}{metric("Saved purchase allocation", savedPurchase)}{metric("Remaining main budget", ws.project.contract_value===null?null:main - other - savedPurchase)}</div>
      {ws.project.contract_value === null && <p role="status" className="mt-3 text-amber-900">Ask an administrator to enter the main project value first.</p>}
      {ws.record?.approved && <p className="mt-3 text-sm">Approved revision {ws.record.approved.revision} remains effective until a new revision is approved.</p>}
      <div className="mt-4 flex flex-wrap gap-2">{ws.can_approve && ws.record?.state === "draft" && <button className={button} disabled={busy} onClick={() => setAction("approve")}>Approve budget</button>}{ws.can_write && ws.record?.state === "approved" && <button className={button} disabled={busy} onClick={() => setAction("revise")}>Start budget revision</button>}<button className={button} disabled={busy} onClick={() => {setWs(null);setError("");setReload(value => value + 1)}}>Reload saved budget</button></div>
    </header>
    {notice&&<p role="status" className="rounded-lg bg-green-50 p-3 text-green-800">{notice}</p>}
    {!ws.can_approve&&<p className="text-sm text-slate-600">Budget approval requires active approval permission within this project.</p>}
    {error && !showItems && !action && <p role="alert" className="rounded-lg bg-red-50 p-3 text-red-800">{error}</p>}
    <DataGrid label="Project budget processes" rows={[{ id: "purchase", name: "Item purchase", amount: savedPurchase }]} searchable={false} paginate={false} columns={[
      { key: "process", label: "Process", value: row => row.name },
      { key: "amount", label: "Budget allocation", value: row => moneyText(row.amount, currency) },
      { key: "items", label: "Purchase FMS items", value: () => String(ws.items.length) },
    ]} actions={() => <button className={button} disabled={busy} onClick={() => { setRates({}); setError(""); setShowItems(true); }}>{editable ? "Enter item unit prices" : "View purchase item budgets"}</button>} />
    {otherHeads.length > 0 && <DataGrid label="Other process budget allocations" rows={otherHeads} columns={[{ key: "name", label: "Other process", value: head => head.name }, { key: "amount", label: "Allocated budget", value: head => moneyText(decimalUnits(head.amount, 2) ?? BigInt(0), currency) }]} />}
    {showItems && <RecordDrawer wide extraWide title="Item purchase — budget unit prices" footer={<div className="space-y-3"><div role="status" aria-live="polite" className="grid grid-cols-2 gap-3 lg:grid-cols-4">{metric("Main project budget", ws.project.contract_value===null?null:main)}{metric("Purchase item total", total)}{metric("Other process allocations", other)}{metric("Remaining main budget", remaining)}</div><div className="flex justify-end gap-2"><button type="button" className={button} disabled={busy} onClick={()=>setShowItems(false)}>Cancel</button>{editable&&<button form={priceFormID} className="min-h-11 rounded-xl bg-blue-700 px-5 py-2 font-semibold text-white disabled:opacity-50" disabled={busy||remaining<BigInt(0)||rows.length===0||pendingQuantities}>{busy?"Saving…":"Save item budgets"}</button>}</div></div>} onClose={() => { if (!busy) setShowItems(false); }}>
      <ValidatedForm id={priceFormID} className="space-y-4" onSubmit={async event => {
        event.preventDefault();
        const form = event.currentTarget;
        const reason = String(new FormData(form).get("reason"));
        if (await command({ action: "price_items", reason, prices: rows.map(row => ({ bo_id: row.bo_id, line_id: row.line_id, bo_version: row.bo_version, quantity: row.quantity, changed: row.changed, unit_price: row.changed && row.rate !== "" ? row.rate : null })) }, form)) { setRates({}); setShowItems(false); }
      }}>
        <p className="text-sm text-slate-600">All current Purchase FMS BO items for this project are listed below. Quantities come from purchasing; enter the budget per unit. Blank prices remain unallocated, and zero is a valid rate. Amounts should include expected taxes and charges.</p>
        {pendingQuantities&&<p role="status" className="rounded-lg bg-amber-50 p-3 text-amber-900">Some BO quantities are pending. Complete their positive quantities in Purchase FMS, then reload this budget before saving item prices.</p>}
        <p className="text-sm font-semibold">{priced} of {rows.length} items budgeted · Line total = required quantity × unit price</p>
        {remaining < BigInt(0) && <p role="alert" className="rounded-lg bg-red-50 p-3 text-red-800">Allocation exceeds the main budget by {moneyText(-remaining, currency)}. Reduce unit prices before saving.</p>}
        {error && <p role="alert" className="rounded-lg bg-red-50 p-3 text-red-800">{error}</p>}
        <DataGrid label="Purchase item unit budgets" rows={rows} searchable={false} paginate={false} columns={[
          { key: "bo", label: "Purchase BO", value: row => row.bo_name },
          { key: "item", label: "Item required", value: row => `${row.item_snapshot.sku ?? ""} ${row.item_snapshot.description ?? "Item"}` },
          { key: "quantity", label: "Required quantity", value: row => row.quantity },
          { key: "uom", label: "UOM", value: row => row.item_snapshot.uom ?? "—" },
          { key: "rate", label: "Budget per unit", value: row => row.rate, render: row => <ValidatedLabel className="block min-w-40 text-sm">Unit price<ValidatedInput name={`prices.${rows.indexOf(row)}.unit_price`} aria-label={`Unit price ${row.item_snapshot.description ?? row.item_snapshot.sku ?? row.line_id}`} type="number" min="0" max="99999999999999.99" step="0.01" inputMode="decimal" className={input} disabled={!editable || busy} value={row.rate} onChange={event => setRates(current => ({ ...current, [row.id]: event.target.value }))} /><span className="text-xs text-slate-500">{row.allocation?.pricing_quantity && decimalUnits(row.allocation.pricing_quantity,4) !== decimalUnits(row.quantity,4) ? "Quantity changed — save a revision to update this item" : row.allocation && !row.allocation.unit_price && !row.changed ? "Existing total retained until you enter a rate" : ""}</span></ValidatedLabel> },
          { key: "amount", label: "Calculated item budget", value: row => row.amount === null ? "Not budgeted" : moneyText(row.amount, currency) },
        ]} />
        {editable && <><ValidatedLabel required className="block text-sm font-semibold">Change reason<ValidatedInput required name="reason" maxLength={2000} className={`${input} mt-1`} placeholder="Reason for this purchase budget" /></ValidatedLabel><p className="text-sm text-slate-600">Item budgets are optional. You can save partial pricing and approve the project budget.</p></>}
        {ws.can_write&&ws.record?.state==="approved"&&<button type="button" className={button} disabled={busy} onClick={()=>setAction("revise")}>Start budget revision</button>}{!editable && <p role="status" className="text-sm">{ws.record?.state === "approved" ? "Start a budget revision to change approved unit prices." : "Budget write access is required to enter prices."}</p>}
      </ValidatedForm>
    </RecordDrawer>}
    {action && <RecordDrawer centered title={action === "approve" ? "Approve budget" : "Start budget revision"} onClose={() => { if (!busy) setAction(null); }}><ValidatedForm className="space-y-4" onSubmit={async event => { event.preventDefault(); if (await command({ action, reason: String(new FormData(event.currentTarget).get("reason")) }, event.currentTarget)) setAction(null); }}><p>{action === "approve" ? "Approve this saved budget as the financial baseline. Delegated approval is final and records a mandatory Director notification." : "The previous approved budget remains effective while you price this revision."}</p>{error && <p role="alert" className="text-red-800">{error}</p>}<ValidatedLabel required>Reason<ValidatedInput required name="reason" maxLength={2000} className={input} /></ValidatedLabel><button className={button} disabled={busy}>Confirm {action}</button></ValidatedForm></RecordDrawer>}
    {editDraft&&<BudgetDraftEditor initial={ws.record?.data ?? {total:ws.project.contract_value ?? "0",currency,owner_id:ws.owners[0]?.id ?? "",notes:"",heads:[],allocations:[]}} owners={ws.owners} error={error} contractValue={ws.project.contract_value ?? "0"} onClose={()=>setEditDraft(false)} onReload={()=>{setEditDraft(false);setWs(null);setError("");setReload(v=>v+1)}} onSave={async(data,reason,form)=>command({action:"save",data,reason},form)}/>}
    <details className="rounded-xl border bg-white p-4"><summary className="cursor-pointer font-semibold">Budget change history ({ws.history.length})</summary>{ws.history.map(entry=><article key={entry.id} className="mt-3 border-t pt-3 text-sm"><div className="flex flex-wrap items-center gap-2"><UserIdentity id={entry.created_by_id} name={ws.owners.find(o=>o.id===entry.created_by_id)?.name ?? entry.created_by_id}/><span>{new Date(entry.created_at).toLocaleString()} · {entry.action}</span></div><p>{entry.reason}</p><details className="mt-2"><summary>View before and after</summary><div className="grid gap-3 md:grid-cols-2"><div><h4 className="font-semibold">Before</h4><pre className="max-h-80 overflow-auto rounded border p-2 text-xs">{JSON.stringify(entry.before,null,2)}</pre></div><div><h4 className="font-semibold">After</h4><pre className="max-h-80 overflow-auto rounded border p-2 text-xs">{JSON.stringify(entry.after,null,2)}</pre></div></div></details></article>)}</details>
  </section>;
}
