"use client";

import { useEffect, useState } from "react";
import { ValidatedForm, ValidatedLabel, ValidatedTextarea } from "@/components/ui/validated-controls";
import { purchaseIndentTermsLimit, type PurchaseIndentSettings } from "@/lib/purchase-indent-settings";

const endpoint = "/api/purchase-fms/indent-settings";
const action = "app-action inline-flex min-h-11 items-center rounded-lg px-4 font-semibold disabled:opacity-50";

async function settingsRequest(init: RequestInit): Promise<PurchaseIndentSettings> {
  const response = await fetch(endpoint, { cache: "no-store", ...init });
  const data = await response.json();
  if (!response.ok) {
    const detail = typeof data.detail === "string" ? data.detail : data.detail?.[0]?.msg;
    throw new Error(detail ?? "Purchase Indent terms could not be loaded or saved.");
  }
  return data;
}

export function PurchaseIndentSettingsForm() {
  const [saved, setSaved] = useState<PurchaseIndentSettings | null>(null);
  const [terms, setTerms] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [reload, setReload] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    void settingsRequest({ signal: controller.signal }).then(value => {
      if (!controller.signal.aborted) {
        setSaved(value);
        setTerms(value.terms);
      }
    }).catch((failure: Error) => {
      if (!controller.signal.aborted) setError(failure.message);
    });
    return () => controller.abort();
  }, [reload]);

  async function save(current: PurchaseIndentSettings) {
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const value = await settingsRequest({
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ terms, version: current.version }),
      });
      setSaved(value);
      setTerms(value.terms);
      setNotice("Default terms saved.");
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "Default terms could not be saved.");
    } finally {
      setBusy(false);
    }
  }

  function reloadSettings() {
    setSaved(null);
    setError("");
    setNotice("");
    setReload(value => value + 1);
  }

  return <section className="max-w-2xl space-y-3 rounded-xl border bg-white p-5" aria-labelledby="purchase-indent-settings">
    <h3 id="purchase-indent-settings" className="text-lg font-semibold">Purchase Indent</h3>
    <p id="purchase-indent-terms-help" className="text-sm text-[var(--go-muted)]">
      New indents start with these company-wide terms. Users can edit the text on each indent.
    </p>
    {error && <p role="alert" className="rounded-lg bg-red-50 p-3 text-red-800">{error}</p>}
    {notice && <p role="status" className="rounded-lg bg-green-50 p-3 text-[#42644f]">{notice}</p>}
    {saved ? <ValidatedForm className="space-y-3" onSubmit={event => {
      event.preventDefault();
      if (!busy && saved.can_write && terms.trim() !== saved.terms) void save(saved);
    }}>
      <ValidatedLabel className="block font-semibold">Default terms and conditions
        <ValidatedTextarea name="terms" rows={8} maxLength={purchaseIndentTermsLimit}
          aria-describedby="purchase-indent-terms-help" value={terms} readOnly={!saved.can_write} disabled={busy}
          className="mt-2 block w-full rounded-lg border bg-[var(--go-panel)] p-3 font-normal"
          onChange={event => { setTerms(event.target.value); setNotice(""); }} />
      </ValidatedLabel>
      {saved.can_write && <div className="flex flex-wrap gap-2">
        <button type="submit" className={action} disabled={busy || terms.trim() === saved.terms}>{busy ? "Saving…" : "Save default terms"}</button>
        {error && <button type="button" className={action} disabled={busy} onClick={reloadSettings}>Reload saved terms</button>}
      </div>}
    </ValidatedForm> : error
      ? <button type="button" className={action} onClick={reloadSettings}>Retry</button>
      : <p role="status">Loading default terms…</p>}
  </section>;
}
