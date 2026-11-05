export type Capabilities = {
    read: boolean;
    write: boolean;
    scoped: boolean;
    work: boolean;
};
export type Order = {
    id: string;
    project_id: string;
    package_id: string;
    version: number;
    number: string;
    date: string;
    supplier_id: string;
    supplier_name: string;
    description: string;
    quantity: string;
    uom: string;
    currency: string;
    total: string;
    payment_terms: string;
    delivery_days: number;
    warranty: string;
    released_at: string;
    released_by_id: string;
};
export type Source = {
    project_id: string;
    package_id: string;
    order: Order | null;
    available: boolean;
    blockers: string[];
};
export type Response = {
    status: string;
    accepted_date: string;
    signatory_name: string;
    document_reference: string;
    remarks: string;
};
export type Record = {
    id: string;
    project_id: string;
    package_id: string;
    version: number;
    order_snapshot: Order;
    response: Response;
    is_active: boolean;
    created_at: string;
    updated_at: string;
    created_by_id: string;
    updated_by_id: string;
};
export type Command = {
    version: number;
    action: string;
    reason: string;
    response: Response;
};
