export type BudgetHead = { id: string; parent_id: string | null; name: string; kind: string; amount: string; owner_id: string; is_active: boolean };
export type BudgetAllocation = { id: string; bo_id: string; line_id: string; head_id: string; amount: string; owner_id: string; is_active: boolean; unit_price: string | null; pricing_quantity: string | null };
export type BudgetData = { total: string; currency: string; owner_id: string; notes: string; pricing_mode?: string; heads: BudgetHead[]; allocations: BudgetAllocation[] };
export type BudgetOwner = { id: string; name: string };
/** Explicit editable DTO: never send retained source snapshots or server attribution. */
export function budgetInput(data: BudgetData): BudgetData {
  return {total:data.total,currency:data.currency,owner_id:data.owner_id,notes:data.notes,pricing_mode:data.pricing_mode ?? "",heads:data.heads.map(h=>({id:h.id,parent_id:h.parent_id,name:h.name,kind:h.kind,amount:h.amount,owner_id:h.owner_id,is_active:h.is_active})),allocations:data.allocations.map(a=>({id:a.id,bo_id:a.bo_id,line_id:a.line_id,head_id:a.head_id,amount:a.amount,owner_id:a.owner_id,is_active:a.is_active,unit_price:a.unit_price ?? null,pricing_quantity:a.pricing_quantity ?? null}))};
}
