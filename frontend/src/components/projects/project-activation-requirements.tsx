"use client";

import { useEffect, useState, type FormEvent } from "react";
import { RecordDrawer } from "@/components/ui/record-drawer";
import { ValidatedForm, ValidatedInput, ValidatedLabel, ValidatedSelect } from "@/components/ui/validated-controls";

type EvidenceState = { has_contract_evidence: boolean; evidence_count: number; can_upload: boolean };
const button = "min-h-11 rounded-lg border border-blue-200 px-3 py-2 text-sm font-semibold text-blue-800 focus-visible:outline-2 focus-visible:outline-blue-600 disabled:opacity-50";

export function ProjectActivationRequirements({ projectId, loiNumber, poNumber, activating, error, revision, onEvidenceState, onAddEvidence, onKeepStatus, onFindReference }: {
  projectId: string; loiNumber: string; poNumber: string; activating: boolean; error: string; revision: number;
  onEvidenceState: (value: boolean | null) => void; onAddEvidence: () => void; onKeepStatus: () => void; onFindReference: () => void;
}) {
  const [state, setState] = useState<EvidenceState | null>(null);
  const [loadError, setLoadError] = useState("");
  const [retry, setRetry] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    void fetch(`/api/projects/${projectId}/activation-requirements`, { cache: "no-store", signal: controller.signal })
      .then(async response => {
        if (!response.ok) throw Error(response.status === 403 ? "Contract access is required to check evidence. An authorised project team member can add it." : "Contract evidence could not be checked. Retry before activating.");
        return response.json() as Promise<EvidenceState>;
      })
      .then(value => { if (!controller.signal.aborted) { setState(value); setLoadError(""); onEvidenceState(value.has_contract_evidence); } })
      .catch(reason => { if (!controller.signal.aborted) setLoadError(reason.message); });
    return () => controller.abort();
  }, [projectId, revision, retry, onEvidenceState]);
  const reference = loiNumber.trim() || poNumber.trim();
  return <section id="project-activation-requirements" aria-labelledby="activation-heading" tabIndex={-1} className="rounded-xl border border-slate-200 bg-white p-4 sm:col-span-2 focus:outline-2 focus:outline-blue-600">
    <h3 id="activation-heading" className="font-bold">Activation requirements</h3>
    <p className="mt-1 text-sm text-slate-600">Required when changing Project status to Active. You can save other details while keeping the current status.</p>
    <ul className="mt-3 space-y-3 text-sm" aria-live="polite">
      <li><strong>LOI or PO reference:</strong> {reference ? `Present — ${loiNumber.trim() ? "LOI" : "PO"} ${reference}` : "Missing — enter at least one reference"} {!reference && <button type="button" className={`${button} ml-2`} onClick={onFindReference}>Enter LOI / PO reference</button>}</li>
      <li><strong>Contract evidence:</strong> {state ? state.has_contract_evidence ? `Uploaded — ${state.evidence_count} qualifying document(s)` : "Missing — upload a Contract, Letter of Intent or Purchase Order" : loadError || "Checking uploaded evidence…"}
        {state?.can_upload && <button type="button" className={`${button} ml-2`} onClick={onAddEvidence}>Add evidence</button>}
        {loadError && <button type="button" className={`${button} ml-2`} onClick={() => {setState(null);setLoadError("");onEvidenceState(null);setRetry(value => value + 1);}}>Retry evidence check</button>}
      </li>
    </ul>
    {error && <p role="alert" className="mt-3 text-sm font-semibold text-red-700">{error}</p>}
    {activating && <button type="button" className={`${button} mt-3`} onClick={onKeepStatus}>Keep current status and edit details</button>}
  </section>;
}

export function ProjectEvidenceUpload({ projectId, onClose, onUploaded }: { projectId: string; onClose: () => void; onUploaded: () => void }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  async function upload(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const inputs = new FormData(form);
    const file = inputs.get("evidence_file");
    if (!(file instanceof File) || !file.size) return;
    setBusy(true); setError("");
    const data = new FormData(); data.set("file", file);
    const params = new URLSearchParams({ category: String(inputs.get("category")), title: String(inputs.get("title") || "").trim() });
    try {
      const response = await fetch(`/api/projects/${projectId}/documents?${params}`, { method: "POST", body: data });
      const result = await response.json().catch(() => ({}));
      if (!response.ok) throw Error(typeof result.detail === "string" ? result.detail : "Unable to upload evidence. Check the file and your contract permissions.");
      onUploaded();
    } catch (reason) { setError(reason instanceof Error ? reason.message : "Evidence upload failed."); }
    finally { setBusy(false); }
  }
  return <RecordDrawer centered title="Add contract evidence" onClose={() => { if (!busy) onClose(); }}>
    <p className="text-sm text-slate-600">Uploads are saved immediately to this project’s Contract control. Your unsaved project edits are retained; uploading does not activate the project.</p>
    <ValidatedForm onSubmit={event => void upload(event)} className="space-y-4">
      <fieldset disabled={busy} className="space-y-4">
        <ValidatedLabel className="block font-semibold">Document category<ValidatedSelect name="category" required className="field"><option value="contract">Contract</option><option value="loi">Letter of Intent</option><option value="purchase_order">Purchase Order</option></ValidatedSelect></ValidatedLabel>
        <ValidatedLabel className="block font-semibold">Document title (optional)<ValidatedInput name="title" maxLength={255} className="field" /></ValidatedLabel>
        <ValidatedLabel className="block font-semibold">Evidence file<ValidatedInput name="evidence_file" type="file" required accept=".pdf,.png,.jpg,.jpeg,.docx" className="field" /></ValidatedLabel>
        <p className="text-xs text-slate-600">PDF, PNG, JPEG or DOCX. The existing upload request-size limit applies.</p>
        {error && <p role="alert" className="text-sm text-red-700">{error}</p>}
        <button type="submit" disabled={busy} className={button}>{busy ? "Uploading…" : "Upload evidence"}</button>
      </fieldset>
    </ValidatedForm>
  </RecordDrawer>;
}
