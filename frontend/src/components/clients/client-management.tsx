"use client";

import { useAuthenticatedUser } from "@/components/auth-user-provider";
import { cloneElement, FormEvent, ReactElement, useDeferredValue, useEffect, useId, useState } from "react";

import { clientErrors, gstinError } from "./client-validation";

const contactTypeLabels: Record<string, string> = { EIC: "EIC", HOD: "HOD (Head of department)", SITE_CONTACT: "Site Contact" };

function contactTypeLabel(type: string) { return contactTypeLabels[type] ?? type; }

type ClientContact = {
  contact_type: string;
  id?: string;
  full_name: string;
  designation: string;
  department: string;
  email: string;
  phone: string;
  is_primary: boolean;
  is_active?: boolean;
};
type Client = {
  missing_contact_types?: string[];
  id: string;
  client_code: string;
  legal_name: string;
  display_name: string;
  parent_organization: string;
  client_type: string;
  industry: string;
  gstin: string;
  pan: string;
  cin: string;
  website: string;
  address_line_1: string;
  address_line_2: string;
  city: string;
  state: string;
  postal_code: string;
  country: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
  contacts: ClientContact[];
};
type Reference = { id: string; name: string; code: string; is_active: boolean; state_id: string | null };
type ClientForm = Omit<Client, "id" | "client_code" | "is_active" | "created_at" | "updated_at"> & {
  client_type_id?: string;
  industry_id?: string;
  state_id?: string;
  city_id?: string;
};
type DialogMode = "create" | "edit" | "view" | null;
type Sort = "legal_name_asc" | "legal_name_desc" | "client_code_asc" | "created_at_desc";

const PAGE_SIZE = 10;
const emptyContact = (): ClientContact => ({
  full_name: "",
  contact_type: "",
  designation: "",
  department: "",
  email: "",
  phone: "",
  is_primary: false,
});
const emptyForm = (): ClientForm => ({
  legal_name: "",
  display_name: "",
  parent_organization: "",
  client_type: "",
  industry: "",
  client_type_id: undefined,
  industry_id: undefined,
  gstin: "",
  pan: "",
  cin: "",
  website: "",
  address_line_1: "",
  address_line_2: "",
  city: "",
  state: "",
  city_id: undefined,
  state_id: undefined,
  postal_code: "",
  country: "India",
  contacts: [],
});

function Icon({ name }: { name: "building" | "search" | "plus" | "eye" | "edit" | "ban" | "close" | "user" | "chevron" }) {
  const paths = {
    building: <><path d="M4 21V5.5A1.5 1.5 0 0 1 5.5 4h8A1.5 1.5 0 0 1 15 5.5V21" /><path d="M8 8h3M8 12h3M8 16h3M17 10.5h1.5A1.5 1.5 0 0 1 20 12v9M2 21h20" /></>,
    search: <><circle cx="10.5" cy="10.5" r="6" /><path d="m15 15 4.5 4.5" /></>,
    plus: <path d="M12 5v14M5 12h14" />,
    eye: <><path d="M2.5 12S6 5.5 12 5.5 21.5 12 21.5 12 18 18.5 12 18.5 2.5 12 2.5 12Z" /><circle cx="12" cy="12" r="2.5" /></>,
    edit: <><path d="m14.5 5.5 4 4M4 20l4.2-1 10.6-10.6a2.8 2.8 0 0 0-4-4L4.2 15 4 20Z" /></>,
    ban: <><circle cx="12" cy="12" r="8.5" /><path d="m6 6 12 12" /></>,
    close: <path d="m6 6 12 12M18 6 6 18" />,
    user: <><circle cx="12" cy="8" r="3.25" /><path d="M5.5 20c.5-3.5 2.5-5.25 6.5-5.25s6 1.75 6.5 5.25" /></>,
    chevron: <path d="m8 10 4 4 4-4" />,
  };
  return <svg aria-hidden="true" className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="1.9">{paths[name]}</svg>;
}

function toForm(client: Client, clientTypes: Reference[], industries: Reference[], states: Reference[], cities: Reference[]): ClientForm {
  return {
    legal_name: client.legal_name,
    display_name: client.display_name,
    parent_organization: client.parent_organization,
    client_type: client.client_type,
    industry: client.industry,
    client_type_id: clientTypes.find((item) => item.name === client.client_type)?.id,
    industry_id: industries.find((item) => item.name === client.industry)?.id,
    gstin: client.gstin,
    pan: client.pan,
    cin: client.cin,
    website: client.website,
    address_line_1: client.address_line_1,
    address_line_2: client.address_line_2,
    city: client.city,
    state: client.state,
    city_id: cities.find((item) => item.name === client.city)?.id,
    state_id: states.find((item) => item.name === client.state)?.id,
    postal_code: client.postal_code,
    country: client.country,
    contacts: client.contacts.filter((contact) => contact.is_active !== false).map(contact => ({ ...contact, contact_type: contact.contact_type ?? "" })),
  };
}

function apiMessage(payload: unknown, fallback: string): string {
  if (typeof payload === "object" && payload !== null && "detail" in payload) {
    const { detail } = payload as { detail?: unknown };
    if (typeof detail === "string") return detail;
  }
  return fallback;
}

export function ClientManagement() {
  const user = useAuthenticatedUser();
  const canWrite = user.permissions.includes("system:admin") || user.permissions.includes("settings.clients.write");
  const [clients, setClients] = useState<Client[]>([]);
  const [total, setTotal] = useState(0);
  const [counts, setCounts] = useState<{ all_count: number; active_count: number; inactive_count: number } | null>(null);
  const [page, setPage] = useState(1);
  const [query, setQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState<"all" | "active" | "inactive">("all");
  const [typeFilter, setTypeFilter] = useState("");
  const [sort, setSort] = useState<Sort>("legal_name_asc");
  const [reload, setReload] = useState(0);
  const [isLoading, setIsLoading] = useState(true);
  const [notice, setNotice] = useState("");
  const [dialogMode, setDialogMode] = useState<DialogMode>(null);
  const [selectedClient, setSelectedClient] = useState<Client | null>(null);
  const [form, setForm] = useState<ClientForm>(emptyForm);
  const [formError, setFormError] = useState("");
  const [serverErrors, setServerErrors] = useState<Record<string, string>>({});
  const [isSaving, setIsSaving] = useState(false);
  const [clientTypes, setClientTypes] = useState<Reference[]>([]);
  const [industries, setIndustries] = useState<Reference[]>([]);
  const [states, setStates] = useState<Reference[]>([]);
  const [cities, setCities] = useState<Reference[]>([]);
  const deferredQuery = useDeferredValue(query);

  useEffect(() => {
    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      const params = new URLSearchParams({ page: String(page), page_size: String(PAGE_SIZE), sort });
      if (deferredQuery.trim()) params.set("search", deferredQuery.trim());
      if (statusFilter !== "all") params.set("status", statusFilter);
      if (typeFilter.trim()) params.set("client_type", typeFilter.trim());
      setIsLoading(true);
      void fetch(`/api/settings/clients?${params}`, { cache: "no-store", signal: controller.signal })
        .then(async (response) => {
          const payload = await response.json();
          if (!response.ok) throw new Error(apiMessage(payload, "Unable to load client master."));
          return payload as { items: Client[]; total: number; all_count: number; active_count: number; inactive_count: number };
        })
        .then((payload) => {
          if (controller.signal.aborted) return;
          setCounts(payload);
          setClients(payload.items);
          setTotal(payload.total);
          setNotice("");
        })
        .catch((error: unknown) => {
          if (error instanceof DOMException && error.name === "AbortError") return;
          setNotice(error instanceof Error ? error.message : "Unable to load client master.");
        })
        .finally(() => { if (!controller.signal.aborted) setIsLoading(false); });
    }, deferredQuery === query ? 0 : 220);
    return () => {
      controller.abort();
      window.clearTimeout(timer);
    };
  }, [deferredQuery, page, query, reload, sort, statusFilter, typeFilter]);

  useEffect(() => {
    void Promise.all(["client-types", "industries", "states"].map(async (kind) => {
      const response = await fetch(`/api/settings/client-references/${kind}`, { cache: "no-store" });
      if (!response.ok) throw new Error("Unable to load client reference data.");
      return await response.json() as Reference[];
    })).then(([types, industryList, stateList]) => {
      setClientTypes(types); setIndustries(industryList); setStates(stateList);
    }).catch(() => setNotice("Client reference data could not be loaded."));
  }, []);

  useEffect(() => {
    if (!form.state_id) return;
    const controller = new AbortController();
    void fetch(`/api/settings/client-references/cities?state_id=${form.state_id}`, { cache: "no-store", signal: controller.signal })
      .then(async (response) => response.ok ? await response.json() as Reference[] : [])
      .then((cityList: Reference[]) => {
        if (controller.signal.aborted) return;
        setCities(cityList);
        setForm((current) => current.city_id || !current.city ? current : {
          ...current,
          city_id: cityList.find((item) => item.name === current.city)?.id,
        });
      })
      .catch(() => { if (!controller.signal.aborted) setCities([]); });
    return () => controller.abort();
  }, [form.state_id, dialogMode]);

  const pageCount = Math.max(1, Math.ceil(total / PAGE_SIZE));


  function openDialog(mode: DialogMode, client: Client | null = null) {
    if (mode !== "view" && !canWrite) return;
    setCities([]);
    setServerErrors({});
    setSelectedClient(client);
    setForm(client ? toForm(client, clientTypes, industries, states, cities) : emptyForm());
    setFormError("");
    setDialogMode(mode);
  }

  function setField<Key extends keyof ClientForm>(key: Key, value: ClientForm[Key]) {
    setServerErrors(current => ({...current, [key]: ""}));
    if (key === "state_id" && value !== form.state_id) setCities([]);
    setForm((current) => ({ ...current, [key]: value }));
  }

  function updateContact(index: number, field: keyof ClientContact, value: string | boolean) {
    setServerErrors(current => ({...current, [`contacts.${index}.${field}`]: ""}));
    setForm((current) => ({
      ...current,
      contacts: current.contacts.map((contact, contactIndex) => ({
        ...contact,
        ...(contactIndex === index ? { [field]: value } : {}),
        ...(field === "is_primary" && value && contactIndex !== index ? { is_primary: false } : {}),
      })),
    }));
  }

  function removeContact(index: number) {
    setForm((current) => ({
      ...current,
      contacts: current.contacts.filter((_, itemIndex) => itemIndex !== index),
    }));
  }

  async function saveClient(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!canWrite) { setFormError("You do not have permission to change client records."); return; }
    if (Object.keys(clientErrors(form)).length) return;
    const contacts = form.contacts
      .map((contact) => ({
        ...(contact.id ? { id: contact.id } : {}),
        full_name: contact.full_name.trim(),
        contact_type: contact.contact_type,
        designation: contact.designation?.trim() ?? "",
        department: contact.department?.trim() ?? "",
        email: contact.email?.trim() ?? "",
        phone: contact.phone?.trim() ?? "",
        is_primary: Boolean(contact.is_primary),
      }));

    const clientPayload: Record<string, unknown> = {
      legal_name: form.legal_name.trim(),
      display_name: form.display_name.trim(),
      parent_organization: form.parent_organization.trim(),
      client_type: form.client_type.trim(),
      industry: form.industry.trim(),
      gstin: form.gstin.trim().toUpperCase(),
      pan: form.pan.trim().toUpperCase(),
      cin: form.cin.trim().toUpperCase(),
      website: form.website.trim(),
      address_line_1: form.address_line_1.trim(),
      address_line_2: form.address_line_2.trim(),
      city: form.city.trim(),
      state: form.state.trim(),
      postal_code: form.postal_code.trim(),
      country: form.country.trim() || "India",
      contacts,
    };
    if (form.client_type_id) clientPayload.client_type_id = form.client_type_id;
    if (form.industry_id) clientPayload.industry_id = form.industry_id;
    if (form.state_id) clientPayload.state_id = form.state_id;
    if (form.city_id) clientPayload.city_id = form.city_id;

    setIsSaving(true);
    setFormError("");
    const isEditing = dialogMode === "edit" && selectedClient;
    try {
      const response = await fetch(
        isEditing ? `/api/settings/clients/${selectedClient.id}` : "/api/settings/clients",
        {
          method: isEditing ? "PATCH" : "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(clientPayload),
        },
      );
      const payload = await response.json().catch(() => ({}));
      if (!response.ok) {
        if (Array.isArray(payload.detail)) {
          const errors: Record<string, string> = {};
          for (const issue of payload.detail) {
            if (Array.isArray(issue.loc) && typeof issue.msg === "string") errors[issue.loc.filter((part: unknown) => part !== "body").join(".")] = issue.msg.replace(/^Value error, /, "");
          }
          setServerErrors(errors);
          setFormError(errors[""] || "");
          return;
        }
        setFormError(apiMessage(payload, "Unable to save the client."));
        return;
      }
      setDialogMode(null);
      setNotice(isEditing ? "Client details updated." : "Client created and ready for project selection.");
      setPage(1);
      setReload((current) => current + 1);
    } finally {
      setIsSaving(false);
    }
  }

  async function changeClientStatus(client: Client) {
    const action = client.is_active ? "deactivate" : "activate";
    if (!window.confirm(`Are you sure you want to ${action} client "${client.display_name}"?`)) return;
    try {
      const response = await fetch(`/api/settings/clients/${client.id}/${action}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
      });
      const payload = await response.json().catch(() => ({}));
      if (!response.ok) {
        setNotice(apiMessage(payload, `Unable to ${action} the client.`));
        return;
      }
      setNotice(`${client.display_name} was ${action}d.`);
      setPage(1);
      setReload((current) => current + 1);
    } catch {
      setNotice(`Network error while trying to ${action} the client. Please try again.`);
    }
  }

  return (
    <section className="mx-auto w-full max-w-[1500px] space-y-5">
      <div className="flex flex-col gap-4 border-b border-slate-200 pb-5 lg:flex-row lg:items-end lg:justify-between">
        <div>
          <p className="text-xs font-bold uppercase tracking-[0.16em] text-blue-700">Settings / Master data</p>
          <h2 className="mt-1 text-2xl font-extrabold tracking-tight text-slate-900">Client Master</h2>
          
        </div>
        {canWrite ? <button type="button" onClick={() => openDialog("create")} className="inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-blue-700 px-4 text-sm font-bold text-white shadow-lg shadow-blue-900/15 transition-colors hover:bg-blue-800 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2">
          <Icon name="plus" /> Create client
        </button> : null}
      </div>

      <div className="grid gap-3 sm:grid-cols-3">
        {[
          ["All clients", counts?.all_count ?? "—", "all", "border-slate-200 bg-white text-slate-900"],
          ["Active", counts?.active_count ?? "—", "active", "border-emerald-100 bg-emerald-50/70 text-emerald-800"],
          ["Inactive", counts?.inactive_count ?? "—", "inactive", "border-slate-200 bg-slate-100/70 text-slate-700"],
        ].map(([label, value, filter, classes]) => (
          <button key={String(filter)} type="button" aria-pressed={statusFilter === filter} onClick={() => { setStatusFilter(filter as "all" | "active" | "inactive"); setPage(1); }} className={`rounded-2xl border p-4 text-left transition-transform hover:-translate-y-0.5 focus:outline-none focus:ring-2 focus:ring-blue-500 ${statusFilter === filter ? "ring-2 ring-blue-600 ring-offset-2" : ""} ${classes}`}>
            <span className="block text-xs font-bold uppercase tracking-[0.13em] opacity-65">{label}</span>
            <span className="mt-1 block text-2xl font-extrabold">{value}</span>
          </button>
        ))}
      </div>

      {notice ? <p role="status" className="rounded-xl border border-blue-100 bg-blue-50 px-4 py-3 text-sm font-medium text-blue-800">{notice}</p> : null}
      <div className="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-[0_14px_34px_-26px_rgba(15,23,42,0.35)]">
        <div className="grid gap-3 border-b border-slate-100 p-4 lg:grid-cols-[minmax(0,1fr)_180px_180px_200px]">
          <label className="relative block"><span className="sr-only">Search clients</span><span className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3 text-slate-400"><Icon name="search" /></span><input value={query} onChange={(event) => { setQuery(event.target.value); setPage(1); }} placeholder="Search client, code, city" className="min-h-11 w-full rounded-xl border border-slate-200 bg-slate-50 pl-10 pr-3 text-sm outline-none transition-colors focus:border-blue-500 focus:ring-2 focus:ring-blue-500/15" /></label>
          <select value={statusFilter} onChange={(event) => { setStatusFilter(event.target.value as "all" | "active" | "inactive"); setPage(1); }} aria-label="Filter by status" className="min-h-11 rounded-xl border border-slate-200 bg-white px-3 text-sm font-semibold text-slate-700 outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-500/15"><option value="all">All statuses</option><option value="active">Active</option><option value="inactive">Inactive</option></select>
          <select value={typeFilter} onChange={(event) => { setTypeFilter(event.target.value); setPage(1); }} aria-label="Filter by client type" className="min-h-11 rounded-xl border border-slate-200 bg-white px-3 text-sm font-semibold text-slate-700 outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-500/15"><option value="">All client types</option>{clientTypes.map((item) => <option key={item.id} value={item.name}>{item.name}</option>)}</select>
          <select value={sort} onChange={(event) => setSort(event.target.value as Sort)} aria-label="Sort clients" className="min-h-11 rounded-xl border border-slate-200 bg-white px-3 text-sm font-semibold text-slate-700 outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-500/15"><option value="legal_name_asc">Name: A to Z</option><option value="legal_name_desc">Name: Z to A</option><option value="client_code_asc">Client code</option><option value="created_at_desc">Newest first</option></select>
        </div>
        {isLoading ? <p role="status" className="border-b border-blue-100 bg-blue-50 px-5 py-3 text-sm font-medium text-blue-800">Loading client records...</p> : null}
        <div className="overflow-x-auto">
          <table className="vtpl-data-table w-full min-w-[940px] text-left">
            <thead className="bg-slate-50 text-[11px] font-bold uppercase tracking-[0.12em] text-slate-500"><tr><th className="px-5 py-3">Client</th><th className="px-5 py-3">Classification</th><th className="px-5 py-3">Location</th><th className="px-5 py-3">Primary contact</th><th className="px-5 py-3">Status</th><th className="px-5 py-3 text-right">Actions</th></tr></thead>
            <tbody className="divide-y divide-slate-100">
              {clients.map((client) => {
                const primary = client.contacts.find((contact) => contact.is_primary) ?? client.contacts[0];
                return <tr key={client.id} className="transition-colors hover:bg-slate-50/80"><td className="px-5 py-3"><div className="flex items-center gap-3"><span className="grid h-9 w-9 place-items-center rounded-xl bg-blue-50 text-blue-700"><Icon name="building" /></span><span><span className="block text-sm font-bold text-slate-800">{client.display_name}</span>{!!client.missing_contact_types?.length && <small className="block text-amber-700">Missing contacts: {client.missing_contact_types.map(contactTypeLabel).join(", ")}</small>}<span className="mt-0.5 block font-mono text-xs text-slate-500">{client.client_code} · {client.legal_name}</span></span></div></td><td className="px-5 py-3"><p className="text-sm font-semibold text-slate-700">{client.client_type || "Unclassified"}</p><p className="mt-0.5 text-xs text-slate-500">{client.industry || "Industry not set"}</p></td><td className="px-5 py-3 text-sm text-slate-600">{[client.city, client.state].filter(Boolean).join(", ") || "Location not set"}</td><td className="px-5 py-3"><p className="text-sm font-semibold text-slate-700">{primary?.full_name || "No contact added"}</p><p className="mt-0.5 text-xs text-slate-500">{primary?.designation || primary?.email || "Add a primary contact"}</p></td><td className="px-5 py-3"><span className={`rounded-full px-2.5 py-1 text-xs font-bold ${client.is_active ? "bg-emerald-50 text-emerald-700" : "bg-slate-100 text-slate-600"}`}>{client.is_active ? "Active" : "Inactive"}</span></td><td className="px-5 py-3"><div className="flex justify-end gap-1"><button type="button" aria-label={`View ${client.display_name}`} onClick={() => openDialog("view", client)} className="grid h-10 w-10 place-items-center rounded-xl text-slate-500 transition-colors hover:bg-blue-50 hover:text-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500"><Icon name="eye" /></button>{canWrite ? <><button type="button" aria-label={`Edit ${client.display_name}`} onClick={() => openDialog("edit", client)} className="grid h-10 w-10 place-items-center rounded-xl text-slate-500 transition-colors hover:bg-blue-50 hover:text-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500"><Icon name="edit" /></button><button type="button" aria-label={`${client.is_active ? "Deactivate" : "Activate"} ${client.display_name}`} title={client.is_active ? "Deactivate client" : "Activate client"} onClick={() => void changeClientStatus(client)} className="grid min-h-10 min-w-10 place-items-center rounded-xl text-slate-500 transition-colors hover:bg-red-50 hover:text-red-700 focus:outline-none focus:ring-2 focus:ring-red-500 disabled:cursor-not-allowed disabled:opacity-35">{client.is_active ? <Icon name="ban" /> : <span className="px-2 text-sm font-bold text-emerald-700">Activate</span>}</button></> : null}</div></td></tr>;
              })}
              {!isLoading && !clients.length ? <tr><td className="px-5 py-14 text-center text-sm text-slate-500" colSpan={6}>No clients match these filters. Try another status or clear your search.</td></tr> : null}
            </tbody>
          </table>
        </div>
        <div className="flex flex-col gap-3 border-t border-slate-100 px-5 py-3 text-sm text-slate-500 sm:flex-row sm:items-center sm:justify-between"><span>Showing {clients.length} of {total} clients</span><div className="flex gap-2"><button type="button" disabled={page === 1} onClick={() => setPage((current) => current - 1)} className="min-h-10 rounded-lg border border-slate-200 px-3 font-bold text-slate-700 transition-colors hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-40">Previous</button><button type="button" disabled={page >= pageCount} onClick={() => setPage((current) => current + 1)} className="min-h-10 rounded-lg border border-slate-200 px-3 font-bold text-slate-700 transition-colors hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-40">Next</button></div></div>
      </div>

      {dialogMode ? <ClientDialogDropdowns serverErrors={serverErrors} mode={dialogMode} client={selectedClient} form={form} formError={formError} isSaving={isSaving} clientTypes={clientTypes} industries={industries} states={states} cities={cities} onClose={() => setDialogMode(null)} onSubmit={saveClient} onField={setField} onContact={updateContact} onAddContact={() => setForm((current) => ({ ...current, contacts: [...current.contacts, emptyContact()] }))} onRemoveContact={removeContact} /> : null}
    </section>
  );
}

function Field({ label, required = false, error = "", children }: { label: string; required?: boolean; error?: string; children: ReactElement<Record<string, unknown>> }) {
  const id = useId();
  return <label className="block text-sm font-bold text-slate-700"><span>{label}{required ? <span aria-hidden="true" className="text-red-600"> *</span> : null}</span>{cloneElement(children, { id, required, "aria-invalid": Boolean(error), "aria-describedby": error ? `${id}-error` : undefined })}{error ? <span id={`${id}-error`} aria-live="polite" className="mt-1 block text-sm font-normal text-red-600">{error}</span> : null}</label>;
}

function AutoSuggestSelect({
  label,
  error,
  placeholder,
  value,
  valueId,
  options,
  disabled = false,
  required = false,
  onSelect,
}: {
  label: string;
  error?: string;
  placeholder: string;
  value: string;
  valueId?: string;
  options: Reference[];
  disabled?: boolean;
  required?: boolean;
  onSelect: (selected: Reference | null, typedName: string) => void;
}) {
  const errorId = useId();
  const [typed, setTyped] = useState<string | null>(null);
  const [open, setOpen] = useState(false);
  const query = typed !== null ? typed : value;

  const activeOptions = options.filter((o) => o.is_active || o.id === valueId);
  const filtered =
    query.trim().length >= 2
      ? activeOptions.filter((o) =>
          o.name.toLowerCase().includes(query.trim().toLowerCase().replace(/^gazi/, "ghazi"))
        )
      : [];

  return (
    <div className="relative block text-sm font-bold text-slate-700">
      <span>
        {label}
        {required ? <span className="text-red-600"> *</span> : null}
      </span>
      <div className="relative mt-1">
        <input
          aria-invalid={Boolean(error)}
          aria-describedby={error ? errorId : undefined}
          required={required}
          aria-label={label}
          disabled={disabled}
          value={query}
          onChange={(e) => {
            const next = e.target.value;
            setTyped(next);
            setOpen(true);
            const exact = activeOptions.find(
              (o) => o.name.toLowerCase() === next.trim().toLowerCase()
            );
            onSelect(exact ?? null, next);
          }}
          onFocus={() => {
            if (query.trim().length >= 2) setOpen(true);
          }}
          placeholder={placeholder}
          className="field w-full"
          autoComplete="off"
        />
        {query ? (
          <button
            type="button"
            disabled={disabled}
            onClick={() => {
              setTyped(null);
              onSelect(null, "");
              setOpen(false);
            }}
            className="absolute right-3 top-1/2 -translate-y-1/2 text-xs font-bold text-slate-400 hover:text-slate-600"
            title="Clear"
          >
            ✕
          </button>
        ) : null}
      </div>
      {error ? <span id={errorId} className="mt-1 block text-sm font-normal text-red-600">{error}</span> : null}
      {open && filtered.length > 0 && !disabled ? (
        <ul className="absolute z-50 mt-1 max-h-48 w-full overflow-y-auto rounded-xl border border-slate-200 bg-white p-1 shadow-xl">
          {filtered.map((item) => (
            <li key={item.id}>
              <button
                type="button"
                className="w-full rounded-lg px-3 py-2 text-left text-xs font-semibold text-slate-700 hover:bg-blue-50 hover:text-blue-700 flex items-center justify-between"
                onClick={() => {
                  setTyped(null);
                  onSelect(item, item.name);
                  setOpen(false);
                }}
              >
                <span>{item.name}</span>
                {item.code ? (
                  <span className="font-mono text-[10px] text-slate-400">{item.code}</span>
                ) : null}
              </button>
            </li>
          ))}
        </ul>
      ) : open && query.trim().length >= 2 && filtered.length === 0 && !disabled ? (
        <div className="absolute z-50 mt-1 w-full rounded-xl border border-slate-200 bg-white p-3 text-xs text-slate-500 shadow-xl">
          No matches found for &quot;{query}&quot;
        </div>
      ) : null}
    </div>
  );
}

function ClientDialogDropdowns({ serverErrors, mode, client, form, formError, isSaving, clientTypes, industries, states, cities, onClose, onSubmit, onField, onContact, onAddContact, onRemoveContact }: { serverErrors: Record<string, string>; mode: Exclude<DialogMode, null>; client: Client | null; form: ClientForm; formError: string; isSaving: boolean; clientTypes: Reference[]; industries: Reference[]; states: Reference[]; cities: Reference[]; onClose: () => void; onSubmit: (event: FormEvent<HTMLFormElement>) => Promise<void>; onField: <Key extends keyof ClientForm>(key: Key, value: ClientForm[Key]) => void; onContact: (index: number, field: keyof ClientContact, value: string | boolean) => void; onAddContact: () => void; onRemoveContact: (index: number) => void }) {
  const readonly = mode === "view";
  const [submitted, setSubmitted] = useState(false);
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const errors = clientErrors(form);
  const fieldError = (key: string, value: string) => serverErrors[key] || (!readonly && (submitted || touched[key] || value.length > 0) ? errors[key] ?? "" : "");

  const selectReference = (field: "client_type" | "industry" | "state" | "city", idField: "client_type_id" | "industry_id" | "state_id" | "city_id", options: Reference[], id: string) => {
    const selected = options.find((item) => item.id === id);
    onField(idField, id || undefined);
    onField(field, selected?.name ?? "");
    if (field === "state") { onField("city_id", undefined); onField("city", ""); }
  };
  return <div role="dialog" aria-modal="true" aria-labelledby="client-dialog-title" className="fixed inset-0 z-50 flex items-end bg-slate-950/45 backdrop-blur-sm sm:items-center sm:justify-center sm:p-5"><form noValidate onBlur={(event) => { const key = ((event.target instanceof HTMLInputElement || event.target instanceof HTMLSelectElement) ? event.target.name : ""); if (key) setTouched((current) => ({ ...current, [key]: true })); }} onSubmit={(event) => { event.preventDefault(); if (readonly) return; setSubmitted(true); if (Object.keys(errors).length) { const element = event.currentTarget; requestAnimationFrame(() => element.querySelector<HTMLInputElement>('[aria-invalid="true"]')?.focus()); return; } void onSubmit(event); }} className="vtpl-modal max-h-[94dvh] w-full overflow-y-auto rounded-t-3xl bg-white p-5 shadow-2xl sm:max-w-5xl sm:rounded-3xl sm:p-7"><div className="flex items-start justify-between gap-4"><div><p className="text-xs font-bold uppercase tracking-[0.14em] text-blue-700">Settings / Client Master</p><h3 id="client-dialog-title" className="mt-1 text-xl font-extrabold text-slate-900">{mode === "create" ? "Create client" : mode === "edit" ? "Edit client" : client?.display_name}</h3><p className="mt-1 text-sm text-slate-500">{readonly ? `${client?.client_code} · Read-only details` : "Choose governed classifications and locations, then add contacts in one place."}</p></div><button type="button" onClick={onClose} aria-label="Close client dialog" className="grid h-10 w-10 place-items-center rounded-xl text-slate-500 hover:bg-slate-100 focus:outline-none focus:ring-2 focus:ring-blue-500"><Icon name="close" /></button></div>{formError ? <div role="alert" className="mt-5 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm font-semibold text-red-800">{formError}</div> : null}<fieldset disabled={readonly} className="mt-6 space-y-6 disabled:opacity-80"><section><h4 className="font-bold text-slate-900">Organisation</h4><p className="mt-1 text-xs text-slate-500">Types and industries are controlled master values, preventing duplicates.</p><div className="mt-4 grid gap-4 md:grid-cols-2 xl:grid-cols-3"><Field label="Legal name" required error={fieldError("legal_name", form.legal_name)}><input name="legal_name" value={form.legal_name} onChange={(event) => onField("legal_name", event.target.value)} autoComplete="organization" className="field" /></Field><Field label="Display name" error={fieldError("display_name", form.display_name)}><input name="display_name" value={form.display_name} onChange={(event) => onField("display_name", event.target.value)} className="field" /></Field><Field label="Parent organisation" error={fieldError("parent_organization", form.parent_organization)}><input name="parent_organization" value={form.parent_organization} onChange={(event) => onField("parent_organization", event.target.value)} className="field" /></Field><Field label="Client type" required error={fieldError("client_type_id", form.client_type_id ?? "") || serverErrors.client_type}><select value={form.client_type_id ?? ""} onChange={(event) => selectReference("client_type", "client_type_id", clientTypes, event.target.value)} className="field"><option value="">Select client type</option>{clientTypes.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></Field><Field label="Industry" required error={fieldError("industry_id", form.industry_id ?? "") || serverErrors.industry}><select value={form.industry_id ?? ""} onChange={(event) => selectReference("industry", "industry_id", industries, event.target.value)} className="field"><option value="">Select industry</option>{industries.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></Field><Field label="Website" error={fieldError("website", form.website)}><input name="website" value={form.website} onChange={(event) => onField("website", event.target.value)} placeholder="https://example.com" inputMode="url" className="field" /></Field></div></section><section className="border-t border-slate-100 pt-6"><h4 className="font-bold text-slate-900">Address and compliance</h4><div className="mt-4 grid gap-4 md:grid-cols-2 xl:grid-cols-3"><Field label="Address line 1" required error={fieldError("address_line_1", form.address_line_1)}><input name="address_line_1" value={form.address_line_1} onChange={(event) => onField("address_line_1", event.target.value)} autoComplete="address-line1" className="field" /></Field><Field label="Address line 2" error={fieldError("address_line_2", form.address_line_2)}><input name="address_line_2" value={form.address_line_2} onChange={(event) => onField("address_line_2", event.target.value)} autoComplete="address-line2" className="field" /></Field><AutoSuggestSelect required error={fieldError("state_id", form.state) || serverErrors.state} label="State / Union Territory" placeholder="Type 2+ letters (e.g. Mah, Del, Raj)..." value={form.state} valueId={form.state_id} options={states} onSelect={(selected, typed) => { onField("state_id", selected?.id); onField("state", selected?.name ?? typed); onField("city_id", undefined); onField("city", ""); }} /><AutoSuggestSelect required error={fieldError("city_id", form.city) || serverErrors.city} key={form.state_id ?? form.state} label="City" placeholder={form.state ? "Type 2+ letters (e.g. Mum, Jpr, Pun)..." : "Select a state first"} value={form.city} valueId={form.city_id} options={cities} disabled={!form.state_id} onSelect={(selected, typed) => { onField("city_id", selected?.id); onField("city", selected?.name ?? typed); }} /><Field label="PIN code" required error={fieldError("postal_code", form.postal_code)}><input name="postal_code" inputMode="numeric" maxLength={6} value={form.postal_code} onChange={(event) => onField("postal_code", event.target.value)} autoComplete="postal-code" className="field" /></Field><Field label="Country" error={fieldError("country", form.country)}><input name="country" value={form.country} onChange={(event) => onField("country", event.target.value)} className="field" /></Field><Field label="GSTIN" required error={fieldError("gstin", form.gstin)}><input name="gstin" value={form.gstin} onChange={(event) => onField("gstin", event.target.value.toUpperCase())} maxLength={15} className="field" /></Field><div><Field label="PAN" required error={fieldError("pan", form.pan)}><input name="pan" value={form.pan} onChange={(event) => onField("pan", event.target.value.toUpperCase())} maxLength={10} className="field" /></Field>{!readonly && !gstinError(form.gstin) && form.pan.trim().toUpperCase() !== form.gstin.trim().toUpperCase().slice(2, 12) ? <button type="button" className="mt-2 min-h-10 text-left text-sm font-semibold text-blue-700 underline underline-offset-2" onClick={() => onField("pan", form.gstin.trim().toUpperCase().slice(2, 12))}>Use PAN from GSTIN: {form.gstin.trim().toUpperCase().slice(2, 12)}</button> : null}</div><Field label="CIN" error={fieldError("cin", form.cin)}><input name="cin" value={form.cin} onChange={(event) => onField("cin", event.target.value.toUpperCase())} maxLength={21} className="field" /></Field></div></section><section className="border-t border-slate-100 pt-6"><div className="flex items-center justify-between gap-3"><div><h4 className="font-bold text-slate-900">Client contacts</h4><p className="mt-1 text-xs text-slate-500">Add EIC, HOD and Site Contact coverage. Multiple contacts per type are allowed; missing types are flagged without blocking creation.</p></div>{!readonly ? <button type="button" onClick={onAddContact} className="min-h-10 rounded-lg border border-blue-200 bg-blue-50 px-3 text-sm font-bold text-blue-700 hover:bg-blue-100"><Icon name="plus" /> Add contact</button> : null}</div><div className="mt-4 space-y-3">{(() => { const missing = Object.keys(contactTypeLabels).filter(type => !form.contacts.some(contact => contact.contact_type === type)); return missing.length ? <p role="status" className="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800">Missing contacts: {missing.map(contactTypeLabel).join(", ")}. You can save the client and complete these later.</p> : null; })()}{submitted && (serverErrors.contacts || errors.contacts) ? <p role="alert" className="text-sm text-red-600">{serverErrors.contacts || errors.contacts}</p> : null}{form.contacts.map((contact, index) => <div key={contact.id ?? `contact-${index}`} className="rounded-2xl border border-slate-200 bg-slate-50 p-4"><div className="mb-3 flex items-center justify-between"><p className="font-bold text-slate-800">{`Contact ${index + 1}${contact.is_primary ? " · Primary" : ""}`}</p>{!readonly ? <button type="button" onClick={() => onRemoveContact(index)} className="text-xs font-bold text-slate-500 underline hover:text-red-700">Remove</button> : null}</div><div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3"><Field label="Full name" required error={fieldError(`contacts.${index}.full_name`, contact.full_name)}><input name={`contacts.${index}.full_name`} value={contact.full_name} onChange={(event) => onContact(index, "full_name", event.target.value)} className="field" /></Field><Field label="Contact type" error={fieldError(`contacts.${index}.contact_type`, contact.contact_type)}><select aria-label="Contact type" name={`contacts.${index}.contact_type`} value={contact.contact_type} onChange={(event) => onContact(index, "contact_type", event.target.value)} className="field"><option value="">Not assigned</option>{Object.entries(contactTypeLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></Field><Field label="Designation" error={fieldError(`contacts.${index}.designation`, contact.designation)}><input name={`contacts.${index}.designation`} value={contact.designation} onChange={(event) => onContact(index, "designation", event.target.value)} className="field" /></Field><Field label="Department (optional)" error={fieldError(`contacts.${index}.department`, contact.department)}><input name={`contacts.${index}.department`} value={contact.department} onChange={(event) => onContact(index, "department", event.target.value)} className="field" /></Field><Field label="Email" required error={fieldError(`contacts.${index}.email`, contact.email)}><input name={`contacts.${index}.email`} value={contact.email} onChange={(event) => onContact(index, "email", event.target.value)} type="email" className="field" /></Field><Field label="Phone" required error={fieldError(`contacts.${index}.phone`, contact.phone)}><input name={`contacts.${index}.phone`} value={contact.phone} onChange={(event) => onContact(index, "phone", event.target.value)} inputMode="tel" className="field" /></Field><label className="flex min-h-11 items-center gap-3 rounded-xl border border-slate-200 bg-white px-3 text-sm font-bold text-slate-700"><input type="checkbox" checked={contact.is_primary} onChange={(event) => onContact(index, "is_primary", event.target.checked)} /> Primary contact</label></div></div>)}</div></section></fieldset><div className="mt-7 flex flex-col-reverse gap-2 border-t border-slate-100 pt-5 sm:flex-row sm:justify-end"><button type="button" onClick={onClose} className="min-h-11 rounded-xl border border-slate-200 px-4 text-sm font-bold text-slate-700">{readonly ? "Close" : "Cancel"}</button>{!readonly ? <button type="submit" disabled={isSaving} className="min-h-11 rounded-xl bg-blue-700 px-5 text-sm font-bold text-white shadow-lg shadow-blue-900/15 hover:bg-blue-800 disabled:opacity-60">{isSaving ? "Saving..." : mode === "create" ? "Create client" : "Save changes"}</button> : null}</div></form></div>;
}
