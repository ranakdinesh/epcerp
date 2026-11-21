import type { DB } from '../../db';
import { query } from '../catalogue/database';
export async function purchaseCategory(db:DB,code:string){return(await query<{code:string;active:boolean;kind:string}>(db,'CategoriesFindEligibility',{codes:[code]})).rows.some(r=>r.active&&r.kind==='category'&&r.code===code);}
export async function purchaseRuleScope(db:DB,code:string){if((await query<{code:string;active:boolean}>(db,'CategoriesFindEligibility',{codes:[code]})).rows.some(r=>r.active&&r.code===code))return true;return(await query<{family_code:string;active:boolean}>(db,'CategoriesItemContexts')).rows.some(r=>r.active&&r.family_code===code);}
