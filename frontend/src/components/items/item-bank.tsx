"use client";
import { displayServerErrors } from "@/components/ui/server-field-errors";
import { ValidatedForm, ValidatedInput, ValidatedLabel, ValidatedSelect } from "@/components/ui/validated-controls";
import { useEffect, useState, type SubmitEvent } from "react";
import { PlusIcon } from "@heroicons/react/24/outline";
import { useAuthenticatedUser } from "@/components/auth-user-provider";
import {
  type Item,
  type Definition,
  type Manufacturer,
  eligible,
  itemName,
  configuredItemName,
  request,
  input,
  button,
  secondary,
  panel,
} from "./item-data";
import { DataGrid, GridAction, GridStatus } from "@/components/ui/data-grid";
import { RecordDrawer } from "@/components/ui/record-drawer";
import { ItemHistory } from "./item-history";
const itemStatus = (status: string) =>
  status === "approved"
    ? "Active"
    : status === "retired"
      ? "Retired"
      : "Unregistered";

export function ItemBank() {
  const user = useAuthenticatedUser();
  const can = (a: string) =>
    user.permissions.includes("system:admin") ||
    user.permissions.includes("settings.items." + a);
  const [items, setItems] = useState<Item[]>([]);
  const [refs, setRefs] = useState<{
    catalogue: Definition[];
    manufacturers: Manufacturer[];
  }>({ catalogue: [], manufacturers: [] });
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [reload, setReload] = useState(0);
  const [adding, setAdding] = useState(false);
  const [search, setSearch] = useState("");
  const [category, setCategory] = useState("");
  const [status, setStatus] = useState("");
  const [reason, setReason] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const [viewId, setViewId] = useState<string | null>(null);
  useEffect(() => {
    let live = true;
    Promise.all([
      request<Item[]>("/api/settings/catalogue-items"),
      request<typeof refs>("/api/settings/catalogue-items/references"),
    ])
      .then(([items, refs]) => {
        if (live) {
          setItems(items);
          setRefs(refs);
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
  }, [reload]);
  const filtered = items.filter(
    (i) =>
      (!category || i.snapshot.configuration.category_code === category) &&
      (!status || i.status === status) &&
      `${i.sku} ${itemName(i)} ${i.description}`
        .toLowerCase()
        .includes(search.toLowerCase()),
  );
  const issued = items.filter((i) => i.serial).length;
  async function transition(item: Item, status: string, form: HTMLFormElement) {
    setBusy(true);
    setError("");
    try {
      const saved = await request<Item>(
        `/api/settings/catalogue-items/${item.id}`,
        { version: item.version, status, reason },
        "PATCH",
      );
      setNotice(
        saved.sku
          ? `${saved.sku} · ${itemStatus(saved.status)}`
          : `Item ${itemStatus(saved.status)}`,
      );
      setReload((r) => r + 1);
    } catch (e) {
      if (displayServerErrors(form, e)) return;
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <section className="space-y-5">
      <div className={panel}>
        <div className="flex flex-col items-stretch justify-between gap-4 sm:flex-row sm:items-start">
          <div>
            <p className="text-xs font-bold uppercase text-blue-700">
              Shared material catalogue
            </p>
            <h2 className="mt-1 text-2xl font-bold">Item bank</h2>
            
          </div>
          {can("write") && (
            <button className={`${button} flex shrink-0 items-center justify-center gap-2 rounded-xl`} disabled={loading || !!error} onClick={() => setAdding(true)}>
              <PlusIcon aria-hidden="true" className="h-5 w-5" /> Create item
            </button>
          )}
        </div>
        <div className="grid gap-3 sm:grid-cols-3">
          <div className="rounded-xl bg-blue-50 p-4">
            <strong className="text-2xl">{issued}</strong>
            <p className="text-sm">SKUs issued · separate 001–999 sequence per category</p>
          </div>
          <div className="rounded-xl bg-amber-50 p-4">
            <strong className="text-2xl">
              {items.filter((i) => i.status === "retired").length}
            </strong>
            <p className="text-sm">Retired items</p>
          </div>
          <div className="rounded-xl bg-slate-50 p-4">
            <strong className="text-2xl">
              {items.filter((i) => i.status === "approved").length}
            </strong>
            <p className="text-sm">Active catalogue items</p>
          </div>
        </div>
      </div>
      {error && (
        <p role="alert" className="text-red-700">
          {error}
        </p>
      )}
      {notice && (
        <p
          role="status"
          className="rounded-xl bg-blue-50 p-4 text-blue-900 break-all"
        >
          {notice}
        </p>
      )}
      {adding && (
        <RecordDrawer title="Create item" onClose={() => setAdding(false)}>
          <ItemCreate
            refs={refs}
            done={(item) => {
              setAdding(false);
              setNotice(
                item.sku
                  ? `${item.sku} · ${itemStatus(item.status)}. Matching items reuse their existing SKU.`
                  : "This existing item has no SKU yet. Open its details to register it.",
              );
              setReload((r) => r + 1);
            }}
            close={() => setAdding(false)}
          />
        </RecordDrawer>
      )}
      <div className="grid gap-3 sm:grid-cols-3">
        <ValidatedLabel>
          Search items
          <ValidatedInput name="search"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className={input}
            placeholder="SKU, specification, manufacturer or model"
          />
        </ValidatedLabel>
        <ValidatedLabel>
          Category
          <ValidatedSelect name="category"
            value={category}
            onChange={(e) => setCategory(e.target.value)}
            className={input}
          >
            <option value="">All categories</option>
            {refs.catalogue
              .filter((c) => c.data.kind === "category")
              .map((c) => (
                <option key={c.id} value={c.code}>
                  {c.name}
                </option>
              ))}
          </ValidatedSelect>
        </ValidatedLabel>
        <ValidatedLabel>
          Status
          <ValidatedSelect name="status"
            value={status}
            onChange={(e) => setStatus(e.target.value)}
            className={input}
          >
            <option value="">All statuses</option>
            {["approved", "retired"].map((s) => (
              <option key={s} value={s}>
                {s === "pending"
                  ? "Unregistered (legacy pending)"
                  : s === "rejected"
                    ? "Unregistered (legacy rejected)"
                    : itemStatus(s)}
              </option>
            ))}
          </ValidatedSelect>
        </ValidatedLabel>
      </div>

      {loading && <p>Loading item bank…</p>}

      <DataGrid<Item>
        label="Item bank"
        rows={filtered}
        loading={loading}
        searchable={false}
        initialSort="description"
        columns={[
          {
            key: "sku",
            label: "SKU",
            value: (r) => r.sku ?? "Not registered",
            className: "max-w-xs break-all font-mono text-xs text-blue-800",
          },
          {
            key: "description",
            label: "Item name",
            value: (r) => itemName(r),
            className: "min-w-64 max-w-lg font-semibold",
          },
          {
            key: "manufacturer",
            label: "Manufacturer",
            value: (r) => r.snapshot.manufacturer.name,
          },
          { key: "model", label: "Model", value: (r) => r.model || "—" },
          {
            key: "uom",
            label: "UOM",
            value: (r) => r.snapshot.configuration.uom ?? "—",
          },
          {
            key: "status",
            label: "Status",
            value: (r) => itemStatus(r.status),
            render: (r) => <GridStatus value={itemStatus(r.status)} />,
          },
        ]}
        actions={(r) => (
          <GridAction
            label={`View ${r.sku ?? itemName(r)}`}
            onClick={() => { setReason(""); setError(""); setViewId(r.id); }}
          />
        )}
      />
      {viewId && (
        <RecordDrawer title="Item details" onClose={() => setViewId(null)}>
          {error && <p role="alert" className="text-red-700">{error}</p>}
          <div className="space-y-4">
            {items
              .filter((item) => item.id === viewId)
              .map((item) => (
                <article className={panel} key={item.id}>
                  <p className="break-all font-mono text-sm font-bold text-blue-800">
                    {item.sku ?? "No SKU issued"}
                  </p>
                  <h3 className="font-semibold">{itemName(item)}</h3>
                  <p className="text-sm text-slate-600">{item.description}</p>
                  <p className="text-sm text-slate-500">
                    {itemStatus(item.status)} ·{" "}
                    {item.snapshot.configuration.uom} · Definition revision{" "}
                    {item.snapshot.configuration.revision}
                  </p>
                  <div className="flex flex-wrap gap-2">
                    {can("write") && (
                      <ValidatedForm className="w-full space-y-3" onSubmit={e => { e.preventDefault(); void transition(item, item.status === "approved" ? "retired" : "approved", e.currentTarget); }}>
                        <ValidatedLabel className="block">Reason for status change<ValidatedInput name="reason" required maxLength={2000} disabled={busy} className={input} value={reason} onChange={e => setReason(e.target.value)} /></ValidatedLabel>
                        <button disabled={busy} className={secondary}>{busy ? "Saving…" : item.status === "approved" ? "Retire" : "Reactivate"}</button>
                      </ValidatedForm>
                    )}
                    <ItemHistory
                      url={`/api/settings/catalogue-items/${item.id}/history`}
                    />
                  </div>
                </article>
              ))}
          </div>
        </RecordDrawer>
      )}
    </section>
  );
}
function ItemCreate({
  refs,
  done,
  close,
}: {
  refs: { catalogue: Definition[]; manufacturers: Manufacturer[] };
  done: (i: Item) => void;
  close: () => void;
}) {
  const [category, setCategory] = useState("");
  const [configuration, setConfiguration] = useState("");
  const [manufacturer, setManufacturer] = useState("");
  const [specs, setSpecs] = useState<Record<string, string>>({});
  const [model, setModel] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const config = refs.catalogue.find((c) => c.id === configuration);
  const make = refs.manufacturers.find((m) => m.id === manufacturer);
  async function save(e: SubmitEvent<HTMLFormElement>) {
    const submittedForm = e.currentTarget instanceof HTMLFormElement ? e.currentTarget : undefined;
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      done(
        await request<Item>("/api/settings/catalogue-items", {
          configuration_id: configuration,
          manufacturer_id: manufacturer,
          specifications: specs,
          model,
        }),
      );
    } catch (e) {
      if (displayServerErrors(submittedForm, e)) return;
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <ValidatedForm onSubmit={save} className={panel}>
      <h3 className="text-lg font-bold">Configure an item</h3>
      {error && (
        <p role="alert" className="text-red-700">
          {error}
        </p>
      )}
      <fieldset disabled={busy} className="min-w-0 space-y-4">
        <div className="grid gap-3 sm:grid-cols-3">
          <ValidatedLabel>
            Item category
            <ValidatedSelect name="category"
              required
              className={input}
              aria-label="Item category"
              value={category}
              onChange={(e) => {
                setCategory(e.target.value);
                setConfiguration("");
                setManufacturer("");
                setSpecs({});
                setModel("");
              }}
            >
              <option value="">Select category</option>
              {refs.catalogue
                .filter((c) => c.is_active && c.data.kind === "category")
                .map((c) => (
                  <option key={c.id} value={c.code}>
                    {c.name}
                  </option>
                ))}
            </ValidatedSelect>
          </ValidatedLabel>
          <ValidatedLabel>
            Product configuration
            <ValidatedSelect name="configuration"
              required
              className={input}
              disabled={!category}
              aria-label="Product configuration"
              value={configuration}
              onChange={(e) => {
                setConfiguration(e.target.value);
                setManufacturer("");
                setSpecs({});
                setModel("");
              }}
            >
              <option value="">Select product type</option>
              {refs.catalogue
                .filter(
                  (c) =>
                    c.is_active &&
                    c.data.kind === "type" &&
                    c.data.category_code === category,
                )
                .map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name} · revision {c.data.revision}
                  </option>
                ))}
            </ValidatedSelect>
          </ValidatedLabel>
          <ValidatedLabel>
            Manufacturer
            <ValidatedSelect name="manufacturer"
              required
              className={input}
              disabled={!configuration}
              aria-label="Manufacturer"
              value={manufacturer}
              onChange={(e) => setManufacturer(e.target.value)}
            >
              <option value="">Select eligible manufacturer</option>
              {config &&
                refs.manufacturers
                  .filter((m) => eligible(m, config))
                  .map((m) => (
                    <option key={m.id} value={m.id}>
                      {m.name}
                    </option>
                  ))}
            </ValidatedSelect>
          </ValidatedLabel>
        </div>
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {config?.data.fields?.map((f) => (
            <ValidatedLabel key={f.code}>
              {f.label}
              
              <ValidatedSelect
                name={f.code}
                aria-label={f.label}
                required={f.required}
                className={input}
                value={specs[f.code] ?? ""}
                onChange={(e) =>
                  setSpecs({ ...specs, [f.code]: e.target.value })
                }
              >
                <option value="">
                  {f.required ? "Choose value" : "Not specified"}
                </option>
                {f.options.map((o) => (
                  <option key={o.code} value={o.code}>
                    {o.label}
                  </option>
                ))}
              </ValidatedSelect>
            </ValidatedLabel>
          ))}
        </div>
        <ValidatedLabel className="block">
          Manufacturer model / part number
          {!config?.data.model_required ? " (if applicable)" : ""}
          <ValidatedInput name="model"
            required={config?.data.model_required}
            maxLength={120}
            value={model}
            onChange={(e) => setModel(e.target.value)}
            className={input}
          />
        </ValidatedLabel>
        {config && (
          <div className="rounded-xl bg-blue-50 p-4 text-sm">
            <strong>Item name preview</strong>
            <p>
              {configuredItemName(
                { name: config.name, fields: config.data.fields },
                specs,
                make?.name,
                model,
              )}
            </p>
            <p className="mt-2">
              {config.data.uom} · Permanent three-digit number assigned when you
              save.
            </p>
          </div>
        )}
        <div className="flex gap-2">
          <button className={`${button} rounded-xl`}>{busy ? "Saving…" : "Create item"}</button>
          <button type="button" className={secondary} onClick={close}>
            Cancel
          </button>
        </div>
      </fieldset>
    </ValidatedForm>
  );
}
