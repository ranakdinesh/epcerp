"use client";
import { displayServerErrors } from "@/components/ui/server-field-errors";
import { ValidatedForm, ValidatedInput, ValidatedLabel, ValidatedSelect } from "@/components/ui/validated-controls";
import { useEffect, useState, type FormEvent } from "react";
import { request, input, button, secondary, panel } from "./item-data";
import { DataGrid, GridAction, GridStatus } from "@/components/ui/data-grid";
import { RecordDrawer } from "@/components/ui/record-drawer";
import { ItemHistory } from "./item-history";
type Choice = {
  id: string;
  sku: string;
  description: string;
  engineering_key: string;
  uom: string;
  manufacturer: string;
  category: string;
  configuration: string;
};
type Demand = {
  id: string;
  item_id: string;
  item_snapshot: Choice;
  engineering_key: string;
  quantity: string;
  required_date: string;
  allocations: { location: string; quantity: string }[];
  alternatives: string[];
  version: number;
  is_active: boolean;
};
export function ProjectItems({ projectId }: { projectId: string }) {
  const [write, setWrite] = useState(false);
  const [records, setRecords] = useState<Demand[]>([]);
  const [choices, setChoices] = useState<Choice[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [search, setSearch] = useState("");
  const [category, setCategory] = useState("");
  const [manufacturer, setManufacturer] = useState("");
  const [reload, setReload] = useState(0);
  const [selection, setSelection] = useState<{
    item: Choice;
    demand?: Demand;
  } | null>(null);
  const [viewId, setViewId] = useState<string | null>(null);
  useEffect(() => {
    let live = true;
    Promise.all([
      request<Demand[]>(`/api/projects/${projectId}/items`),
      request<Choice[]>(`/api/projects/${projectId}/items/catalogue`),
      request<{ can_write: boolean }>(
        `/api/projects/${projectId}/items/access`,
      ),
    ])
      .then(([r, c, access]) => {
        if (live) {
          setWrite(access.can_write);
          setRecords(r);
          setChoices(c);
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
  }, [projectId, reload]);
  const filtered = choices.filter(
    (c) =>
      (!category || c.category === category) &&
      (!manufacturer || c.manufacturer === manufacturer) &&
      `${c.sku} ${c.description}`.toLowerCase().includes(search.toLowerCase()),
  );
  return (
    <section className="space-y-5">
      <div className={panel}>
        <h2 className="text-2xl font-bold">Project items</h2>
        <p className="text-slate-600">
          One material requirement per configuration. Allocate quantities to
          locations and record manufacturer alternatives under the same
          requirement.
        </p>
        <p>{records.filter((r) => r.is_active).length} active requirements</p>
      </div>
      {error && (
        <p role="alert" className="text-red-700">
          {error}
        </p>
      )}
      {loading && <p>Loading project items…</p>}
      {selection && (
        <RecordDrawer
          title="Project item quantity & allocations"
          onClose={() => setSelection(null)}
        >
          <DemandEditor
            key={selection.demand?.id ?? selection.item.id}
            projectId={projectId}
            item={selection.item}
            source={selection.demand}
            choices={choices}
            done={() => {
              setSelection(null);
              setReload((r) => r + 1);
            }}
            close={() => setSelection(null)}
          />
        </RecordDrawer>
      )}
      <DataGrid<Demand>
        label="Project requirements"
        rows={records}
        loading={loading}
        initialSort="description"
        status={(r) => (r.is_active ? "Active" : "Inactive")}
        columns={[
          {
            key: "sku",
            label: "SKU",
            value: (r) => r.item_snapshot.sku,
            className: "max-w-xs break-all font-mono text-xs text-blue-800",
          },
          {
            key: "description",
            label: "Item / configuration",
            value: (r) => r.item_snapshot.description,
            className: "min-w-60 max-w-lg font-semibold",
          },
          {
            key: "manufacturer",
            label: "Manufacturer",
            value: (r) => r.item_snapshot.manufacturer,
          },
          {
            key: "quantity",
            label: "Quantity",
            value: (r) => Number(r.quantity),
            render: (r) => r.quantity,
          },
          { key: "uom", label: "UOM", value: (r) => r.item_snapshot.uom },
          {
            key: "date",
            label: "Required date",
            value: (r) => r.required_date,
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
              label={`View ${r.item_snapshot.sku}`}
              onClick={() => setViewId(r.id)}
            />
            {write && (
              <GridAction
                kind="edit"
                label={`Edit ${r.item_snapshot.sku}`}
                onClick={() =>
                  setSelection({ item: r.item_snapshot, demand: r })
                }
              />
            )}
          </>
        )}
      />
      {viewId && (
        <RecordDrawer
          title="Project requirement details"
          onClose={() => setViewId(null)}
        >
          {" "}
          <div className="grid gap-4">
            {records
              .filter((r) => r.id === viewId)
              .map((r) => (
                <article key={r.id} className={panel}>
                  <p className="break-all text-sm font-bold text-blue-800">
                    {r.item_snapshot.sku}
                  </p>
                  <h3 className="font-semibold">
                    {r.item_snapshot.description}
                  </h3>
                  <p>
                    {r.quantity} {r.item_snapshot.uom} · Required{" "}
                    {r.required_date} · {r.is_active ? "Active" : "Inactive"}
                  </p>
                  {r.allocations.length > 0 && (
                    <p className="text-sm">
                      {r.allocations
                        .map((a) => `${a.location}: ${a.quantity}`)
                        .join(" · ")}
                    </p>
                  )}
                  <p className="text-sm">
                    {r.alternatives.length} manufacturer alternatives
                  </p>
                  <div className="flex flex-wrap gap-2">
                    {write && (
                      <button
                        className={secondary}
                        onClick={() => {
                          setViewId(null);
                          setSelection({ item: r.item_snapshot, demand: r });
                        }}
                      >
                        Edit quantity & allocations
                      </button>
                    )}
                    <ItemHistory
                      url={`/api/projects/${projectId}/items/${r.id}/history`}
                    />
                  </div>
                </article>
              ))}
          </div>
        </RecordDrawer>
      )}
      <div className={panel}>
        <h3 className="text-lg font-bold">Pick from the item bank</h3>
        <div className="grid gap-3 sm:grid-cols-3">
          <ValidatedLabel>
            Search catalogue
            <ValidatedInput name="search"
              className={input}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="SKU, configuration or model"
            />
          </ValidatedLabel>
          <ValidatedLabel>
            Category
            <ValidatedSelect name="category"
              className={input}
              value={category}
              onChange={(e) => setCategory(e.target.value)}
            >
              <option value="">All categories</option>
              {[...new Set(choices.map((c) => c.category))].map((c) => (
                <option key={c}>{c}</option>
              ))}
            </ValidatedSelect>
          </ValidatedLabel>
          <ValidatedLabel>
            Manufacturer
            <ValidatedSelect name="manufacturer"
              className={input}
              value={manufacturer}
              onChange={(e) => setManufacturer(e.target.value)}
            >
              <option value="">All manufacturers</option>
              {[...new Set(choices.map((c) => c.manufacturer))].map((c) => (
                <option key={c}>{c}</option>
              ))}
            </ValidatedSelect>
          </ValidatedLabel>
        </div>
        <DataGrid<Choice>
          label="Approved item picker"
          rows={filtered}
          loading={loading}
          searchable={false}
          initialSort="description"
          columns={[
            {
              key: "sku",
              label: "SKU",
              value: (r) => r.sku,
              className: "max-w-xs break-all font-mono text-xs text-blue-800",
            },
            {
              key: "description",
              label: "Item / configuration",
              value: (r) => r.description,
              className: "min-w-60 max-w-lg",
            },
            {
              key: "manufacturer",
              label: "Manufacturer",
              value: (r) => r.manufacturer,
            },
            { key: "uom", label: "UOM", value: (r) => r.uom },
            {
              key: "added",
              label: "Project selection",
              value: (c) =>
                records.some((r) => r.engineering_key === c.engineering_key)
                  ? "Already represented"
                  : "Available",
            },
          ]}
          actions={(c) => {
            const existing = records.find(
              (r) => r.engineering_key === c.engineering_key,
            );
            return write ? (
              <button
                className={existing ? secondary : button}
                onClick={() =>
                  setSelection(
                    existing
                      ? { item: existing.item_snapshot, demand: existing }
                      : { item: c },
                  )
                }
              >
                {existing
                  ? existing.item_id === c.id
                    ? "Already added — edit"
                    : "Configure alternative"
                  : "Add to project"}
              </button>
            ) : null;
          }}
        />
        {!loading && !filtered.length && (
          <p className="py-6 text-slate-500">
            No active catalogue items match. Create an item in Item bank first.
          </p>
        )}
      </div>
    </section>
  );
}
function DemandEditor({
  projectId,
  item,
  source,
  choices,
  done,
  close,
}: {
  projectId: string;
  item: Choice;
  source?: Demand;
  choices: Choice[];
  done: () => void;
  close: () => void;
}) {
  const [quantity, setQuantity] = useState(source?.quantity ?? "1");
  const [date, setDate] = useState(source?.required_date ?? "");
  const [allocations, setAllocations] = useState(source?.allocations ?? []);
  const [alternatives, setAlternatives] = useState(source?.alternatives ?? []);
  const [active, setActive] = useState(source?.is_active ?? true);
  const [reason, setReason] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const wholeQuantity = ["nos", "no", "pcs", "pieces", "each"].includes(item.uom.trim().toLowerCase());
  async function save(e: FormEvent) {
    const submittedForm = e.currentTarget instanceof HTMLFormElement ? e.currentTarget : undefined;
    e.preventDefault();
    setBusy(true);
    try {
      await request(
        `/api/projects/${projectId}/items${source ? `/${source.id}` : ""}`,
        {
          item_id: source?.item_id ?? item.id,
          quantity,
          required_date: date,
          allocations,
          alternatives,
          is_active: active,
          version: source?.version ?? 1,
          reason,
        },
        source ? "PATCH" : "POST",
      );
      done();
    } catch (e) {
      if (displayServerErrors(submittedForm, e)) return;
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <ValidatedForm onSubmit={save} className={panel}>
      <h3 className="font-bold">
        {source ? "Update project requirement" : "Add project requirement"}
      </h3>
      <p className="break-all text-sm">{item.sku}</p>
      {error && (
        <p role="alert" className="text-red-700">
          {error}
        </p>
      )}
      <fieldset disabled={busy} className="space-y-4">
        <div className="grid gap-3 sm:grid-cols-2">
          <ValidatedLabel>
            Quantity ({item.uom})
            <ValidatedInput name="quantity"
              required
              type="number"
              min={wholeQuantity ? "1" : "0.001"}
              step={wholeQuantity ? "1" : "0.001"}
              className={input}
              value={quantity}
              onChange={(e) => setQuantity(e.target.value)}
            />
          </ValidatedLabel>
          <ValidatedLabel>
            Required date
            <ValidatedInput name="date"
              required
              type="date"
              className={input}
              value={date}
              onChange={(e) => setDate(e.target.value)}
            />
          </ValidatedLabel>
        </div>
        <p className="text-sm text-slate-500">
          Location quantities must add up to the total. Leave allocations empty
          if locations are not yet planned.
        </p>
        {allocations.map((a, i) => (
          <div className="grid gap-3 sm:grid-cols-[1fr_1fr_auto]" key={i}>
            <ValidatedLabel>
              Location
              <ValidatedInput
                required
                className={input}
                value={a.location}
                onChange={(e) =>
                  setAllocations(
                    allocations.map((a, j) =>
                      j === i ? { ...a, location: e.target.value } : a,
                    ),
                  )
                }
              />
            </ValidatedLabel>
            <ValidatedLabel>
              Allocated quantity
              <ValidatedInput
                required
                type="number"
                min={wholeQuantity ? "1" : "0.001"}
                step={wholeQuantity ? "1" : "0.001"}
                className={input}
                value={a.quantity}
                onChange={(e) =>
                  setAllocations(
                    allocations.map((a, j) =>
                      j === i ? { ...a, quantity: e.target.value } : a,
                    ),
                  )
                }
              />
            </ValidatedLabel>
            <button
              type="button"
              className={secondary}
              onClick={() =>
                setAllocations(allocations.filter((_, j) => j !== i))
              }
            >
              Remove location
            </button>
          </div>
        ))}
        <button
          type="button"
          className={secondary}
          onClick={() =>
            setAllocations([...allocations, { location: "", quantity: "" }])
          }
        >
          Add location allocation
        </button>
        <fieldset>
          <legend className="font-semibold">
            Alternative manufacturers for this requirement
          </legend>
          <p className="text-sm text-slate-500">
            Alternatives do not add quantity or imply client approval.
          </p>
          {choices
            .filter(
              (c) =>
                c.engineering_key === item.engineering_key &&
                c.id !== (source?.item_id ?? item.id),
            )
            .map((c) => (
              <ValidatedLabel key={c.id} className="flex min-h-11 items-center gap-2">
                <ValidatedInput
                  type="checkbox"
                  checked={alternatives.includes(c.id)}
                  onChange={() =>
                    setAlternatives(
                      alternatives.includes(c.id)
                        ? alternatives.filter((id) => id !== c.id)
                        : [...alternatives, c.id],
                    )
                  }
                />
                <span className="break-all">
                  {c.manufacturer} · {c.sku}
                </span>
              </ValidatedLabel>
            ))}
        </fieldset>
        {source && (
          <ValidatedLabel className="flex items-center gap-2">
            <ValidatedInput
              type="checkbox"
              checked={active}
              onChange={(e) => setActive(e.target.checked)}
            />
            Active requirement
          </ValidatedLabel>
        )}
        <ValidatedLabel className="block">
          Change reason
          <ValidatedInput name="reason"
            required
            className={input}
            value={reason}
            onChange={(e) => setReason(e.target.value)}
          />
        </ValidatedLabel>
        <div className="flex gap-2">
          <button className={button}>
            {source ? "Save project item" : "Add requirement"}
          </button>
          <button type="button" className={secondary} onClick={close}>
            Cancel
          </button>
        </div>
      </fieldset>
    </ValidatedForm>
  );
}
