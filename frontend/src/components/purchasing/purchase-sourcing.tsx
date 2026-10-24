"use client";
import { displayServerErrors } from "@/components/ui/server-field-errors";
import { ValidatedForm, ValidatedInput, ValidatedLabel, ValidatedSelect, ValidatedTextarea } from "@/components/ui/validated-controls";
import { useEffect, useState } from "react";
import { DataGrid } from "@/components/ui/data-grid";
import { RecordDrawer } from "@/components/ui/record-drawer";
import { request, input, button, secondary } from "./purchase-data";
import type { SourcingOffer, SourcingWorkspace } from "./sourcing-data";
import { sourcingDocument } from "./sourcing-documents";

export function PurchaseSourcing({
  base,
  step,
  dirty,
  revision = 0,
}: {
  base: string;
  step: number;
  dirty: (v: boolean) => void;
  revision?: number;
}) {
  const [data, setData] = useState<SourcingWorkspace | null>(null),
    [error, setError] = useState(""),
    [reload, setReload] = useState(0);
  const [form, setForm] = useState<{
    action: string;
    offer?: SourcingOffer;
  } | null>(null);
  const [changed, setChanged] = useState(false);
  useEffect(() => {
    let live = true;
    request<SourcingWorkspace>(base + "/sourcing")
      .then((w) => {
        if (live) {
          setData(w);
          setError("");
        }
      })
      .catch((e) => {
        if (live) setError(e.message);
      });
    return () => {
      live = false;
    };
  }, [base, reload, revision]);
  function close() {
    if (!changed || window.confirm("Discard unsaved sourcing changes?")) {
      setForm(null);
      setChanged(false);
      dirty(false);
    }
  }
  if (!data)
    return (
      <section className="mt-5 rounded-xl border p-4">
        <p role={error ? "alert" : "status"}>
          {error || "Loading supplier sourcing…"}
        </p>
        <button className={secondary} onClick={() => setReload((n) => n + 1)}>
          Retry sourcing
        </button>
      </section>
    );
  const c = data.case,
    can = (action: string) => data.actions.includes(action),
    draft = c?.state === "draft";
  const name = (id: string | null) =>
    data.approvers.find((a) => a.id === id)?.label ||
    "Unassigned or former member";
  return (
    <section
      className="mt-5 space-y-4 border-t pt-5"
      aria-label="Supplier sourcing"
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h4 className="text-lg font-bold">RFQ → comparison → LOI</h4>
          <p className="text-sm text-slate-600">
            {c
              ? `${c.state.replaceAll("_", " ")} · version ${c.version} · Approver: ${name(c.approver_id)}`
              : "Configure an independent approver to begin."}
          </p>
        </div>
        <button className={secondary} onClick={() => setReload((n) => n + 1)}>
          Refresh sourcing
        </button>
      </div>
      <p className="rounded-lg bg-blue-50 p-3 text-sm">
        Budget: {data.source.currency} {data.source.budget} for{" "}
        {data.source.quantity} {data.source.uom}. Quotes must cover the complete
        released quantity and exact specified item. External sending is manual.
      </p>
      {error && <p role="alert">{error}</p>}
      <div className="flex flex-wrap gap-2">
        {can("configure") && (!c || draft) && (
          <button
            className={secondary}
            onClick={() => setForm({ action: "configure" })}
          >
            {c ? "Change approver" : "Configure sourcing"}
          </button>
        )}
        {step === 3 && draft && can("rfq") && (
          <button className={button} onClick={() => setForm({ action: "rfq" })}>
            Prepare supplier RFQ
          </button>
        )}
        {c && (
          <button
            className={secondary}
            onClick={() => sourcingDocument(data, "CST")}
          >
            Download comparison
          </button>
        )}
        {c &&
          ["pending", "approved"].includes(c.state) &&
          step === 4 &&
          can("reopen") && (
            <button
              className={secondary}
              onClick={() => setForm({ action: "reopen" })}
            >
              Return to draft
            </button>
          )}
      </div>
      {c && (
        <DataGrid
          label="Supplier offers"
          rows={c.offers}
          status={(o) => o.state}
          columns={[
            {
              key: "supplier",
              label: "Supplier",
              value: (o) => o.supplier_snapshot.name,
            },
            { key: "state", label: "Status", value: (o) => o.state },
            {
              key: "due",
              label: "Due / validity",
              value: (o) => o.valid_until || o.due_date,
            },
            {
              key: "total",
              label: `Landed (${data.source.currency})`,
              value: (o) =>
                data.comparison.find((r) => r.offer_id === o.id)?.total || "—",
            },
            {
              key: "rank",
              label: "Rank / review",
              value: (o) => {
                const r = data.comparison.find((r) => r.offer_id === o.id);
                return r
                  ? (r.expired ? "Expired · " : "") +
                      (r.rank ? "L" + r.rank : "Noncompliant")
                  : "Awaiting quote";
              },
            },
            ...(step === 4
              ? [
                  {
                    key: "variance",
                    label: "Budget variance",
                    value: (o: SourcingOffer) =>
                      data.comparison.find((r) => r.offer_id === o.id)
                        ?.variance || "—",
                  },
                  {
                    key: "delivery",
                    label: "Delivery days",
                    value: (o: SourcingOffer) =>
                      o.state === "quoted" ? o.delivery_days : "—",
                  },
                  {
                    key: "payment",
                    label: "Payment terms",
                    value: (o: SourcingOffer) => o.payment_terms || "—",
                  },
                  {
                    key: "warranty",
                    label: "Warranty",
                    value: (o: SourcingOffer) => o.warranty || "—",
                  },
                ]
              : []),
          ]}
          actions={(o) => (
            <div className="flex flex-wrap gap-2">
              <button
                className={secondary}
                onClick={() => setForm({ action: "view", offer: o })}
              >
                View
              </button>
              {step === 3 && draft && o.state === "draft" && can("rfq") && (
                <button
                  className={secondary}
                  onClick={() => setForm({ action: "rfq", offer: o })}
                >
                  Edit RFQ
                </button>
              )}
              {step === 3 &&
                draft &&
                o.state === "draft" &&
                can("dispatch") && (
                  <button
                    className={secondary}
                    onClick={() => setForm({ action: "dispatch", offer: o })}
                  >
                    Record sent
                  </button>
                )}
              {step === 3 && draft && o.state !== "no_bid" && can("no_bid") && (
                <button
                  className={secondary}
                  onClick={() => setForm({ action: "no_bid", offer: o })}
                >
                  No bid
                </button>
              )}
              {step === 4 && draft && o.sent_date && can("quote") && (
                <button
                  className={secondary}
                  onClick={() => setForm({ action: "quote", offer: o })}
                >
                  {o.state === "quoted" ? "Revise quote" : "Enter quote"}
                </button>
              )}
              {step === 4 &&
                draft &&
                o.state === "quoted" &&
                o.compliant &&
                can("propose") && (
                  <button
                    className={button}
                    onClick={() => setForm({ action: "propose", offer: o })}
                  >
                    Propose supplier
                  </button>
                )}
            </div>
          )}
        />
      )}
      {c?.selected_offer_id && (
        <div className="rounded-lg border bg-slate-50 p-4 text-sm">
          <h5 className="font-bold">
            Selection:{" "}
            {
              c.offers.find((o) => o.id === c.selected_offer_id)
                ?.supplier_snapshot.name
            }
          </h5>
          <p className="mt-2 whitespace-pre-wrap">{c.selection_reason}</p>
          <p className="mt-2 whitespace-pre-wrap">LOI terms: {c.loi_terms}</p>
          <p className="mt-2">
            {c.approved_at
              ? `Approved ${new Date(c.approved_at).toLocaleString()}`
              : "Awaiting independent approval"}
          </p>
          {can("decide") && (
            <button
              className={button}
              onClick={() => setForm({ action: "decide" })}
            >
              Review selection
            </button>
          )}
          {["approved", "released"].includes(c.state) && (
            <button
              className={secondary}
              onClick={() => sourcingDocument(data, "LOI")}
            >
              Download LOI copy
            </button>
          )}
          {step === 5 && c.state === "approved" && can("release") && (
            <button
              className={button}
              onClick={() => setForm({ action: "release" })}
            >
              Record LOI release
            </button>
          )}
          {c.state === "released" && (
            <p className="mt-2 font-semibold text-emerald-800">
              {c.loi_reference} · Sent {c.loi_sent_date} via {c.loi_channel} ·{" "}
              {c.loi_dispatch_reference}
            </p>
          )}
        </div>
      )}
      <details>
        <summary className="min-h-11 cursor-pointer font-semibold">
          Sourcing activity ({data.history.length})
        </summary>
        <ol className="max-h-64 space-y-3 overflow-auto text-sm">
          {data.history.map((e) => (
            <li key={e.id}>
              <strong>{e.action}</strong> ·{" "}
              {new Date(e.created_at).toLocaleString()}
              <p>{e.reason}</p>
            </li>
          ))}
        </ol>
      </details>
      <p className="text-xs text-slate-500">
        Downloads are printable HTML copies; open and use Print → Save as PDF.
        Evidence references point to your controlled records. No email, WhatsApp
        message or purchase order is sent by this screen.
      </p>
      {form && (
        <RecordDrawer
          title={
            form.action === "view"
              ? "Supplier offer details"
              : form.action.replaceAll("_", " ").toUpperCase()
          }
          onClose={close}
        >
          {form.action === "view" && form.offer ? (
            <OfferDetails offer={form.offer} data={data} />
          ) : (
            <SourcingForm
              key={form.action + (form.offer?.id || "")}
              action={form.action}
              offer={form.offer}
              data={data}
              base={base}
              changed={() => {
                setChanged(true);
                dirty(true);
              }}
              saved={(w) => {
                setData(w);
                setForm(null);
                setChanged(false);
                dirty(false);
              }}
            />
          )}
        </RecordDrawer>
      )}
    </section>
  );
}
function OfferDetails({
  offer: o,
  data,
}: {
  offer: SourcingOffer;
  data: SourcingWorkspace;
}) {
  const r = data.comparison.find((r) => r.offer_id === o.id);
  return (
    <div className="space-y-3">
      <h3 className="text-xl font-bold">{o.supplier_snapshot.name}</h3>
      <dl className="grid gap-3 sm:grid-cols-2">
        {Object.entries({
          Status: o.state,
          "RFQ deadline": o.due_date,
          "RFQ terms": o.rfq_terms,
          "Dispatch evidence": o.dispatch_reference,
          "Quote reference": o.quote_reference,
          "Quote evidence": o.evidence_reference,
          "Unit price": o.unit_price,
          Discount: o.discount,
          "Tax total": o.tax,
          Freight: o.freight,
          "Other charges": o.other,
          "Landed total": r?.total,
          "Budget variance": r?.variance,
          "Delivery days": o.delivery_days,
          "Payment terms": o.payment_terms,
          Warranty: o.warranty,
          Compliance: o.compliance_notes,
          "No bid reason": o.no_bid_reason,
        }).map(([k, v]) => (
          <div key={k}>
            <dt className="text-sm font-semibold">{k}</dt>
            <dd className="whitespace-pre-wrap break-words text-sm text-slate-600">
              {v || "—"}
            </dd>
          </div>
        ))}
      </dl>
      <button
        className={secondary}
        onClick={() => sourcingDocument(data, "RFQ", o)}
      >
        Download supplier RFQ
      </button>
    </div>
  );
}
function SourcingForm({
  action,
  offer,
  data,
  base,
  changed,
  saved,
}: {
  action: string;
  offer?: SourcingOffer;
  data: SourcingWorkspace;
  base: string;
  changed: () => void;
  saved: (w: SourcingWorkspace) => void;
}) {
  const [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const c = data.case;
  const [editVersion] = useState(c?.version || 0);
  const field = (
    name: string,
    label: string,
    type = "text",
    value?: string | number,
  ) => (
    <ValidatedLabel className="block text-sm font-semibold">
      {label}
      <ValidatedInput
        name={name}
        type={type}
        required
        className={input}
        defaultValue={value ?? ""}
        maxLength={name === "quote_reference" ? 200 : 1000}
        min={type === "number" ? 0 : undefined}
        step={
          type === "number"
            ? name === "delivery_days"
              ? "1"
              : "0.01"
            : undefined
        }
      />
    </ValidatedLabel>
  );
  const area = (name: string, label: string, value?: string, max = 2000) => (
    <ValidatedLabel className="block text-sm font-semibold">
      {label}
      <ValidatedTextarea
        name={name}
        required
        className={input}
        rows={3}
        maxLength={max}
        defaultValue={value || ""}
      />
    </ValidatedLabel>
  );
  async function submit(e: React.FormEvent<HTMLFormElement>) {
    const submittedForm = e.currentTarget instanceof HTMLFormElement ? e.currentTarget : undefined;
    e.preventDefault();
    const f = new FormData(e.currentTarget);
    const values = Object.fromEntries(f.entries()) as Record<string, unknown>;
    values.action = action;
    values.version = editVersion;
    if (offer) values.offer_id = offer.id;
    if (action === "quote") {
      values.delivery_days = Number(values.delivery_days);
      values.compliant = values.compliant === "true";
    }
    if (action === "decide") values.approve = values.approve === "true";
    setBusy(true);
    setError("");
    try {
      saved(await request<SourcingWorkspace>(base + "/sourcing", values));
    } catch (err) {
      if (displayServerErrors(submittedForm, err)) return;
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <ValidatedForm onSubmit={submit} onChange={changed} className="space-y-4">
      <fieldset disabled={busy} className="space-y-4">
        {offer && (
          <p className="font-bold">
            {offer.supplier_snapshot.name} · {data.source.quantity}{" "}
            {data.source.uom}
          </p>
        )}
        {action === "configure" && (
          <ValidatedLabel className="block text-sm font-semibold">
            Independent approver
            <ValidatedSelect
              className={input}
              name="approver_id"
              required
              defaultValue={c?.approver_id || ""}
            >
              <option value="">Select active approver</option>
              {data.approvers.map((a) => (
                <option key={a.id} value={a.id}>
                  {a.label}
                </option>
              ))}
            </ValidatedSelect>
            <span className="font-normal">
              Add a project member with sourcing read and approve permissions.
              The proposer cannot approve their own selection.
            </span>
          </ValidatedLabel>
        )}
        {action === "rfq" && (
          <>
            <ValidatedLabel className="block text-sm font-semibold">
              Eligible supplier
              <ValidatedSelect
                className={input}
                required
                name="supplier_id"
                defaultValue={offer?.supplier_id || ""}
              >
                <option value="">Choose supplier</option>
                {data.suppliers
                  .filter(
                    (s) =>
                      s.eligible &&
                      (s.id === offer?.supplier_id ||
                        !c?.offers.some((o) => o.supplier_id === s.id)),
                  )
                  .map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name} ({s.code})
                    </option>
                  ))}
              </ValidatedSelect>
            </ValidatedLabel>
            <p className="text-sm">
              Suppliers must be active, approved and linked to this item in
              Supplier Master.
            </p>
            {field("due_date", "Quotation due date", "date", offer?.due_date)}
            {area(
              "rfq_terms",
              "Commercial requirements / RFQ terms",
              offer?.rfq_terms,
              4000,
            )}
          </>
        )}
        {(action === "dispatch" || action === "release") && (
          <>
            {field(
              "sent_date",
              "Actual date sent",
              "date",
              new Date().toISOString().slice(0, 10),
            )}
            <ValidatedLabel className="block text-sm font-semibold">
              Dispatch channel
              <ValidatedSelect className={input} name="channel" required>
                <option value="manual">Manual</option>
                <option value="email">Email</option>
                <option value="whatsapp">WhatsApp</option>
              </ValidatedSelect>
            </ValidatedLabel>
            {field(
              "dispatch_reference",
              "Dispatch evidence / message reference",
            )}
            <p className="text-sm">
              Confirm the document was actually sent. This records dispatch; it
              does not send a message.
            </p>
          </>
        )}
        {action === "quote" && (
          <>
            <div className="grid gap-3 sm:grid-cols-2">
              {field(
                "quote_reference",
                "Supplier quotation reference",
                "text",
                offer?.quote_reference,
              )}
              {field(
                "valid_until",
                "Valid until",
                "date",
                offer?.valid_until || "",
              )}
              {field(
                "evidence_reference",
                "Controlled quotation evidence reference",
                "text",
                offer?.evidence_reference,
              )}
              {(
                ["unit_price", "discount", "tax", "freight", "other"] as const
              ).map((k) => (
                <div key={k}>
                  {field(
                    k,
                    k === "tax"
                      ? "Tax amount for whole line"
                      : k.replaceAll("_", " "),
                    "number",
                    offer?.[k] || "0",
                  )}
                </div>
              ))}
              {field(
                "delivery_days",
                "Delivery lead time (whole days)",
                "number",
                offer?.delivery_days || 0,
              )}
            </div>
            <p className="text-sm">
              All amounts in {data.source.currency}. Discount, tax, freight and
              other charges are line totals. No tax-credit assumption.
            </p>
            {area("payment_terms", "Payment terms", offer?.payment_terms)}
            {area("warranty", "Warranty", offer?.warranty, 1000)}
            <ValidatedLabel className="block text-sm font-semibold">
              Technical compliance
              <ValidatedSelect
                className={input}
                name="compliant"
                aria-label="Technical compliance"
                defaultValue={offer?.compliant ? "true" : "false"}
              >
                <option value="false">Not compliant / review pending</option>
                <option value="true">
                  Complies with exact released item and specification
                </option>
              </ValidatedSelect>
            </ValidatedLabel>
            {area(
              "compliance_notes",
              "Technical review / deviations",
              offer?.compliance_notes,
            )}
          </>
        )}
        {action === "propose" && (
          <>
            <p className="text-sm">
              Review every considered offer before submission. Explain delivery,
              compliance and value, particularly a non-L1 or single-offer
              selection. Terms are frozen for the approver.
            </p>
            {area("loi_terms", "Reviewed LOI terms", c?.loi_terms, 4000)}
          </>
        )}
        {action === "decide" && (
          <>
            <p className="text-sm">
              Review the comparison and quoted terms before deciding. Your
              identity and decision will be recorded.
            </p>
            <ValidatedLabel className="block text-sm font-semibold">
              Decision
              <ValidatedSelect
                className={input}
                name="approve"
                aria-label="Decision"
                required
              >
                <option value="false">Return for revision</option>
                <option value="true">Approve selection and LOI terms</option>
              </ValidatedSelect>
            </ValidatedLabel>
            <p className="whitespace-pre-wrap">{c?.selection_reason}</p>
            <p className="whitespace-pre-wrap">{c?.loi_terms}</p>
          </>
        )}
        {action === "reopen" && (
          <p className="text-amber-900">
            Returning to draft invalidates approval. All changes will require a
            new decision before release.
          </p>
        )}
        {area(
          "reason",
          action === "propose"
            ? "Selection justification"
            : action === "no_bid"
              ? "No-bid / withdrawal reason"
              : "Reason / decision note",
        )}
        {error && (
          <p role="alert" className="rounded-lg bg-red-50 p-3 text-red-800">
            {error} Your input is retained. Close this form to discard it and
            refresh if the saved version changed.
          </p>
        )}
        <button className={button} disabled={busy} type="submit">
          {busy
            ? "Saving…"
            : action === "propose"
              ? "Submit for approval"
              : action === "release"
                ? "Confirm LOI release"
                : "Save"}
        </button>
      </fieldset>
    </ValidatedForm>
  );
}
