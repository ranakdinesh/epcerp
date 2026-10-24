export type PurchaseProject = { pending_approval_groups?:number;returned_groups?:number;approved_groups?:number; id: string; name: string; project_number: string; project_manager_id: string; purchase_manager_id: string | null; ownership_version: number; lifecycle_status: string; is_active: boolean; can_assign: boolean; progress_percent: number | null; progress_status: string; recorded_packages: number | null; completed_steps?: number[]; next_step?: string };
export type Dashboard = { items: PurchaseProject[]; total: number; page: number; page_size: number; portfolio: boolean };
export type History = { id: string; previous_manager_id: string | null; purchase_manager_id: string; created_by_id: string | null; reason: string; version: number; created_at: string };
export type Context = { project: PurchaseProject; history: History[] };
export type Candidate = { id: string; name: string };
export function purchaseProjectHref(id: string, step="PUR-01"): string { return `/dashboard?screen=purchase-fms&project=${encodeURIComponent(id)}&step=${encodeURIComponent(step)}`; }
export function purchaseProgressLabel(row: PurchaseProject): string { return row.progress_percent === null ? "Progress report not available yet" : `${row.progress_percent}%`; }
export async function read<T>(path: string, signal?: AbortSignal): Promise<T> {
 const response = await fetch(path, {cache:"no-store", signal});
 const body = await response.json();
 if (!response.ok) throw Error(typeof body.detail === "string" ? body.detail : "Purchase records could not be loaded.");
 return body as T;
}
