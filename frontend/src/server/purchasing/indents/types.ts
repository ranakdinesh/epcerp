import type { Snapshot } from '../core/catalogue';
import type { Allocation } from '../materials/repository';
export type Line = { demand_id:string;quantity:string;budget:string;full_kit:string };
export type Input = { currency:string;doer_id:string;resolver_id:string;lines:Line[];documents:string };
export type Hold = { id:string;reason:string;doer_id:string;resolver_id:string;started_at:string };
export type Source = { active:boolean;quantity:string;demand_version:number;item:Snapshot;required_date:string;allocations:Allocation[];client_id:string;policy:{status:string;reasons:string[];rules:{id:string;version:number}[]};holds:{holds:Hold[];hold_minutes:number;finish_impact:string} };
export type Release = Line & { at:string;actor_id:string;evidence:string;source:Source };
export type Review = { actor_id:string;at:string;evidence:string;demand_versions:Record<string,number> };
export type Data = Input & { number:string;state:string;released:Release[];review:Review|null };
