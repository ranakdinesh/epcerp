export type Snapshot = { id:string;sku:string;description:string;engineering_key:string;uom:string;manufacturer:string;demand_version?:number };
export type Line = { id:string;item_id:string|null;quantity:string|null;required_date:string|null;reference:string;notes:string;item_snapshot:Snapshot;demand_id:string|null };
export type Record = { id:string;project_id:string;preparer_id:string;name:string;version:number;state:string;is_active:boolean;lines:Line[];completed_at:string|null;completed_by_id:string|null;created_at:string;updated_at:string;created_by_id:string|null;updated_by_id:string|null };
export type InputLine = Omit<Line,'item_snapshot'|'demand_id'>;
export type Save = { name:string|null;version:number;reason:string;lines:InputLine[] };
