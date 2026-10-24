import { FieldValidationError } from "@/components/ui/server-field-errors";
export type SpecField = {
  code: string;
  label: string;
  required: boolean;
  options: { code: string; label: string }[];
};
export type Definition = {
  id: string;
  code: string;
  name: string;
  is_active: boolean;
  version: number;
  data: {
    kind: "category" | "type";
    category_code?: string;
    family_code?: string;
    uom?: string;
    revision?: number;
    model_required?: boolean;
    fields?: SpecField[];
  };
};
export type Manufacturer = {
  id: string;
  code: string;
  name: string;
  is_active: boolean;
  version: number;
  data: { eligible: string[]; notes: string };
};
export type Item = {
  id: string;
  description: string;
  configuration_id: string;
  manufacturer_id: string;
  model: string;
  specifications: Record<string, string>;
  engineering_key: string;
  sku: string | null;
  serial: number | null;
  status: string;
  is_active: boolean;
  version: number;
  snapshot: {
    configuration: Definition["data"] & { name: string; code: string };
    manufacturer: { id: string; code: string; name: string };
  };
};
export const input =
  "mt-1 min-h-11 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 focus:outline-2 focus:outline-blue-500";
export const button =
  "min-h-11 rounded-lg bg-blue-700 px-4 py-2 text-sm font-semibold text-white disabled:opacity-50";
export const secondary =
  "min-h-11 rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-slate-700 disabled:opacity-50";
export const panel =
  "rounded-2xl border border-slate-200 bg-white p-5 space-y-4";
export async function request<T>(
  url: string,
  body?: unknown,
  method = "POST",
): Promise<T> {
  const response = await fetch(
    url,
    body === undefined
      ? { cache: "no-store" }
      : {
          method,
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body),
        },
  );
  const data = await response.json();
  if (!response.ok) throw new FieldValidationError(data.detail, "Request failed. Check the fields and try again.");
  return data as T;
}
export function eligible(m: Manufacturer, c: Definition) {
  return (
    m.is_active &&
    [c.code, c.data.family_code, c.data.category_code].some(
      (code) => code && m.data.eligible.includes(code),
    )
  );
}

export function configuredItemName(
  configuration: { name: string; fields?: SpecField[] },
  specifications: Record<string, string>,
  manufacturer: string | undefined,
  model: string,
) {
  return [
    configuration.name,
    ...(configuration.fields ?? []).map((field) => {
      const value = specifications[field.code];
      return value
        ? (field.options.find((option) => option.code === value)?.label ??
            value)
        : "";
    }),
    manufacturer,
    model.trim(),
  ]
    .filter(Boolean)
    .join(" - ");
}

export function itemName(item: Item) {
  return configuredItemName(
    item.snapshot.configuration,
    item.specifications,
    item.snapshot.manufacturer.name,
    item.model,
  );
}
