"use client";
import { displayServerErrors } from "@/components/ui/server-field-errors";
import { ValidatedForm, ValidatedInput, ValidatedLabel, ValidatedSelect, ValidatedTextarea } from "@/components/ui/validated-controls";
import { useCallback, useEffect, useState, type FormEvent } from "react";

import { useAuthenticatedUser } from "@/components/auth-user-provider";
import {
  type Item,
  request,
  input,
  button,
  secondary,
  panel,
} from "./item-data";
import { DataGrid, GridAction, GridStatus } from "@/components/ui/data-grid";
import { RecordDrawer } from "@/components/ui/record-drawer";
import { ItemHistory } from "./item-history";
type Category = {
  id: string;
  code: string;
  name: string;
  is_active: boolean;
  data: {
    kind: string;
    uom?: string;
  };
};
type PaymentTerms = {
  advance_pct: number;
  qc_pct: number;
  delivery_pct: number;
  retention_pct: number;
  credit_days: number;
  description?: string;
};

type SupplierDocument = {
  id: string;
  supplier_id: string;
  doc_type: "aadhar" | "pan" | "gst" | "cheque" | "other";
  title: string;
  original_filename: string;
  content_type: string;
  byte_size: number;
  is_encrypted: boolean;
  is_active: boolean;
  created_at: string;
  created_by_id?: string | null;
};

type Supplier = {
  id: string;
  code: string;
  name: string;
  version: number;
  is_active: boolean;
  data: {
    registration_reference: string;
    contact: string;
    location: string;
    qualification: string;
    category_ids?: string[];
    item_ids: string[];
    contact_person_name?: string;
    email?: string;
    phone?: string;
    aadhar_number?: string;
    pan_number?: string;
    gst_number?: string;
    cancelled_cheque_ref?: string;
    onboarding_verified?: boolean;
    payment_terms?: PaymentTerms;
    notes: string;
  };
};

export function SupplierBank() {
  const user = useAuthenticatedUser();
  const write =
    user.permissions.includes("system:admin") ||
    user.permissions.includes("settings.suppliers.write");
  const [records, setRecords] = useState<Supplier[]>([]);
  const [items, setItems] = useState<Item[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [error, setError] = useState("");
  const [reload, setReload] = useState(0);
  const [loading, setLoading] = useState(true);
  const [edit, setEdit] = useState<Supplier | null | undefined>();
  const [viewId, setViewId] = useState<string | null>(null);
  useEffect(() => {
    let live = true;
    Promise.all([
      request<Supplier[]>("/api/settings/suppliers"),
      request<Item[]>("/api/settings/catalogue-items"),
      request<Category[]>("/api/settings/material-catalogue"),
    ])
      .then(([r, i, c]) => {
        if (live) {
          setRecords(r);
          setItems(i);
          setCategories(c.filter((cat) => cat.is_active && cat.data.kind === "category"));
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

  const [verificationFilter, setVerificationFilter] = useState<"all" | "verified" | "unverified">("all");

  function getNextSupplierCode(existingSuppliers: Supplier[]): string {
    let maxNum = 0;
    for (const s of existingSuppliers) {
      const match = s.code.match(/(\d+)/);
      if (match) {
        const num = parseInt(match[1], 10);
        if (num > maxNum) maxNum = num;
      }
    }
    return `SUP-${String(maxNum + 1).padStart(3, "0")}`;
  }

  const displayedSuppliers = records.filter((r) => {
    if (verificationFilter === "verified") {
      return Boolean(r.data.onboarding_verified || r.data.qualification === "approved");
    }
    if (verificationFilter === "unverified") {
      return !r.data.onboarding_verified && r.data.qualification !== "approved";
    }
    return true;
  });

  return (
    <section className="space-y-5">
      <div className={panel}>
        <h2 className="text-2xl font-bold">Supplier bank</h2>
        
        {write && (
          <button className={button} onClick={() => setEdit(null)}>
            Add supplier
          </button>
        )}
      </div>
      {error && (
        <p role="alert" className="text-red-700">
          {error}
        </p>
      )}
      {loading && <p>Loading suppliers…</p>}
      {edit !== undefined && (
        <RecordDrawer
          title={edit ? "Edit suppliers" : "New suppliers"}
          onClose={() => setEdit(undefined)}
        >
          <SupplierEditor
            key={edit?.id ?? "new"}
            source={edit ?? undefined}
            suggestedCode={!edit ? getNextSupplierCode(records) : undefined}
            items={items}
            categories={categories}
            done={() => {
              setEdit(undefined);
              setReload((r) => r + 1);
            }}
            close={() => setEdit(undefined)}
          />
        </RecordDrawer>
      )}
      <DataGrid<Supplier>
        label="Suppliers"
        rows={displayedSuppliers}
        loading={loading}
        initialSort="code"
        status={(r) => (r.is_active ? "Active" : "Inactive")}
        toolbar={
          <ValidatedLabel className="text-xs font-semibold text-slate-600">
            Verification
            <ValidatedSelect name="verification_filter"
              aria-label="Filter by verification"
              className="mt-1 block min-h-11 rounded-lg border border-slate-200 px-3 text-sm bg-white"
              value={verificationFilter}
              onChange={(e) =>
                setVerificationFilter(e.target.value as "all" | "verified" | "unverified")
              }
            >
              <option value="all">All (Verified & Unverified)</option>
              <option value="verified">Verified only</option>
              <option value="unverified">Unverified only</option>
            </ValidatedSelect>
          </ValidatedLabel>
        }
        columns={[
          {
            key: "code",
            label: "Code",
            value: (r) => r.code,
            className: "font-mono text-xs",
          },
          {
            key: "name",
            label: "Name",
            value: (r) => r.name,
            className: "font-semibold",
          },
          { key: "location", label: "Location", value: (r) => r.data.location },
          {
            key: "qualification",
            label: "Qualification",
            value: (r) => r.data.qualification,
          },
          {
            key: "categories",
            label: "Categories",
            value: (r) => r.data.category_ids?.length ?? 0,
            render: (r) => (
              <span className="text-xs text-slate-700">
                {(r.data.category_ids?.length ?? 0) > 0
                  ? `${r.data.category_ids?.length} categories`
                  : "—"}
              </span>
            ),
          },
          {
            key: "items",
            label: "Linked items",
            value: (r) => r.data.item_ids.length,
          },
          {
            key: "onboarding",
            label: "Onboarding",
            value: (r) => (r.data.onboarding_verified ? "Verified" : "Pending"),
            render: (r) => (
              <GridStatus
                value={r.data.onboarding_verified ? "Active" : "Unverified"}
              />
            ),
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
              label={`View ${r.name}`}
              onClick={() => setViewId(r.id)}
            />
            {write && (
              <GridAction
                kind="edit"
                label={`Edit ${r.name}`}
                onClick={() => setEdit(r)}
              />
            )}
          </>
        )}
      />
      {viewId && (
        <RecordDrawer title="Suppliers details" onClose={() => setViewId(null)}>
          {" "}
          <div className="space-y-4">
            {records
              .filter((r) => r.id === viewId)
              .map((r) => (
                <article className={panel} key={r.id}>
                  <h3 className="font-bold">
                    {r.name} · {r.code}
                  </h3>
                  <p className="text-sm">
                    {r.data.qualification} ·{" "}
                    {r.is_active ? "Active" : "Inactive"} · {r.data.location}
                  </p>
                  <p className="text-sm">{r.data.contact}</p>
                  <div className="rounded border border-slate-200 bg-slate-50 p-3 text-xs space-y-1">
                    <div className="flex justify-between items-center pb-1 border-b border-slate-200">
                      <span className="font-semibold text-slate-700">Onboarding Status</span>
                      <span className={r.data.onboarding_verified ? "font-bold text-emerald-700" : "font-bold text-amber-700"}>
                        {r.data.onboarding_verified ? "✓ Verified & Approved" : "⏳ Pending Documents"}
                      </span>
                    </div>
                    {r.data.contact_person_name && <p><strong>Contact Person:</strong> {r.data.contact_person_name} ({r.data.phone || r.data.contact})</p>}
                    {r.data.email && <p><strong>Email:</strong> {r.data.email}</p>}
                    {r.data.pan_number && <p><strong>PAN:</strong> {r.data.pan_number} {r.data.gst_number ? `· GST: ${r.data.gst_number}` : ""}</p>}
                    {r.data.aadhar_number && <p><strong>Aadhar:</strong> {r.data.aadhar_number}</p>}
                    {r.data.cancelled_cheque_ref && <p><strong>Bank/Cheque:</strong> {r.data.cancelled_cheque_ref}</p>}
                  </div>

                  {/* Payment Terms Summary */}
                  <div className="rounded border border-slate-200 bg-slate-50 p-3 text-xs space-y-2">
                    <div className="flex justify-between items-center pb-1 border-b border-slate-200">
                      <span className="font-semibold text-slate-700">Payment Terms</span>
                      <span className="font-medium text-slate-600">
                        Credit Period: {r.data.payment_terms?.credit_days ?? 0} days
                      </span>
                    </div>
                    <div className="grid grid-cols-2 sm:grid-cols-4 gap-2 pt-1">
                      <div className="rounded bg-white p-2 border border-slate-200">
                        <span className="text-slate-500 block text-[11px]">Advance</span>
                        <span className="font-bold text-slate-900 text-sm">{r.data.payment_terms?.advance_pct ?? 0}%</span>
                      </div>
                      <div className="rounded bg-white p-2 border border-slate-200">
                        <span className="text-slate-500 block text-[11px]">After QC</span>
                        <span className="font-bold text-slate-900 text-sm">{r.data.payment_terms?.qc_pct ?? 0}%</span>
                      </div>
                      <div className="rounded bg-white p-2 border border-slate-200">
                        <span className="text-slate-500 block text-[11px]">After Delivery</span>
                        <span className="font-bold text-slate-900 text-sm">{r.data.payment_terms?.delivery_pct ?? 0}%</span>
                      </div>
                      <div className="rounded bg-white p-2 border border-slate-200">
                        <span className="text-slate-500 block text-[11px]">Retention</span>
                        <span className="font-bold text-slate-900 text-sm">{r.data.payment_terms?.retention_pct ?? 0}%</span>
                      </div>
                    </div>
                    {r.data.payment_terms?.description && (
                      <p className="text-[11px] text-slate-600 italic pt-1">Note: {r.data.payment_terms.description}</p>
                    )}
                  </div>

                  {/* Encrypted Documents */}
                  <SupplierDocumentsManager
                    supplierId={r.id}
                    supplierName={r.name}
                    write={write}
                  />

                  <details open>
                    <summary className="font-semibold text-sm cursor-pointer">
                      Supplied Categories ({r.data.category_ids?.length ?? 0})
                    </summary>
                    <div className="flex flex-wrap gap-1 pt-2">
                      {(r.data.category_ids ?? []).map((cid) => {
                        const cat = categories.find((c) => c.id === cid);
                        return (
                          <span
                            key={cid}
                            className="rounded bg-blue-50 border border-blue-200 px-2 py-0.5 text-xs font-medium text-blue-800"
                          >
                            {cat ? `${cat.code} · ${cat.name}` : cid}
                          </span>
                        );
                      })}
                      {(!r.data.category_ids || r.data.category_ids.length === 0) && (
                        <span className="text-xs text-slate-500 italic">No categories mapped yet.</span>
                      )}
                    </div>
                  </details>
                  <details>
                    <summary className="font-semibold text-sm cursor-pointer">
                      Supplied SKUs ({r.data.item_ids.length})
                    </summary>
                    <ul className="space-y-2 pt-3 text-sm">
                      {r.data.item_ids.map((id) => (
                        <li key={id} className="break-all text-xs">
                          {items.find((i) => i.id === id)?.description || items.find((i) => i.id === id)?.sku || id}
                        </li>
                      ))}
                    </ul>
                  </details>
                  {write && (
                    <button
                      className={secondary}
                      onClick={() => {
                        setViewId(null);
                        setEdit(r);
                      }}
                    >
                      Edit supplier
                    </button>
                  )}
                  <ItemHistory
                    url={`/api/settings/suppliers/${r.id}/history`}
                  />
                </article>
              ))}
          </div>
        </RecordDrawer>
      )}

      {!loading && !records.length && <p>No suppliers registered yet.</p>}
    </section>
  );
}

function SupplierDocumentsManager({
  supplierId,
  supplierName,
  write,
}: {
  supplierId: string;
  supplierName: string;
  write: boolean;
}) {
  const [docs, setDocs] = useState<SupplierDocument[]>([]);
  const [loading, setLoading] = useState(true);
  const [uploading, setUploading] = useState(false);
  const [docType, setDocType] = useState<
    "aadhar" | "pan" | "gst" | "cheque" | "other"
  >("gst");
  const [title, setTitle] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");

  const loadDocs = useCallback(() => {
    fetch(`/api/settings/suppliers/${supplierId}/documents`)
      .then((r) => {
        if (!r.ok) throw new Error("Failed to load documents");
        return r.json();
      })
      .then((data) => {
        setDocs(data);
        setLoading(false);
      })
      .catch((e) => {
        setError(e.message);
        setLoading(false);
      });
  }, [supplierId]);

  useEffect(() => {
    loadDocs();
  }, [loadDocs]);


  const handleUpload = async (e: FormEvent) => {
    e.preventDefault();
    if (!file) return;
    setUploading(true);
    setError("");
    setSuccess("");
    try {
      const fd = new FormData();
      fd.append("file", file);
      fd.append("doc_type", docType);
      fd.append("title", title || file.name);
      const res = await fetch(
        `/api/settings/suppliers/${supplierId}/documents`,
        {
          method: "POST",
          body: fd,
        },
      );
      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        throw new Error(err.detail || err.message || "Upload failed");
      }
      setSuccess(`✓ Successfully uploaded and encrypted ${file.name}`);
      setFile(null);
      setTitle("");
      const fileInput = document.getElementById(
        `doc-upload-${supplierId}`,
      ) as HTMLInputElement;
      if (fileInput) fileInput.value = "";
      loadDocs();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setUploading(false);
    }
  };

  const handleDownload = async (doc: SupplierDocument) => {
    try {
      const res = await fetch(
        `/api/settings/suppliers/${supplierId}/documents/${doc.id}/download`,
      );
      if (!res.ok) throw new Error("Failed to download document");
      const blob = await res.blob();
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = doc.original_filename;
      document.body.appendChild(a);
      a.click();
      window.URL.revokeObjectURL(url);
      document.body.removeChild(a);
    } catch (e) {
      alert((e as Error).message);
    }
  };

  const handleDelete = async (docId: string, filename: string) => {
    if (!confirm(`Are you sure you want to deactivate ${filename}?`)) return;
    try {
      const res = await fetch(
        `/api/settings/suppliers/${supplierId}/documents/${docId}`,
        {
          method: "DELETE",
        },
      );
      if (!res.ok) throw new Error("Failed to delete document");
      loadDocs();
    } catch (e) {
      setError((e as Error).message);
    }
  };

  const getDocBadge = (type: string) => {
    switch (type) {
      case "aadhar":
        return (
          <span className="rounded bg-amber-100 text-amber-800 px-1.5 py-0.5 text-xs font-semibold">
            AADHAR
          </span>
        );
      case "pan":
        return (
          <span className="rounded bg-indigo-100 text-indigo-800 px-1.5 py-0.5 text-xs font-semibold">
            PAN
          </span>
        );
      case "gst":
        return (
          <span className="rounded bg-blue-100 text-blue-800 px-1.5 py-0.5 text-xs font-semibold">
            GST CERT
          </span>
        );
      case "cheque":
        return (
          <span className="rounded bg-emerald-100 text-emerald-800 px-1.5 py-0.5 text-xs font-semibold">
            CHEQUE
          </span>
        );
      default:
        return (
          <span className="rounded bg-slate-100 text-slate-800 px-1.5 py-0.5 text-xs font-semibold">
            {type.toUpperCase()}
          </span>
        );
    }
  };

  return (
    <div className="rounded-lg border border-slate-200 bg-white p-3 space-y-3">
      <div className="flex items-center justify-between pb-2 border-b border-slate-200">
        <div>
          <h4 className="font-semibold text-sm text-slate-900 flex items-center gap-1.5">
            🔒 Onboarding Documents · {supplierName}
            <span className="rounded bg-emerald-50 border border-emerald-200 text-emerald-800 text-[10px] font-mono px-1.5 py-0.5">
              AES-256-GCM
            </span>
          </h4>

          <p className="text-xs text-slate-500">
            Encrypted storage. Raw documents cannot be read from disk or database dumps.
          </p>
        </div>
        <span className="text-xs text-slate-500 font-medium">
          {docs.length} {docs.length === 1 ? "document" : "documents"}
        </span>
      </div>

      {error && <p className="text-xs text-red-700 bg-red-50 p-2 rounded">{error}</p>}
      {success && (
        <p className="text-xs text-emerald-700 bg-emerald-50 p-2 rounded">
          {success}
        </p>
      )}

      {loading && <p className="text-xs text-slate-500">Loading documents…</p>}

      {!loading && docs.length === 0 && (
        <p className="text-xs text-slate-400 italic py-2">
          No onboarding documents uploaded yet.
        </p>
      )}

      {docs.length > 0 && (
        <ul className="space-y-2 max-h-52 overflow-auto pr-1">
          {docs.map((doc) => (
            <li
              key={doc.id}
              className="flex items-center justify-between gap-2 p-2 rounded border border-slate-200 bg-slate-50 text-xs hover:bg-slate-100 transition-colors"
            >
              <div className="min-w-0 flex items-center gap-2">
                {getDocBadge(doc.doc_type)}
                <div className="truncate">
                  <p
                    className="font-semibold text-slate-800 truncate"
                    title={doc.title}
                  >
                    {doc.title || doc.original_filename}
                  </p>
                  <p className="text-[11px] text-slate-500">
                    {doc.original_filename} · {(doc.byte_size / 1024).toFixed(1)} KB
                    · {new Date(doc.created_at).toLocaleDateString()}
                  </p>
                </div>
              </div>
              <div className="flex items-center gap-1.5 shrink-0">
                <button
                  type="button"
                  onClick={() => handleDownload(doc)}
                  className="rounded bg-white border border-slate-300 px-2 py-1 text-[11px] font-semibold text-slate-700 hover:bg-slate-100 shadow-sm"
                  title="Decrypt and download file"
                >
                  📥 Download
                </button>
                {write && (
                  <button
                    type="button"
                    onClick={() => handleDelete(doc.id, doc.original_filename)}
                    className="rounded bg-white border border-red-200 px-1.5 py-1 text-[11px] font-semibold text-red-600 hover:bg-red-50"
                    title="Deactivate document"
                  >
                    ✕
                  </button>
                )}
              </div>
            </li>
          ))}
        </ul>
      )}

      {write && (
        <ValidatedForm
          onSubmit={handleUpload}
          className="rounded border border-dashed border-slate-300 bg-slate-50 p-2.5 space-y-2 mt-2"
        >
          <p className="text-xs font-semibold text-slate-700">
            Upload New Onboarding Document
          </p>
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
            <div>
              <ValidatedLabel className="text-[11px] text-slate-600 block mb-0.5">
                Document Type

              <ValidatedSelect name="doc_type"
                className={input}
                value={docType}
                onChange={(e) =>
                  setDocType(e.target.value as SupplierDocument["doc_type"])
                }

              >
                <option value="aadhar">Aadhar Card</option>
                <option value="pan">PAN Card</option>
                <option value="gst">GST Registration</option>
                <option value="cheque">Cancelled Cheque</option>
                <option value="other">Other Document</option>
              </ValidatedSelect></ValidatedLabel>
            </div>
            <div>
              <ValidatedLabel className="text-[11px] text-slate-600 block mb-0.5">
                Title / Note (optional)

              <ValidatedInput name="title"
                className={input}
                placeholder="e.g. GST Certificate 2026"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
              /></ValidatedLabel>
            </div>
            <div>
              <ValidatedLabel className="text-[11px] text-slate-600 block mb-0.5">
                Select File (PDF/Image)

              <ValidatedInput
                id={`doc-upload-${supplierId}`}
                type="file"
                required
                className="w-full text-xs file:mr-2 file:py-1 file:px-2 file:rounded file:border-0 file:text-xs file:font-semibold file:bg-blue-50 file:text-blue-700 hover:file:bg-blue-100"
                onChange={(e) => setFile(e.target.files?.[0] ?? null)}
              /></ValidatedLabel>
            </div>
          </div>
          <div className="flex justify-end pt-1">
            <button
              type="submit"
              disabled={uploading || !file}
              className="rounded bg-slate-900 px-3 py-1.5 text-xs font-semibold text-white hover:bg-slate-800 disabled:opacity-50"
            >
              {uploading ? "Encrypting & Uploading…" : "🔒 Encrypt & Upload"}
            </button>
          </div>
        </ValidatedForm>
      )}
    </div>
  );
}

function SupplierEditor({
  source,
  suggestedCode,
  items,
  categories,
  done,
  close,
}: {
  source?: Supplier;
  suggestedCode?: string;
  items: Item[];
  categories: Category[];
  done: () => void;
  close: () => void;
}) {
  const [code, setCode] = useState(source?.code ?? suggestedCode ?? "");
  const [name, setName] = useState(source?.name ?? "");
  const [data, setData] = useState<Supplier["data"]>(() => ({
    registration_reference: "",
    contact: "",
    location: "",
    qualification: "unverified",
    category_ids: [],
    item_ids: [],
    contact_person_name: "",
    email: "",
    phone: "",
    aadhar_number: "",
    pan_number: "",
    gst_number: "",
    cancelled_cheque_ref: "",
    onboarding_verified: false,
    notes: "",
    ...(source?.data ?? {}),
    payment_terms: {
      advance_pct: 0,
      qc_pct: 0,
      delivery_pct: 0,
      retention_pct: 0,
      credit_days: 0,
      description: "",
      ...(source?.data?.payment_terms ?? {}),
    },
  }));
  const [reason, setReason] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const updateTerms = (
    field: keyof PaymentTerms,
    value: number | string,
  ) => {

    setData((prev) => ({
      ...prev,
      payment_terms: {
        advance_pct: 0,
        qc_pct: 0,
        delivery_pct: 0,
        retention_pct: 0,
        credit_days: 0,
        description: "",
        ...(prev.payment_terms ?? {}),
        [field]: value,
      },
    }));
  };

  const terms = data.payment_terms;
  const termsTotal = terms
    ? (terms.advance_pct || 0) +
      (terms.qc_pct || 0) +
      (terms.delivery_pct || 0) +
      (terms.retention_pct || 0)
    : 0;

  async function save(e: FormEvent) {
    const submittedForm = e.currentTarget instanceof HTMLFormElement ? e.currentTarget : undefined;
    e.preventDefault();
    if (!reason.trim()) {
      setError("A change reason is required for audit traceability.");
      return;
    }
    if (terms) {
      const total =
        (terms.advance_pct || 0) +
        (terms.qc_pct || 0) +
        (terms.delivery_pct || 0) +
        (terms.retention_pct || 0);
      if (total > 0 && Math.abs(total - 100) > 0.01) {
        setError(
          `Payment terms percentages must sum to 100% (currently ${total.toFixed(1)}%)`,
        );
        return;
      }
    }
    setBusy(true);
    try {
      await request(
        `/api/settings/suppliers${source ? `/${source.id}/profile` : ""}`,
        {
          ...(source ? { version: source.version } : { code }),
          name,
          data,
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
      <h3 className="font-bold">{source ? "Edit supplier" : "New supplier"}</h3>
      {error && (
        <p role="alert" className="text-red-700">
          {error}
        </p>
      )}
      <fieldset disabled={busy} className="space-y-4">
        <div className="grid gap-3 sm:grid-cols-2">
          <ValidatedLabel>
            Supplier code
            <ValidatedInput name="code"
              className={input}
              required
              disabled={!!source}
              pattern="[A-Z0-9_-]{1,16}"
              value={code}
              onChange={(e) => setCode(e.target.value.toUpperCase())}
            />
          </ValidatedLabel>
          <ValidatedLabel>
            Supplier name
            <ValidatedInput name="name"
              className={input}
              required
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </ValidatedLabel>
          {(["registration_reference", "contact", "location"] as const).map(
            (k) => (
              <ValidatedLabel key={k}>
                {k.replaceAll("_", " ")}
                <ValidatedInput
                  className={input}
                  value={data[k]}
                  onChange={(e) => setData({ ...data, [k]: e.target.value })}
                />
              </ValidatedLabel>
            ),
          )}
          <ValidatedLabel>
            Qualification
            <ValidatedSelect name="qualification"
              className={input}
              value={data.qualification}
              onChange={(e) =>
                setData({ ...data, qualification: e.target.value })
              }
            >
              {["unverified", "approved", "suspended"].map((s) => (
                <option key={s}>{s}</option>
              ))}
            </ValidatedSelect>
          </ValidatedLabel>
        </div>
        <ValidatedLabel className="block">
          Qualification notes / evidence reference
          <ValidatedTextarea name="notes"
            className={input}
            value={data.notes}
            onChange={(e) => setData({ ...data, notes: e.target.value })}
          />
        </ValidatedLabel>

        {/* Mapped Categories */}
        <fieldset className="rounded-lg border border-slate-200 bg-slate-50 p-3 space-y-2">
          <legend className="font-semibold text-sm text-slate-800 px-1">
            Supplied Categories ({data.category_ids?.length ?? 0} selected)
          </legend>
          <p className="text-xs text-slate-500">
            One vendor can supply multiple categories. Selecting a category makes this vendor eligible for all items in it.
          </p>
          <div className="grid grid-cols-2 sm:grid-cols-3 gap-2 max-h-48 overflow-auto pt-1">
            {categories.map((cat) => {
              const checked = (data.category_ids ?? []).includes(cat.id);
              return (
                <ValidatedLabel
                  key={cat.id}
                  className="flex items-center gap-2 rounded border border-slate-200 bg-white p-2 text-xs cursor-pointer hover:bg-blue-50"
                >
                  <ValidatedInput
                    type="checkbox"
                    checked={checked}
                    onChange={() => {
                      const cur = data.category_ids ?? [];
                      setData({
                        ...data,
                        category_ids: checked
                          ? cur.filter((id) => id !== cat.id)
                          : [...cur, cat.id],
                      });
                    }}
                  />
                  <span className="truncate" title={`${cat.code} · ${cat.name}`}>
                    <strong>{cat.code}</strong> {cat.name}
                  </span>
                </ValidatedLabel>
              );
            })}
            {!categories.length && (
              <p className="text-xs text-slate-500 col-span-full">No active categories found in catalogue.</p>
            )}
          </div>
        </fieldset>

        {/* 7 Mandatory Onboarding Documents */}
        <fieldset className="rounded-lg border border-slate-200 bg-slate-50 p-3 space-y-3">
          <div className="flex justify-between items-center">
            <legend className="font-semibold text-sm text-slate-800 px-1">
              Vendor Onboarding Documents (7 Mandatory Records)
            </legend>
            <ValidatedLabel className="flex items-center gap-2 text-xs font-semibold text-slate-800 bg-white border border-slate-200 rounded px-2 py-1 cursor-pointer">
              <ValidatedInput
                type="checkbox"
                checked={data.onboarding_verified ?? false}
                onChange={(e) =>
                  setData({ ...data, onboarding_verified: e.target.checked })
                }
              />
              ✓ Verified & Approved
            </ValidatedLabel>
          </div>
          <p className="text-xs text-slate-500">
            Required for new vendor qualification and release of Letter of Intent (LOI).
          </p>
          <div className="grid gap-3 sm:grid-cols-2">
            <ValidatedLabel className="text-xs">
              1. Contact Person Name
              <ValidatedInput
                className={input}
                value={data.contact_person_name ?? ""}
                onChange={(e) =>
                  setData({ ...data, contact_person_name: e.target.value })
                }
                placeholder="e.g. Rahul Sharma"
              />
            </ValidatedLabel>
            <ValidatedLabel className="text-xs">
              2. Mail ID
              <ValidatedInput
                type="email"
                className={input}
                value={data.email ?? ""}
                onChange={(e) => setData({ ...data, email: e.target.value })}
                placeholder="vendor@company.com"
              />
            </ValidatedLabel>
            <ValidatedLabel className="text-xs">
              3. Phone Number
              <ValidatedInput
                className={input}
                value={data.phone ?? ""}
                onChange={(e) => setData({ ...data, phone: e.target.value })}
                placeholder="+91 98765 43210"
              />
            </ValidatedLabel>
            <ValidatedLabel className="text-xs">
              4. Aadhar Card Ref / No.
              <ValidatedInput
                className={input}
                value={data.aadhar_number ?? ""}
                onChange={(e) =>
                  setData({ ...data, aadhar_number: e.target.value })
                }
                placeholder="XXXX-XXXX-XXXX"
              />
            </ValidatedLabel>
            <ValidatedLabel className="text-xs">
              5. PAN Card Number
              <ValidatedInput
                className={input}
                value={data.pan_number ?? ""}
                onChange={(e) =>
                  setData({ ...data, pan_number: e.target.value.toUpperCase() })
                }
                placeholder="ABCDE1234F"
              />
            </ValidatedLabel>
            <ValidatedLabel className="text-xs">
              6. GST Registration Certificate / GSTIN
              <ValidatedInput
                className={input}
                value={data.gst_number ?? ""}
                onChange={(e) =>
                  setData({ ...data, gst_number: e.target.value.toUpperCase() })
                }
                placeholder="07AAAAA0000A1Z5"
              />
            </ValidatedLabel>
            <ValidatedLabel className="text-xs sm:col-span-2">
              7. Cancelled Cheque Ref / Bank Details
              <ValidatedInput
                className={input}
                value={data.cancelled_cheque_ref ?? ""}
                onChange={(e) =>
                  setData({ ...data, cancelled_cheque_ref: e.target.value })
                }
                placeholder="e.g. HDFC Bank, A/c 50100..., IFSC HDFC000..."
              />
            </ValidatedLabel>
          </div>
        </fieldset>

        {/* Vendor Payment Terms */}
        <fieldset className="rounded-lg border border-slate-200 bg-slate-50 p-3 space-y-3">
          <div className="flex justify-between items-center">
            <legend className="font-semibold text-sm text-slate-800 px-1">
              Vendor Payment Terms & Credit Period
            </legend>
            <span
              className={`text-xs font-bold px-2 py-0.5 rounded ${
                termsTotal === 100
                  ? "bg-emerald-100 text-emerald-800"
                  : termsTotal === 0
                  ? "bg-slate-200 text-slate-700"
                  : "bg-amber-100 text-amber-800"
              }`}
            >
              {termsTotal === 100
                ? "✓ 100% Allocated"
                : termsTotal === 0
                ? "Initial (0%)"
                : `Total: ${termsTotal.toFixed(1)}% (Must equal 100%)`}
            </span>
          </div>
          <p className="text-xs text-slate-500">
            Define milestone release percentages. When specified, advance + QC + delivery + retention must equal 100%.
          </p>
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <ValidatedLabel className="text-xs">
              1. Advance (%)
              <ValidatedInput
                type="number"
                min="0"
                max="100"
                step="0.1"
                className={input}
                value={data.payment_terms?.advance_pct ?? 0}
                onChange={(e) =>
                  updateTerms("advance_pct", parseFloat(e.target.value) || 0)
                }
              />
            </ValidatedLabel>
            <ValidatedLabel className="text-xs">
              2. After Quality Check (%)
              <ValidatedInput
                type="number"
                min="0"
                max="100"
                step="0.1"
                className={input}
                value={data.payment_terms?.qc_pct ?? 0}
                onChange={(e) =>
                  updateTerms("qc_pct", parseFloat(e.target.value) || 0)
                }
              />
            </ValidatedLabel>
            <ValidatedLabel className="text-xs">
              3. After Delivery (%)
              <ValidatedInput
                type="number"
                min="0"
                max="100"
                step="0.1"
                className={input}
                value={data.payment_terms?.delivery_pct ?? 0}
                onChange={(e) =>
                  updateTerms("delivery_pct", parseFloat(e.target.value) || 0)
                }
              />
            </ValidatedLabel>
            <ValidatedLabel className="text-xs">
              4. Retention / Warranty (%)
              <ValidatedInput
                type="number"
                min="0"
                max="100"
                step="0.1"
                className={input}
                value={data.payment_terms?.retention_pct ?? 0}
                onChange={(e) =>
                  updateTerms("retention_pct", parseFloat(e.target.value) || 0)
                }
              />
            </ValidatedLabel>
            <ValidatedLabel className="text-xs sm:col-span-2">
              Credit Period (Days after bill/dispatch)
              <ValidatedInput
                type="number"
                min="0"
                max="365"
                className={input}
                value={data.payment_terms?.credit_days ?? 0}
                onChange={(e) =>
                  updateTerms("credit_days", parseInt(e.target.value) || 0)
                }
                placeholder="e.g. 30"
              />
            </ValidatedLabel>
            <ValidatedLabel className="text-xs sm:col-span-2">
              Payment Terms Notes
              <ValidatedInput
                type="text"
                className={input}
                value={data.payment_terms?.description ?? ""}
                onChange={(e) => updateTerms("description", e.target.value)}
                placeholder="e.g. 20% advance with PO, balance against dispatch"
              />
            </ValidatedLabel>
          </div>
        </fieldset>

        {/* Existing Uploaded Documents for this Supplier */}
        {source && (
          <div className="pt-2">
            <SupplierDocumentsManager
              supplierId={source.id}
              supplierName={name}
              write={true}
            />
          </div>
        )}


        {/* Specific Item Overrides */}
        <fieldset>
          <legend className="font-semibold text-sm">Specific Supplied SKUs (Optional Item Overrides)</legend>
          <div className="max-h-56 overflow-auto space-y-2 pt-1">
            {items
              .filter(
                (i) => i.status === "approved" || data.item_ids.includes(i.id),
              )
              .map((i) => (
                <ValidatedLabel
                  className="flex items-start gap-2 rounded-lg bg-slate-50 p-2 text-sm"
                  key={i.id}
                >
                  <ValidatedInput
                    type="checkbox"
                    checked={data.item_ids.includes(i.id)}
                    onChange={() =>
                      setData({
                        ...data,
                        item_ids: data.item_ids.includes(i.id)
                          ? data.item_ids.filter((id) => id !== i.id)
                          : [...data.item_ids, i.id],
                      })
                    }
                  />
                  <span className="min-w-0 break-words text-xs">
                    {i.description}
                    <span className="block break-all text-xs text-blue-700">
                      {i.sku}
                    </span>
                  </span>
                </ValidatedLabel>
              ))}
          </div>
        </fieldset>
        <ValidatedLabel className="block text-xs font-bold text-slate-700">
          Change reason <span className="text-red-600 font-bold">*</span>
          <ValidatedInput name="reason"
            className={input}
            required
            placeholder="Audit reason for creating or modifying this supplier..."
            value={reason}
            onChange={(e) => setReason(e.target.value)}
          />
        </ValidatedLabel>
        <div className="flex flex-wrap gap-2">
          <button className={button}>Save supplier</button>
          {source && (
            <button
              type="button"
              className={secondary}
              disabled={!reason.trim()}
              onClick={async () => {
                setBusy(true);
                try {
                  await request(
                    `/api/settings/suppliers/${source.id}`,
                    {
                      version: source.version,
                      is_active: !source.is_active,
                      reason,
                    },
                    "PATCH",
                  );
                  done();
                } catch (e) {
                  setError((e as Error).message);
                } finally {
                  setBusy(false);
                }
              }}
            >
              {source.is_active ? "Deactivate" : "Reactivate"}
            </button>
          )}
          <button type="button" className={secondary} onClick={close}>
            Cancel
          </button>
        </div>
      </fieldset>
    </ValidatedForm>
  );
}
