import type { DB } from '../../db';
import { query } from '../catalogue/database';
export async function purchaseCategory(db:DB,code:string){return(await query<{code:string;active:boolean;kind:string}>(db,'CategoriesFindEligibility',{codes:[code]})).rows.some(r=>r.active&&r.kind==='category'&&r.code===code);}
