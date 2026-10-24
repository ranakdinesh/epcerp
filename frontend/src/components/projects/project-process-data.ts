import { FieldValidationError } from "@/components/ui/server-field-errors";
export type ProjectProcess = {
  id: string;
  code: string;
  name: string;
  position: number;
  purpose: string;
  deliverables: string[];
  workspace: string | null;
  owner_user_id: string | null;
  planned_start_date: string | null;
  planned_finish_date: string | null;
  status: "not_started" | "in_progress" | "blocked" | "completed" | "bypassed";
  notes: string;
  completion_reference: string;
  dependencies_confirmed: boolean;
  started_at: string | null;
  completed_at: string | null;
  version: number;
};
export type ProcessBoard = {
  processes: ProjectProcess[];
  dependencies: { predecessor_id: string; successor_id: string }[];
  owners: { id: string; label: string }[];
  can_write: boolean;
};
export const processButton =
  "min-h-11 rounded-xl bg-blue-700 px-4 py-2 text-sm font-bold text-white hover:bg-blue-800 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500 disabled:opacity-50";
export const processSecondary =
  "min-h-11 rounded-xl border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 focus-visible:outline-2 focus-visible:outline-blue-500 disabled:opacity-50";
export function processStatus(value: string) {
  return value.replaceAll("_", " ");
}
export async function processRequest<T>(
  url: string,
  init?: RequestInit,
): Promise<T> {
  const response = await fetch(url, { cache: "no-store", ...init });
  const payload = await response.json().catch(() => ({}));
  if (!response.ok)
    throw new FieldValidationError(payload.detail, "Unable to complete the request. Please retry.");

  return payload;
}
