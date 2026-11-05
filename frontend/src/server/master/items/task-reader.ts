import 'server-only';
import type { DB } from '../../db';
import { AppError } from '../../errors';
import { ItemRepository } from './repository';
import { eligible } from './domain';
// Published purchase-source read contract; reuse catalogue eligibility and decoding.
export async function purchaseTaskItem(db:DB,id:string){
    const repo=new ItemRepository(db),item=await repo.get(id);let approved=false;
    if(item.is_active&&item.status==='approved'){
        try{approved=!!eligible(await repo.configuration(item.configuration_id),await repo.manufacturer(item.manufacturer_id));}
        catch(e){if(!(e instanceof AppError)||![404,422].includes(e.status))throw e;}
    }
    return {...item,approved_for_purchase:approved};
}
export async function purchaseTaskCatalogue(db:DB){return new ItemRepository(db).list();}
