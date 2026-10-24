"use client";
import { displayServerErrors } from "@/components/ui/server-field-errors";
import { ValidatedForm, ValidatedInput, ValidatedLabel, ValidatedSelect, ValidatedTextarea } from "@/components/ui/validated-controls";
import { useState, type FormEvent } from "react";
import {
  type Definition,
  type SpecField,
  request,
  input,
  button,
  secondary,
  panel,
} from "./item-data";
export function ConfigurationEditor({
  records,
  source,
  onSaved,
  onClose,
}: {
  records: Definition[];
  source?: Definition;
  onSaved: () => void;
  onClose: () => void;
}) {
  const [kind, setKind] = useState(source?.data.kind ?? "category");
  const [code, setCode] = useState("");
  const [name, setName] = useState(source?.name ?? "");
  const [category, setCategory] = useState(source?.data.category_code ?? "");
  const [uom, setUom] = useState(source?.data.uom ?? "Nos");
  const [modelRequired, setModelRequired] = useState(
    source?.data.model_required ?? false,
  );
  const [fields, setFields] = useState<SpecField[]>(source?.data.fields ?? []);
  const [reason, setReason] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  async function save(e: FormEvent) {
    const submittedForm = e.currentTarget instanceof HTMLFormElement ? e.currentTarget : undefined;
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await request("/api/settings/material-catalogue", {
        code,
        name,
        data: {
          kind,
          category_code: kind === "type" ? category : null,
          family_code: source?.data.family_code ?? null,
          uom,
          model_required: modelRequired,
          fields: kind === "type" ? fields : [],
        },
        reason,
      });
      onSaved();
    } catch (e) {
      if (displayServerErrors(submittedForm, e)) return;
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  function field(index: number, patch: Partial<SpecField>) {
    setFields(fields.map((f, i) => (i === index ? { ...f, ...patch } : f)));
  }
  return (
    <ValidatedForm onSubmit={save} className={panel}>
      <h3 className="text-lg font-bold">
        {source
          ? "Publish configuration revision"
          : "New category or configuration"}
      </h3>
      <p className="text-sm text-slate-500">
        Codes are permanent. Existing items retain their original
        specifications. Option format: CODE = readable value, one per line.
      </p>
      {error && (
        <p role="alert" className="text-red-700">
          {error}
        </p>
      )}
      <fieldset disabled={busy} className="space-y-4">
        <div className="grid gap-4 sm:grid-cols-3">
          <ValidatedLabel>
            Record type
            <ValidatedSelect name="kind"
              className={input}
              value={kind}
              disabled={!!source}
              onChange={(e) => setKind(e.target.value as "category" | "type")}
            >
              <option value="category">Category</option>
              <option value="type">Product configuration</option>
            </ValidatedSelect>
          </ValidatedLabel>
          <ValidatedLabel>
            Code
            <ValidatedInput name="code"
              required
              maxLength={16}
              pattern="[A-Z0-9]+"
              value={code}
              onChange={(e) => setCode(e.target.value.toUpperCase())}
              className={input}
            />
          </ValidatedLabel>
          <ValidatedLabel>
            Name
            <ValidatedInput name="name"
              required
              maxLength={255}
              value={name}
              onChange={(e) => setName(e.target.value)}
              className={input}
            />
          </ValidatedLabel>
        </div>
        {kind === "type" && (
          <>
            <div className="grid gap-4 sm:grid-cols-3">
              <ValidatedLabel>
                Category
                <ValidatedSelect name="category"
                  required
                  disabled={!!source}
                  value={category}
                  onChange={(e) => setCategory(e.target.value)}
                  className={input}
                >
                  <option value="">Select category</option>
                  {records
                    .filter((r) => r.is_active && r.data.kind === "category")
                    .map((r) => (
                      <option key={r.id} value={r.code}>
                        {r.name}
                      </option>
                    ))}
                </ValidatedSelect>
              </ValidatedLabel>
              <ValidatedLabel>
                Base UOM
                <ValidatedSelect name="uom"
                  value={uom}
                  disabled={!!source}
                  onChange={(e) => setUom(e.target.value)}
                  className={input}
                >
                  {[
                    "Nos",
                    "Meter",
                    "Kg",
                    "Set",
                    "Pair",
                    "Lot",
                    "Sqm",
                    "Rmt",
                    "Bundle",
                    "Box",
                  ].map((u) => (
                    <option key={u}>{u}</option>
                  ))}
                </ValidatedSelect>
              </ValidatedLabel>
              <ValidatedLabel className="flex items-center gap-2">
                <ValidatedInput
                  type="checkbox"
                  checked={modelRequired}
                  onChange={(e) => setModelRequired(e.target.checked)}
                />
                Manufacturer model required
              </ValidatedLabel>
            </div>
            {fields.map((f, i) => (
              <div
                key={i}
                className="grid gap-3 rounded-xl bg-slate-50 p-4 sm:grid-cols-2"
              >
                <ValidatedLabel>
                  Field code
                  <ValidatedInput
                    required
                    value={f.code}
                    pattern="[A-Z0-9]{1,16}"
                    className={input}
                    onChange={(e) =>
                      field(i, { code: e.target.value.toUpperCase() })
                    }
                  />
                </ValidatedLabel>
                <ValidatedLabel>
                  Field label
                  <ValidatedInput
                    required
                    value={f.label}
                    className={input}
                    onChange={(e) => field(i, { label: e.target.value })}
                  />
                </ValidatedLabel>
                <ValidatedLabel required className="sm:col-span-2">
                  Allowed values
                  <OptionEditor
                    options={f.options}
                    onChange={(options) => field(i, { options })}
                  />
                </ValidatedLabel>
                <ValidatedLabel>
                  <ValidatedInput
                    type="checkbox"
                    checked={f.required}
                    onChange={(e) => field(i, { required: e.target.checked })}
                  />{" "}
                  Required field
                </ValidatedLabel>
                <button
                  type="button"
                  className={secondary}
                  onClick={() => setFields(fields.filter((_, j) => j !== i))}
                >
                  Remove field
                </button>
              </div>
            ))}
            <button
              type="button"
              className={secondary}
              onClick={() =>
                setFields([
                  ...fields,
                  { code: "", label: "", required: true, options: [] },
                ])
              }
            >
              Add specification field
            </button>
          </>
        )}
        <ValidatedLabel className="block">
          Review reason
          <ValidatedInput name="reason"
            required
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            className={input}
          />
        </ValidatedLabel>
        <div className="flex gap-2">
          <button className={button}>
            {busy
              ? "Saving…"
              : source
                ? "Publish new revision"
                : "Create definition"}
          </button>
          <button type="button" className={secondary} onClick={onClose}>
            Cancel
          </button>
        </div>
      </fieldset>
    </ValidatedForm>
  );
}
function OptionEditor({
  options,
  onChange,
}: {
  options: SpecField["options"];
  onChange: (o: SpecField["options"]) => void;
}) {
  const [text, setText] = useState(
    options.map((o) => `${o.code} = ${o.label}`).join("\n"),
  );
  return (
    <ValidatedTextarea name="text"
      required
      rows={5}
      className={input}
      value={text}
      onChange={(e) => {
        setText(e.target.value);
        onChange(
          e.target.value
            .split("\n")
            .filter((l) => l.trim())
            .map((l) => {
              const index = l.indexOf("=");
              return {
                code: (index < 0 ? l : l.slice(0, index)).trim().toUpperCase(),
                label: (index < 0 ? l : l.slice(index + 1)).trim(),
              };
            }),
        );
      }}
    />
  );
}
