"use client";

import { ValidatedLabel, ValidatedInput } from "@/components/ui/validated-controls";

import "./choose-vendors.css";
import { CategoryThumbnail } from "@/components/items/category-thumbnail";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { DataGrid } from "@/components/ui/data-grid";
import { RecordDrawer } from "@/components/ui/record-drawer";
import { PurchaseSectionHeader } from "./purchase-layout";
import { purchaseProjectHref, read } from "./data";

import { purchaseCategory, purchaseItemType } from "./purchase-items-model";

type Vendor = { id: string; code: string; name: string };

type Item = { id: string; category: string | null; category_name: string | null; description: string | null; sku: string | null };
type ItemSet = { category: string; itemIds: string[]; vendorIds: string[] };
type Selection = { itemSets: ItemSet[] | null; version: number };
type Workspace = { selection?: Selection; vendors: Vendor[]; items: Item[]; can_edit: boolean };
const action = "app-action inline-flex min-h-11 items-center rounded-lg px-4 font-semibold disabled:opacity-50";
const endpoint = (project: string) => `/api/projects/${encodeURIComponent(project)}/purchase-indents/project-vendors`;

const itemName = (item: Item) => purchaseItemType({ item_snapshot: { description: item.description ?? undefined } });
const itemSpecs = (item: Item) => (item.description ?? "").split(/[·\n]/).slice(1)
  .map(spec => spec.trim()).filter(spec => spec && spec !== item.sku?.trim());
const identifyingSpecs = (item: Item) => itemSpecs(item)
  .filter(spec => /^(Wattage|Size|Number of Cores|Voltage Grade):/i.test(spec)).join(" · ");

export function ChooseVendors({ project }: { project: string }) {
  const [data, setData] = useState<Workspace>();
  const [error, setError] = useState("");
  useEffect(() => {
    const controller = new AbortController();
    void read<Workspace>(endpoint(project), controller.signal).then(value => {
      if (!controller.signal.aborted) setData(value);
    }).catch(error => {
      if (!controller.signal.aborted) setError(error.message);
    });
    return () => controller.abort();
  }, [project]);
  if (error) return <p role="alert" className="text-red-700">{error}</p>;
  if (!data) return <p role="status">Loading vendors…</p>;
  return <VendorSelection project={project} data={data} />;
}

function VendorSelection({ project, data }: { project: string; data: Workspace }) {
  const router = useRouter();
  const indentHref = purchaseProjectHref(project, "PUR-02");
  const { vendors, items, can_edit: editable } = data;
  const [selection, setSelection] = useState(data.selection);
  const [sets, setSets] = useState(data.selection?.itemSets ?? []);
  const [unchecked, setUnchecked] = useState<string[]>([]);
  const [picker, setPicker] = useState<{ category: string; itemIds: string[]; vendorIds: string[] }>();
  const [viewedSet, setViewedSet] = useState<ItemSet>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const vendorsById = new Map(vendors.map(vendor => [vendor.id, vendor]));
  const itemsById = new Map(items.map(item => [item.id, item]));
  const categories = new Map<string, { id: string; name: string; items: Item[] }>();
  for (const item of items) {
    const id = item.category ?? "";
    const row = categories.get(id) ?? { id, name: item.category_name ?? purchaseCategory(id), items: [] };
    row.items.push(item);
    categories.set(id, row);
  }
  const assigned = new Set(sets.flatMap(set => set.itemIds.filter(id => itemsById.get(id)?.category === set.category)));
  const pending = items.filter(item => !assigned.has(item.id));
  const valid = items.length > 0 && !pending.length && sets.every(set =>
    set.itemIds.every(id => itemsById.get(id)?.category === set.category) &&
    set.vendorIds.every(id => vendorsById.has(id)));
  const dirty = sets !== selection?.itemSets;
  const usedVendors = new Set(sets.filter(set => set.category === picker?.category).flatMap(set => set.vendorIds));
  const available = vendors.filter(vendor => !usedVendors.has(vendor.id));

  async function save() {
    setBusy(true);
    setError("");
    try {
      const response = await fetch(endpoint(project), { method: "POST", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ itemSets: sets, version: selection?.version ?? 0 }) });
      const value = await response.json();
      if (!response.ok) throw new Error(value.detail ?? "Could not save vendors.");
      setSelection(value);
      setSets(value.itemSets);
      router.push(indentHref);
    } catch (error) {
      setError(error instanceof Error ? error.message : "Could not save vendors.");
    } finally {
      setBusy(false);
    }
  }
  function applyVendors() {
    if (!picker || picker.vendorIds.length < 3) return;
    setSets(current => [...current, picker]);
    setPicker(undefined);
  }
  return <section className="choose-vendors space-y-4">
    <PurchaseSectionHeader title="Choose vendors">
      {valid && !dirty && <Link href={indentHref} className={action}>Continue to purchase indent</Link>}
    </PurchaseSectionHeader>
    {pending.length > 0 && <p role="status" className="text-sm text-yellow-900">{pending.length} items still need vendors.</p>}
    {categories.has("") && <p role="status" className="text-sm text-yellow-900">Assign a category to unclassified items before choosing vendors.</p>}
    {error && <p role="alert" className="text-red-700">{error}</p>}
    <div className="vendor-assignment-grid">
    <DataGrid label="Vendor selection by category" rows={items} initialSort="item" paginate={false}
      grouping={{
        columnKey: "category",
        key: item => item.category ?? "",
        label: item => categories.get(item.category ?? "")?.name ?? "Unclassified",
        spacerRow: true,
        header: item => {
          const category = categories.get(item.category ?? "")!;
          const remaining = category.items.filter(line => !assigned.has(line.id));
          const itemIds = remaining.filter(line => !unchecked.includes(line.id)).map(line => line.id);
          return <div className="flex flex-wrap items-center gap-x-5 gap-y-2">
              <span className="text-base font-bold">{category.name}</span>
              <span className="rounded-full border border-teal-600/20 px-3 py-1 text-xs font-medium">
                {category.items.length - remaining.length} of {category.items.length} assigned
                {editable && remaining.length > 0 && ` · ${itemIds.length} checked`}
              </span>
              {editable && category.id && remaining.length > 0 &&
                <button type="button" className={`${action} ml-auto`} disabled={busy || !itemIds.length}
                  aria-label={`Add vendors for ${category.name}`}
                  onClick={() => setPicker({ category: category.id, itemIds, vendorIds: [] })}>Add vendors</button>}
            </div>;
        },
      }} columns={[
        { key: "item", label: "Item / selection", value: itemName, render: item => {
          const assignment = sets.filter(set => set.category === item.category).findIndex(set => set.itemIds.includes(item.id));
          if (assignment >= 0) return <div className="flex items-start gap-2 [overflow-wrap:anywhere]">
            <CategoryThumbnail code={item.category} />
            <div>
            <p className="font-semibold">{itemName(item)}</p>
            <p className="text-xs">Assigned · Set {assignment + 1}</p>
            </div>
          </div>;
          return <ValidatedLabel className="flex items-start gap-[5px] font-semibold [overflow-wrap:anywhere]">
            <ValidatedInput type="checkbox" className="mt-1 h-4 w-4 shrink-0" checked={!unchecked.includes(item.id)}
              aria-label={`Select ${itemName(item)} ${identifyingSpecs(item) || item.sku || ""}`.trim()}
              disabled={!editable || busy || !item.category} onChange={event => {
                const checked = event.target.checked;
                setUnchecked(current => checked ? current.filter(id => id !== item.id) : [...current, item.id]);
              }} />
            <CategoryThumbnail code={item.category} />
            <span className="min-w-0">{itemName(item)}</span>
          </ValidatedLabel>;
        } },
        { key: "sku", label: "SKU", value: item => item.sku ?? "", className: "text-xs", render: item => <div className="[overflow-wrap:anywhere]">{item.sku}</div> },
        { key: "specifications", label: "Specifications", value: item => itemSpecs(item).join(" "), sortable: false,
          className: "text-xs [overflow-wrap:anywhere]", render: item => <ul className="space-y-1">
            {itemSpecs(item).map((specification, index) => {
              const separator = specification.indexOf(": ");
              return separator < 0 ? <li key={index}><span className="vendor-spec-make">{specification}</span></li> :
                <li key={index} className="vendor-spec-detail">
                  <strong>{specification.slice(0, separator + 1)}</strong>{specification.slice(separator + 1)}
                </li>;
            })}
          </ul> },
        { key: "vendors", label: "Vendors", sortable: false, value: item =>
          sets.find(set => set.category === item.category && set.itemIds.includes(item.id))?.vendorIds.length ?? 0,
          render: item => {
            const set = sets.find(value => value.category === item.category && value.itemIds.includes(item.id));
            if (!set?.vendorIds.length) return <span>Vendors 0</span>;
            return <button type="button" className="whitespace-nowrap text-teal-800 underline underline-offset-2 dark:text-teal-300"
              aria-haspopup="dialog" onClick={() => setViewedSet(set)}>Vendors {set.vendorIds.length}</button>;
          },
        },
      ]} />
    </div>
    {editable && sets.filter(set => !categories.has(set.category)).map(set =>
      <p key={set.itemIds[0]} className="text-sm text-yellow-900">
        A saved set contains items outside the finalised list.{" "}
        <button type="button" className={action} disabled={busy}
          onClick={() => setSets(current => current.filter(value => value !== set))}>Remove outdated set</button>
      </p>)}
    {editable && <div className="flex justify-end">
      <button type="button" className={action} disabled={busy || !dirty || !valid} onClick={() => void save()}>Save vendors</button>
    </div>}
    {viewedSet && <RecordDrawer title={`Vendors for ${categories.get(viewedSet.category)?.name}`} onClose={() => setViewedSet(undefined)}>
      <ul className="list-disc space-y-1 pl-5 text-sm">
        {viewedSet.vendorIds.map(id => <li key={id}>{vendorsById.get(id)?.name ?? "Vendor no longer eligible"}</li>)}
      </ul>
      {viewedSet.itemIds.some(id => itemsById.get(id)?.category !== viewedSet.category) &&
        <p className="mt-3 text-sm text-yellow-900">Includes items outside this finalised category.</p>}
      {editable && <button type="button" className={`${action} mt-3`} disabled={busy}
        onClick={() => {
          setSets(current => current.filter(set => set !== viewedSet));
          setViewedSet(undefined);
        }}>Remove set</button>}
    </RecordDrawer>}
    {picker && <RecordDrawer title="Add vendors" wide onClose={() => setPicker(undefined)}
      footer={<button type="button" className={action} disabled={picker.vendorIds.length < 3} onClick={applyVendors}>Apply to checked items</button>}>
      <p className="mb-3 text-sm">{picker.vendorIds.length} vendors selected for {picker.itemIds.length} checked items.</p>
      <ul className="mb-3 list-disc pl-5 text-sm">
        {picker.itemIds.map(id => {
          const item = itemsById.get(id);
          return item && <li key={id}>{itemName(item)}{identifyingSpecs(item) && ` · ${identifyingSpecs(item)}`}</li>;
        })}
      </ul>
      {available.length < 3 && <p className="mb-3 text-sm text-yellow-900">Fewer than three unused eligible vendors are available in this category.</p>}
      <DataGrid label="Available vendors" rows={available} initialSort="name" columns={[
        { key: "name", label: "Vendor", value: vendor => vendor.name },
        { key: "code", label: "Vendor code", value: vendor => vendor.code },
        { key: "select", label: "Select", value: vendor => picker.vendorIds.includes(vendor.id) ? "Selected" : "", sortable: false,
          className: "text-right", headerClassName: "text-right", render: vendor =>
            <ValidatedInput type="checkbox" aria-label={`Select ${vendor.name}`} className="size-5"
              checked={picker.vendorIds.includes(vendor.id)} onChange={event => {
                const checked = event.target.checked;
                setPicker(current => current && { ...current, vendorIds: checked
                  ? [...current.vendorIds, vendor.id] : current.vendorIds.filter(id => id !== vendor.id) });
              }} /> },
      ]} />
    </RecordDrawer>}
  </section>;
}
