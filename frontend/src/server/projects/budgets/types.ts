export type Audit = {
    created_at: string;
    updated_at: string;
    created_by_id: string | null;
    updated_by_id: string | null;
};
export type Head = Audit & {
    id: string;
    parent_id: string | null;
    name: string;
    kind: string;
    amount: string;
    owner_id: string;
    is_active: boolean;
};
export type Item = {
    bo_id: string;
    line_id: string;
    bo_version: number;
    bo_name: string;
    quantity: string;
    sku: string;
    description: string;
    uom: string;
    manufacturer: string;
    item_id: string;
    item_snapshot: ItemSnapshot;
};
export type ItemSnapshot = {
    sku: string;
    description: string;
    uom: string;
    manufacturer: string;
};
export type Allocation = Audit & {
    id: string;
    bo_id: string;
    line_id: string;
    head_id: string;
    amount: string;
    owner_id: string;
    is_active: boolean;
    unit_price: string | null;
    pricing_quantity: string | null;
    source: Item | null;
};
export type Data = {
    total: string;
    currency: string;
    owner_id: string;
    notes: string;
    pricing_mode?: string;
    heads: Head[];
    allocations: Allocation[];
};
export type Approved = {
    revision: number;
    data: Data;
    approved_at: string;
    approved_by_id: string;
    decision_id: string;
};
export type Budget = Audit & {
    project_id: string;
    version: number;
    revision: number;
    state: string;
    data: Data;
    approved: Approved | null;
    is_active: boolean;
};
export type Price = {
    bo_id: string;
    line_id: string;
    bo_version: number;
    quantity: string;
    unit_price: string | null;
    changed: boolean;
};
export type Command = {
    action: string;
    version: number;
    reason: string;
    data: Data | null;
    prices: Price[];
};
export type Owner = {
    id: string;
    name: string;
};
export type Project = {
    id: string;
    name: string;
    currency: string;
    contract_value: string | null;
    available: boolean;
};
export type HeadReport = Head & {
    unallocated: string;
};
export type Report = {
    total: string;
    allocated_heads: string;
    purchase_total: string;
    unallocated: string;
    item_allocations: string;
    heads: HeadReport[];
};
export type Event = {
    id: string;
    project_id: string;
    action: string;
    reason: string;
    before: Budget | null;
    after: Budget;
    created_by_id: string;
    created_at: string;
};
export type Workspace = {
    record: Budget | null;
    project: Project;
    owners: Owner[];
    items: Item[];
    can_write: boolean;
    can_approve: boolean;
    report: Report | null;
    approved_report: Report | null;
    history: Event[];
};
