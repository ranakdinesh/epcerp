"use client";

import "./indent-document.css";
import { indentApprovalLabels, indentFields, indentSignatureNotice, readIndentContent, validateIndentContent, type IndentContent } from "@/lib/indent-content";
import { IndentParagraphs, IndentRichEditor } from "./indent-rich-editor";
import { CategoryThumbnail } from "@/components/items/category-thumbnail";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useEffect, useState, type FormEvent } from "react";
import { DataGrid } from "@/components/ui/data-grid";
import { DocumentPDFViewer } from "@/components/documents/document-pdf-viewer";
import { RecordDrawer } from "@/components/ui/record-drawer";
import { PurchaseSectionHeader } from "./purchase-layout";
import { purchaseProjectHref, read } from "./data";
import { purchaseCategory } from "./purchase-items-model";

type Item = { id: string; category: string | null; category_name: string | null; sku: string | null; description: string | null; quantity: string; unit: string | null; required_date: string };
type Draft = { category: string; version: number; terms: string; vendorIds: string[] };
type DispatchRow = { id: string; version: number; category: string; group: string; vendor: string; number: string; itemCount: number; generatedDate: string; sentDate: string | null };
type Workspace = { items: Item[]; drafts: Draft[]; details: { client: string; site: string }; can_edit: boolean; vendors_ready: boolean; vendor_counts: Record<string, number>; dispatches: DispatchRow[]; can_dispatch: boolean; dispatch_complete: boolean; reissue_required: boolean };
const action = "app-action inline-flex min-h-11 items-center rounded-lg px-4 font-semibold disabled:opacity-50";

export function TypeIndents({ project, projectName }: { project: string; projectName: string }) {
  const params = useSearchParams();
  const router = useRouter();
  const category = params.get("indentType");
  const view = params.get("indentView");
  const path = `/api/projects/${encodeURIComponent(project)}/purchase-indents/by-type`;
  const dataPath = view === "dispatch" ? `${path}/dispatch` : path;
  const [data, setData] = useState<Workspace | null>(null);
  const [error, setError] = useState("");
  useEffect(() => {
    const controller = new AbortController();
    void read<Workspace>(dataPath, controller.signal).then(value => {
      if (!controller.signal.aborted) setData(value);
    }).catch(error => {
      if (!controller.signal.aborted) setError(error.message);
    });
    return () => controller.abort();
  }, [dataPath]);
  function href(code?: string, view?: string) {
    const next = new URLSearchParams(params.toString());
    next.delete("editTemplate");
    if (view) next.set("indentView", view);
    else next.delete("indentView");
    if (code) next.set("indentType", code);
    else next.delete("indentType");
    return `/dashboard?${next}`;
  }
  if (error) return <p role="alert" className="text-red-700">{error}</p>;
  if (!data) return <p role="status">Loading indent lists…</p>;

  if (category && view === "dispatch") return <IndentDispatch key={category} path={path}
    rows={data.dispatches.filter(row => row.category === category)} canDispatch={data.can_dispatch} complete={data.dispatch_complete}
    listHref={href()} quotationHref={purchaseProjectHref(project, "PUR-03")}
    onRecorded={() => read<Workspace>(dataPath).then(value => {
      setData(value);
      window.dispatchEvent(new Event("purchase-progress-saved"));
    })} />;

  if (!data.vendors_ready) return <section className="space-y-4">
    <PurchaseSectionHeader title="Purchase Indent" />
    <p role="status" className="rounded-xl bg-amber-50 p-5">
      Save vendor assignments for every finalised item before previewing an indent.{" "}
      <Link className="font-semibold underline" href={purchaseProjectHref(project, "PUR-VENDORS")}>Open Choose vendors</Link>
    </p>
  </section>;

  const grouped = new Map<string, Item[]>();
  for (const item of data.items) {
    const code = item.category || "unclassified";
    const items = grouped.get(code) ?? [];
    items.push(item);
    grouped.set(code, items);
  }
  const selectedItems = category ? grouped.get(category) ?? [] : [];
  if (category && category !== "unclassified" && selectedItems.length) {
    if (data.dispatches.some(row => row.category === category)) return <section className="space-y-4">
      <PurchaseSectionHeader title={`${purchaseCategory(category)} · Finalised indent`} />
      <Link className={action} href={href(category, "dispatch")}>Open vendor dispatch</Link>
    </section>;
    const draft = data.drafts.find(row => row.category === category);
    return <IndentWorkspace key={category} path={path} projectName={projectName}
      details={data.details} category={category} draft={draft} items={selectedItems}
      view={view} links={{ list: href(), draft: href(category), items: href(category, "items") }}
      editable={data.can_edit} reissueRequired={data.reissue_required} onSaved={async () => {
        const updated = await read<Workspace>(path);
        setData(updated);
        router.push(href(category, "dispatch"));
      }} />;
  }
  const rows = [...grouped].map(([id, items]) => ({
    id, name: id === "unclassified" ? purchaseCategory() : items[0].category_name && !["CAB", "PAN", "LED"].includes(id) ? items[0].category_name : purchaseCategory(id), count: items.length,
    vendorCount: data.vendor_counts[id] ?? 0,
    finalised: data.dispatches.some(row => row.category === id),
  }));
  return <section className="space-y-4">
    <PurchaseSectionHeader title="Purchase Indent" />
    <DataGrid label="Purchase indents" rows={rows} initialSort="type" columns={[
      { key: "type", label: "Item type", value: row => row.name, render: row =>
        <div className="flex items-center gap-3"><CategoryThumbnail code={row.id === "unclassified" ? null : row.id} /><strong>{row.name}</strong></div> },
      { key: "count", label: "Finalised items", value: row => row.count },
      { key: "vendors", label: "Vendors selected", value: row => row.vendorCount },
    ]} actions={row => row.id !== "unclassified" ? <Link href={href(row.id, row.finalised ? "dispatch" : undefined)} className={action}>{row.finalised ? "Vendor dispatch" : "Preview indent"}</Link> : null} />
    {!rows.length && <p>No finalised items are available for indent preparation.</p>}
    {rows.some(row => row.id === "unclassified") && <p role="status" className="rounded-lg bg-yellow-50 p-3 text-yellow-900">
      Assign an item type to unclassified items before preparing their indent.
    </p>}
  </section>;
}

function IndentWorkspace({ path, projectName, details, category, draft, items, view, links, editable, reissueRequired, onSaved }: {
  path: string; projectName: string; details: Workspace["details"]; category: string;
  draft?: Draft; items: Item[]; view: string | null;
  links: { list: string; draft: string; items: string }; editable: boolean; reissueRequired: boolean; onSaved: () => Promise<void>;
}) {
  const [content, setContent] = useState(() => readIndentContent(draft?.terms ?? ""));
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    if (!validateIndentContent(content)) {
      setError("Enter specifications and keep the total content within 24,000 characters.");
      return;
    }
    const reissue = new FormData(event.currentTarget).has("reissue");
    setBusy(true);
    setError("");
    try {
      const response = await fetch(path, { method: "POST", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ category, content, version: draft?.version ?? 0, reissue }) });
      const value = await response.json();
      if (!response.ok) throw new Error(typeof value.detail === "string" ? value.detail : value.detail?.[0]?.msg ?? "The indent could not be saved.");
      await onSaved();
    } catch (error) {
      setError(error instanceof Error ? error.message : "The indent could not be saved.");
    } finally {
      setBusy(false);
    }
  }
  const typeName = purchaseCategory(category);
  if (view === "items") return <section className="space-y-4">
    <PurchaseSectionHeader title={`${typeName} · Indent items`}>
      <Link href={links.draft} className={action}>Back to indent</Link>
    </PurchaseSectionHeader>
    <DataGrid label="Indent items" rows={items} initialSort="sku" columns={[
        { key: "sku", label: "SKU", value: item => item.sku ?? "" },
        { key: "description", label: "Technical description", value: item => item.description ?? "" },
        { key: "quantity", label: "Quantity", value: item => item.quantity },
        { key: "unit", label: "Unit", value: item => item.unit ?? "" },
        { key: "date", label: "Required at site", value: item => item.required_date ?? "" },
      ]} />
  </section>;

  return <section className="space-y-4">
    <PurchaseSectionHeader title={`${typeName} · Preview indent`}>
      <Link href={links.list} className={action}>Back to type lists</Link>
      <Link href={links.items} className={action}>View items</Link>
    </PurchaseSectionHeader>
    {draft && <p className="text-sm">Draft version {draft.version}</p>}
    <div className="grid items-start gap-5 xl:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
      <IndentHTMLPreview typeName={typeName} projectName={projectName} details={details} items={items} content={content} />
      <form onSubmit={save} className="space-y-3 rounded-xl border border-teal-200 p-4">
        {editable && indentFields.map(([key, label]) => <IndentRichEditor key={key} label={label} initial={content[key]} spacing={content.spacing?.[key]}
          onSpacingChange={spacing => setContent(current => ({ ...current, spacing: { ...current.spacing, [key]: spacing } }))}
          onChange={paragraphs => setContent(current => ({ ...current, [key]: paragraphs }))} />)}
        {error && <p role="alert" className="text-red-700">{error}</p>}
        {editable && reissueRequired && <label className="flex items-start gap-2 rounded-lg bg-amber-50 p-3">
          <input className="mt-1" name="reissue" type="checkbox" required />
          <span>Reissue indents using the current vendor assignments. Earlier dispatches stay in the history. Send the new indents before entering quotations.</span>
        </label>}
        {editable && <div className="flex justify-end">
          <button className={action} disabled={busy} type="submit">{busy ? "Saving…" : "Finalise and open vendor dispatch"}</button>
        </div>}
      </form>
    </div>
  </section>;
}


function IndentDispatch({ path, rows, canDispatch, complete, listHref, quotationHref, onRecorded }: {
  path: string; rows: DispatchRow[]; canDispatch: boolean; complete: boolean; listHref: string; quotationHref: string;
  onRecorded: () => Promise<void>;
}) {
  const [recording, setRecording] = useState<DispatchRow | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [preview, setPreview] = useState<DispatchRow | null>(null);
  const pdf = (row: DispatchRow) => `${path}/pdf?document=${encodeURIComponent(row.id)}`;

  async function record(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!recording || busy) return;
    const fields = Object.fromEntries(new FormData(event.currentTarget));
    setBusy(true);
    setError("");
    try {
      const response = await fetch(`${path}/dispatch`, {
        method: "POST", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ...fields, document_id: recording.id, version: recording.version }),
      });
      const result = await response.json();
      if (!response.ok) throw new Error(typeof result.detail === "string" ? result.detail : result.detail?.[0]?.msg ?? "Dispatch could not be recorded.");
      setRecording(null);
      await onRecorded();
    } catch (error) {
      setError(error instanceof Error ? error.message : "Dispatch could not be recorded.");
    } finally {
      setBusy(false);
    }
  }

  return <section className="space-y-4">
    <PurchaseSectionHeader title="Vendor dispatch">
      <Link href={listHref} className={action}>Back to purchase indents</Link>
    </PurchaseSectionHeader>
    <p>Download the indent for each vendor. After sending it, record the date, channel and reference.</p>
    {error && !recording && <p role="alert" className="text-red-700">{error}</p>}
    <DataGrid label="Vendor indents" rows={rows} initialSort="group" columns={[
      { key: "group", label: "Vendor set", value: row => row.group },
      { key: "vendor", label: "Vendor", value: row => row.vendor },
      { key: "items", label: "Items", value: row => row.itemCount },
      { key: "sent", label: "Dispatch", value: row => row.sentDate ?? "Awaiting dispatch" },
    ]} actions={row => <div className="flex flex-wrap justify-end gap-2">
      <button type="button" className={action} onClick={() => setPreview(row)}>View PDF</button>
      {canDispatch && !row.sentDate && <button type="button" className={action} onClick={() => {
        setError("");
        setRecording(row);
      }}>Record dispatch</button>}
    </div>} />
    {complete && <div className="flex justify-end"><a className={action} href={quotationHref}>Receive vendor quotations</a></div>}
    {preview && <RecordDrawer title="Purchase indent PDF" wide extraWide onClose={() => setPreview(null)}>
      <DocumentPDFViewer url={pdf(preview)} filename={`${preview.number}.pdf`} />
    </RecordDrawer>}
    {recording && <RecordDrawer title={`Record dispatch · ${recording.vendor}`} onClose={() => { if (!busy) setRecording(null); }}>
      <form onSubmit={record} className="space-y-4">
        <label className="block">Date sent
          <input className="mt-1 block w-full rounded-lg border p-2" type="date" name="sent_date" min={recording.generatedDate.slice(0, 10)} required />
        </label>
        <label className="block">Channel
          <select className="mt-1 block w-full rounded-lg border p-2" name="channel" defaultValue="" required>
            <option value="" disabled>Choose channel</option>
            <option>Email</option><option>Vendor portal</option><option>Courier</option><option>Other</option>
          </select>
        </label>
        <label className="block">Dispatch reference
          <input className="mt-1 block w-full rounded-lg border p-2" name="dispatch_reference" maxLength={500} required />
        </label>
        {error && <p role="alert" className="text-red-700">{error}</p>}
        <div className="flex justify-end">
          <button className={action} type="submit" disabled={busy}>{busy ? "Saving…" : "Save dispatch"}</button>
        </div>
      </form>
    </RecordDrawer>}
  </section>;
}

function IndentHTMLPreview({ typeName, projectName, details, items, content }: {
  typeName: string; projectName: string; details: Workspace["details"]; items: Item[]; content: IndentContent;
}) {
  const projectFields = [
    ["Name of work", projectName],
    ["Client", details.client],
    ["Delivery site", details.site || "To confirm"],
  ];
  return <article aria-label="Indent preview" className="indent-document">
    <header className="indent-document-title">
      <h2>Purchase Indent</h2>
      <p>{typeName}</p>
    </header>
    <table className="indent-document-table">
      <caption className="sr-only">Project details</caption>
      <tbody>
        {projectFields.map(([label, value]) => <tr key={label}>
          <th scope="row" className="indent-document-label">{label}</th>
          <td>{value}</td>
        </tr>)}
      </tbody>
    </table>
    <table className="indent-document-table indent-document-items">
      <caption className="sr-only">Indent items</caption>
      <thead>
        <tr>
          <th scope="col">No.</th>
          <th scope="col">Description</th>
          <th scope="col">Unit</th>
          <th scope="col">Qty.</th>
        </tr>
      </thead>
      <tbody>
        <tr>
          <td colSpan={4} className="indent-document-specifications">
            <h3 className="font-bold">Specifications</h3>
            <IndentParagraphs paragraphs={content.specifications} spacing={content.spacing?.specifications} />
          </td>
        </tr>
        {items.map((item, index) => <tr key={item.id}>
          <td>{index + 1}</td>
          <td>{item.description ?? item.sku}</td>
          <td>{item.unit}</td>
          <td>{item.quantity}</td>
        </tr>)}
      </tbody>
    </table>
    {indentFields.slice(1).map(([key, label]) => content[key].some(p => p.some(run => run.text.trim())) &&
      <section key={key} className="mt-4 space-y-1">
        <h3 className="font-bold">{label}</h3>
        <IndentParagraphs paragraphs={content[key]} spacing={content.spacing?.[key]} />
      </section>)}
    <footer className="mt-6 border-t pt-3">
      <div className="grid grid-cols-3 gap-2 font-bold [&>:nth-child(2)]:text-center [&>:last-child]:text-right">
        {indentApprovalLabels.map(label => <span key={label}>{label}</span>)}
      </div>
      <p className="mt-2 text-center">{indentSignatureNotice}</p>
    </footer>
  </article>;
}
