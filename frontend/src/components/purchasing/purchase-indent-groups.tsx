"use client";

import { useState } from "react";
import { createRecordId } from "@/lib/record-id";
import { DataGrid } from "@/components/ui/data-grid";
import { RecordDrawer } from "@/components/ui/record-drawer";
import { ValidatedForm, ValidatedInput, ValidatedLabel, ValidatedTextarea, ValidatedSelect } from "@/components/ui/validated-controls";
import type { BOLine, Supplier } from "./purchase-indent-workspace";

export type Invitation = { scope_revision?: number; line_ids?: string[]; vendor_id: string; sent_date: string; channel: string; dispatch_reference: string; vendor_name?: string };
export type IndentGroup = { id: string; name: string; line_ids: string[]; vendor_ids: string[]; invitations?: Invitation[]; scope_revision?: number };
export type GroupRecord = { version: number; state: "draft" | "grouped" | "created"; bo_version: number; groups: IndentGroup[]; source_lines: BOLine[]; number: string; selected_line_ids: string[] };
export type ItemVendorReport = { id: string; sku: string; item: string; quantity: string; uom: string; group: string; selected_vendors: {id: string; name: string}[]; invitations: Invitation[] };
export type GroupWorkspace = { record: GroupRecord | null; vendors: Supplier[]; can_write: boolean; can_manage_vendors: boolean; history: { action: string; reason: string; created_at: string }[]; item_report: ItemVendorReport[] };
export type GroupCommand = {
  action: "save" | "create" | "add_vendors" | "select_items" | "save_groups" | "set_vendors" | "invite" | "create_group" | "reset_item" | "add_items";
  groups?: IndentGroup[]; line_ids?: string[]; group_id?: string; line_id?: string; vendor_ids?: string[]; reason: string; selected_line_ids?: string[];
  vendor_id?: string; sent_date?: string; channel?: string; dispatch_reference?: string;
};
const button = "min-h-11 rounded-lg border border-slate-300 px-4 py-2 text-sm font-semibold disabled:opacity-50";
const input = "mt-1 block min-h-11 w-full rounded-lg border border-slate-300 px-3";
export type Stage = "items" | "vendors" | "report";

export function PurchaseIndentGroups({ lines, suppliers, record, canWrite, canManageVendors, canViewVendors, canViewRegister, busy, history, notice, report, stage, initialVendorGroup,onStageChange, onSave, onPreview }: {
  lines: BOLine[]; suppliers: Supplier[]; record: GroupRecord | null; canWrite: boolean; canManageVendors: boolean; canViewVendors: boolean; canViewRegister: boolean; busy: boolean;
  initialVendorGroup?:string;stage: Stage; onStageChange: (stage: Stage) => void;
  notice: string; history: GroupWorkspace["history"]; report: ItemVendorReport[];
  onSave: (command: GroupCommand, form?: HTMLFormElement) => Promise<boolean>; onPreview: (group: string) => void;
}) {
  const created = record?.state === "created";
  const groupsSaved = record?.state === "grouped" || created;
  const groups = record?.groups ?? [];
  const [selected, setSelected] = useState<string[]>([]);
  const [editing, setEditing] = useState<IndentGroup | null>(null);
  const [inspecting, setInspecting] = useState<{group: IndentGroup; kind: "items" | "vendors"} | null>(null);
  const [vendorGroup, setVendorGroup] = useState<IndentGroup | null>(()=>stage==="vendors"&&canManageVendors?groups.find(g=>g.id===initialVendorGroup)??null:null);
  const [vendorIds, setVendorIds] = useState<string[]>(()=>created?[]:vendorGroup?.vendor_ids??[]);
  const [reason, setReason] = useState("");
  const [adding, setAdding] = useState(false);
  const [targetGroup, setTargetGroup] = useState("");
  const [appendReason, setAppendReason] = useState("Client requested additional items");
  const retainedVendors = created && !!vendorGroup?.vendor_ids.length;
  const [nameError, setNameError] = useState("");
  const [vendorError, setVendorError] = useState("");
  const assigned = new Set(groups.flatMap(group => group.line_ids));
  const ready = groupsSaved && groups.length > 0 && groups.every(group => group.vendor_ids.filter(id => suppliers.some(supplier => supplier.id === id && supplier.is_active)).length >= 3);
  const itemColumns = [
    { key: "sku", label: "SKU & item", value: (line: BOLine) => `${line.item_snapshot.sku || ""} ${line.item_snapshot.description || "BO item"}`, render: (line: BOLine) => <div className="min-w-56 max-w-xl"><p className="break-all text-xs font-semibold text-blue-700">{line.item_snapshot.sku}</p><p className="mt-1 leading-relaxed">{line.item_snapshot.description || "BO item"}</p></div> },
    { key: "quantity", label: "Quantity", value: (line: BOLine) => `${line.quantity || "—"} ${line.item_snapshot.uom || ""}` },
    { key: "required_date", label: "Required date", value: (line: BOLine) => line.required_date || "—" },
  ];
  function newGroup() {
    const id = createRecordId();
    setEditing({id, name: `Group-${id.slice(0,8)}`, line_ids: selected, vendor_ids: []});
    setNameError("");
  }
  function chooseVendors(group: IndentGroup) {
    setVendorGroup(group); setVendorIds([...group.vendor_ids]); setVendorError(""); setReason("");
  }
  return <div className="indent-group-workspace space-y-4 p-4 sm:p-6">
    {stage !== "items" && <div className="rounded-xl border border-slate-200 bg-white p-4">
      <h2 className="text-lg font-bold">{stage === "vendors" ? "Allocate vendors to item groups" : "Item vendor invitation report"}</h2>
      <p className="mt-1 text-sm text-slate-600">{stage === "vendors" ? "Choose at least three active vendors for each group. Vendor choices save immediately. When every group is ready, the vendor indent register opens automatically." : "Review the vendors selected and actual dispatch recorded for each item."}</p>
      <p className="mt-3 text-sm font-semibold">{assigned.size} items grouped · {groups.length} groups · {created ? "Issued groups — additional items allowed" : groupsSaved ? "Groups saved — choose vendors" : "Grouping in progress"}</p>
    </div>}
    {stage === "items" && <>
      <DataGrid label="Purchase items" rows={lines} columns={[...itemColumns,
        {key:"group",label:"Group",value:(line:BOLine)=>groups.find(group=>group.line_ids.includes(line.id))?.name || "Ungrouped"},
      ]}
        toolbar={canWrite ? <div className="flex flex-wrap items-center gap-2"><button type="button" className={button} disabled={busy} onClick={() => setSelected(lines.filter(line=>!assigned.has(line.id)).map(line => line.id))}>Select all ungrouped</button><button type="button" className={`${button} bg-blue-600 text-white`} disabled={busy || !selected.length} onClick={newGroup}>Group ({selected.length})</button><button type="button" className={button} disabled={busy || !selected.length || !groups.length} onClick={()=>{setTargetGroup(groups[0]?.id||"");setAdding(true);}}>Add to existing group ({selected.length})</button></div> : null}
        actions={line => <div className="flex items-center gap-3"><ValidatedLabel className="indent-selection"><ValidatedInput type="checkbox" aria-label={`Select item ${line.item_snapshot.sku || line.id}`} disabled={!canWrite || busy || assigned.has(line.id)} checked={assigned.has(line.id) || selected.includes(line.id)} onChange={event => setSelected(event.target.checked ? [...selected, line.id] : selected.filter(id => id!==line.id))} />{assigned.has(line.id) ? "Grouped" : "Select"}</ValidatedLabel>{assigned.has(line.id) && canWrite && <button type="button" className={button} aria-label={`Reset item ${line.item_snapshot.sku || line.id}`} disabled={busy || created} onClick={()=>void onSave({action:"reset_item",line_id:line.id,reason:"Reset item for regrouping"})}>Reset</button>}</div>} />
      {groups.length > 0 && canViewVendors && <button type="button" className={button} disabled={busy} onClick={()=>onStageChange("vendors")}>Continue to group vendors</button>}
      <p className="text-sm text-slate-600">{created ? "Issued items stay grouped. Add new items to create a revised scope; earlier indents and quotations remain in history." : "Grouped items stay checked and disabled. Use Reset to move an item into another group."}</p>
    </>}
    {stage === "vendors" && <>
      {!canManageVendors && <p role="status" className="text-sm text-slate-600">Read-only access. Vendor allocation requires permission.</p>}
      <DataGrid label="Group vendors" rows={groups} columns={[
        {key:"name",label:"Group",value:group=>group.name},
        {key:"items",label:"Items",value:group=>group.line_ids.length,render:group=><button type="button" className={`${button} whitespace-nowrap text-blue-700`} aria-label={`View items in ${group.name}`} onClick={()=>setInspecting({group,kind:"items"})}>View items ({group.line_ids.length})</button>},
        {key:"vendors",label:"Selected vendors",value:group=>group.vendor_ids.map(id=>suppliers.find(supplier=>supplier.id===id)?.name || "Unavailable vendor").join(", ") || "No vendors selected",render:group=><button type="button" className={`${button} whitespace-nowrap text-blue-700`} aria-label={`View vendors in ${group.name}`} onClick={()=>setInspecting({group,kind:"vendors"})}>View vendors ({group.vendor_ids.length})</button>},
        {key:"invitations",label:"Recorded invitations",value:group=>group.invitations?.length ?? 0},
      ]} actions={group=><div className="flex flex-wrap gap-2">{canManageVendors && <button type="button" className={button} disabled={busy} onClick={()=>chooseVendors(group)}>{created && group.vendor_ids.length ? "Add vendors" : "Choose vendors"}</button>}{created && canViewRegister && <><button type="button" className={button} onClick={()=>onPreview(group.id)}>Generate indent</button></>}</div>} />
      {ready && canViewRegister && <button type="button" className={`${button} bg-blue-600 text-white`} disabled={busy || !ready} onClick={()=>onPreview(groups[0].id)}>Continue to vendor indent register</button>}
      <p className="text-sm text-slate-600">Vendor choices are saved per group. Generate vendor indents to print them and record actual sent dates. You can add vendors later.</p>
    </>}
    {stage === "report" && <>
      <p className="text-sm text-slate-600">Each item inherits the vendors of its saved group. “Invited” means dispatch has been recorded; selection alone does not mark a vendor as invited.</p>
      <DataGrid label="Item vendor invitation report" rows={report} columns={[
        {key:"sku",label:"SKU",value:row=>row.sku}, {key:"item",label:"Item",value:row=>row.item},
        {key:"quantity",label:"Quantity",value:row=>`${row.quantity || "—"} ${row.uom}`}, {key:"group",label:"Group",value:row=>row.group},
        {key:"selected",label:"Selected vendors",value:row=>row.selected_vendors.map(v=>v.name).join(", ") || "None"},
        {key:"invited",label:"Invited vendors / dispatch",value:row=>row.invitations.map(inv=>`${inv.vendor_name} · ${inv.sent_date} · ${inv.channel} · ${inv.dispatch_reference}`).join("; ") || "No invitation recorded"},
      ]} actions={()=>null} />
    </>}
    {history.length>0 && <details className="rounded-xl border border-slate-200 bg-white p-4"><summary className="cursor-pointer font-semibold">Saved change history ({history.length})</summary><ol className="mt-3 space-y-2 text-sm">{history.map((event,index)=><li key={index}>{new Date(event.created_at).toLocaleString()} · {event.action} · {event.reason}</li>)}</ol></details>}
    {inspecting && <RecordDrawer centered title={`${inspecting.kind === "items" ? "Items" : "Selected vendors"} — ${inspecting.group.name}`} onClose={()=>setInspecting(null)}>
      <p className="text-sm text-slate-600">{inspecting.kind === "items" ? `${inspecting.group.line_ids.length} items in this saved group.` : `${inspecting.group.vendor_ids.length} selected vendors. Invitations appear only when dispatch is recorded.`}</p>
      {inspecting.kind === "items" ? <ul aria-label="Group item details" className="space-y-3">{inspecting.group.line_ids.map(id=>{
        const line=lines.find(item=>item.id===id);
        return <li key={id} className="rounded-lg border border-slate-200 bg-white p-3">{line ? <><p className="break-all text-xs font-semibold text-blue-700">{line.item_snapshot.sku || "SKU unavailable"}</p><p className="mt-1 font-semibold">{line.item_snapshot.description || "BO item"}</p><dl className="mt-3 grid grid-cols-2 gap-2 text-sm"><div><dt className="text-slate-500">Quantity</dt><dd>{line.quantity || "—"} {line.item_snapshot.uom}</dd></div><div><dt className="text-slate-500">Required date</dt><dd>{line.required_date || "—"}</dd></div></dl></> : <p>Item details unavailable. Reload the BO list to review.</p>}</li>;
      })}</ul> : inspecting.group.vendor_ids.length ? <ul aria-label="Group vendor details" className="space-y-3">{inspecting.group.vendor_ids.map(id=>{
        const vendor=suppliers.find(supplier=>supplier.id===id);
        const invitation=inspecting.group.invitations?.find(entry=>entry.vendor_id===id&&(entry.scope_revision??1)===(inspecting.group.scope_revision??1));
        return <li key={id} className="rounded-lg border border-slate-200 bg-white p-3"><p className="font-semibold">{vendor?.name || "Unavailable vendor"}</p><p className="mt-1 text-sm text-slate-600">{vendor?.code || "Code unavailable"} · {vendor?.is_active ? "Active" : "Inactive / unavailable"}</p><p className="mt-3 break-words text-sm">{invitation ? `Invited: ${invitation.sent_date} · ${invitation.channel} · ${invitation.dispatch_reference}` : "No invitation recorded"}</p></li>;
      })}</ul> : <p className="rounded-lg border border-slate-200 bg-white p-3 text-sm">No vendors selected for this group yet.</p>}
      <div className="flex justify-end"><button type="button" className={button} onClick={()=>setInspecting(null)}>Close</button></div>
    </RecordDrawer>}
    {adding && <RecordDrawer centered title="Add items to existing group" onClose={()=>{if(!busy)setAdding(false);}}>
      {notice && <p role="alert" className="text-sm text-red-700">{notice}</p>}
      <ValidatedForm className="space-y-4" onSubmit={async event=>{event.preventDefault();if(await onSave({action:"add_items",group_id:targetGroup,line_ids:selected,reason:appendReason},event.currentTarget)){setAdding(false);setSelected([]);}}}>
        <p>{selected.length} selected items will join this group and inherit its selected vendors.</p>
        {created && <p className="rounded-lg bg-amber-50 p-3 text-sm">A revised vendor indent must be generated and sent for the expanded scope. Earlier documents and quotations remain unchanged.</p>}
        <ValidatedLabel className="block font-semibold">Existing group<ValidatedSelect required name="group_id" value={targetGroup} onChange={event=>setTargetGroup(event.target.value)} className={input}><option value="">Choose a group</option>{groups.map(group=><option key={group.id} value={group.id}>{group.name} ({group.line_ids.length} items, {group.vendor_ids.length} vendors)</option>)}</ValidatedSelect></ValidatedLabel>
        <ValidatedLabel className="block font-semibold">Reason<ValidatedTextarea required name="reason" maxLength={2000} value={appendReason} onChange={event=>setAppendReason(event.target.value)} className={input}/></ValidatedLabel>
        <button disabled={busy} className={`${button} bg-blue-600 text-white`}>{busy?"Saving…":"Save items to group"}</button>
      </ValidatedForm>
    </RecordDrawer>}
    {editing && <RecordDrawer centered title="Name item group" onClose={()=>{if(!busy)setEditing(null);}}>
      {notice && <p role="alert" className="text-sm text-red-700">{notice}</p>}
      <ValidatedForm className="space-y-5" onSubmit={async event=>{
        event.preventDefault();
        if(groups.some(group=>group.name.trim().toLowerCase()===editing.name.trim().toLowerCase())) {setNameError("Use a distinct group name.");event.currentTarget.querySelector<HTMLInputElement>('[name="name"]')?.focus();return;}
        if(await onSave({action:"create_group",groups:[editing],reason:`Create item group ${editing.name}`},event.currentTarget)){setEditing(null);setSelected([]);}
      }}>
        <p className="text-sm text-slate-600">{editing.line_ids.length} selected items will be saved in this group.</p>
        <ValidatedLabel className="block text-sm font-semibold">Group name<ValidatedInput autoFocus required name="name" maxLength={128} value={editing.name} error={nameError} onChange={event=>{setNameError("");setEditing({...editing,name:event.target.value});}} className={input} /></ValidatedLabel>
        <div className="flex justify-end gap-2"><button type="button" disabled={busy} className={button} onClick={()=>setEditing(null)}>Cancel</button><button type="submit" disabled={busy} className={`${button} bg-blue-600 text-white`}>{busy ? "Saving…" : "Save group"}</button></div>
      </ValidatedForm>
    </RecordDrawer>}
    {vendorGroup && <RecordDrawer title={`${retainedVendors ? "Add vendors" : "Choose vendors"} — ${vendorGroup.name}`} onClose={()=>{if(!busy)setVendorGroup(null);}}>
      {notice && <p role="alert" className="text-sm text-red-700">{notice}</p>}
      <ValidatedForm className="space-y-4" onSubmit={async event=>{
        event.preventDefault();
        const additions=vendorIds.filter(id=>!vendorGroup.vendor_ids.includes(id));
        if(retainedVendors ? !additions.length : vendorIds.filter(id=>suppliers.some(s=>s.id===id && s.is_active)).length<3) {setVendorError(retainedVendors ? "Select an additional vendor." : "Select at least three distinct active vendors.");event.currentTarget.querySelector<HTMLInputElement>('[name="vendor_selection"]')?.focus();return;}
        if(await onSave({action:retainedVendors ? "add_vendors" : "set_vendors",group_id:vendorGroup.id,vendor_ids:retainedVendors ? additions : vendorIds,reason:retainedVendors ? reason : `Save vendors for ${vendorGroup.name}`},event.currentTarget))setVendorGroup(null);
      }}>
        <fieldset aria-describedby={vendorError ? "vendor-selection-error" : undefined}>
          <legend className="font-semibold">{retainedVendors ? "Additional vendors" : "At least three vendors"} <span className="text-red-600">*</span></legend>
          <div className="mt-2 max-h-80 overflow-y-auto rounded-lg border border-slate-200 p-3">{suppliers.map(supplier=><ValidatedLabel key={supplier.id} className="indent-selection flex min-h-11 items-center gap-3 text-sm"><ValidatedInput type="checkbox" name="vendor_selection" disabled={(retainedVendors && vendorGroup.vendor_ids.includes(supplier.id)) || (!supplier.is_active && !vendorIds.includes(supplier.id))} checked={vendorIds.includes(supplier.id)} onChange={event=>{setVendorError("");setVendorIds(event.target.checked ? [...vendorIds,supplier.id] : vendorIds.filter(id=>id!==supplier.id));}} />{supplier.name} ({supplier.code}){!supplier.is_active && " — inactive"}</ValidatedLabel>)}</div>
          {vendorError && <p id="vendor-selection-error" role="alert" className="text-sm text-red-600">{vendorError}</p>}
        </fieldset>
        {retainedVendors && <ValidatedLabel className="block font-semibold">Reason for adding vendors<ValidatedTextarea required name="reason" maxLength={2000} value={reason} onChange={event=>setReason(event.target.value)} className={input} /></ValidatedLabel>}
        <button type="submit" disabled={busy} className={`${button} bg-blue-600 text-white`}>{busy ? "Saving…" : "Save group vendors"}</button>
      </ValidatedForm>
    </RecordDrawer>}

  </div>;
}
