import 'server-only';
import type { DB } from '../../db';
import { VendorRepository } from './repository';
export async function purchaseVendors(db:DB){return(await new VendorRepository(db).list()).filter(v=>v.is_active).map(v=>({id:v.id,name:v.name,code:v.code,is_active:v.is_active,data:v.data,version:v.version}));}
