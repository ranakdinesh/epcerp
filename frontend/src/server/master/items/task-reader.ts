import 'server-only';
import type { DB } from '../../db';
import { AppError } from '../../errors';
import { ItemRepository } from './repository';
import { eligibleLegacyMake } from './domain';
// Preserve make eligibility for historical purchasing records.
export async function purchaseTaskItem(db:DB,id:string){
    const repo=new ItemRepository(db),item=await repo.get(id);let approved=false;
    if(item.is_active&&item.status==='approved'){
        try {
            const configuration = await repo.configuration(item.configuration_id);
            approved = configuration.is_active &&
                (!item.manufacturer_id || eligibleLegacyMake(configuration, await repo.manufacturer(item.manufacturer_id)));
        }
        catch(e){if(!(e instanceof AppError)||![404,422].includes(e.status))throw e;}
    }
    return {...item,approved_for_purchase:approved};
}
export async function purchaseTaskCatalogue(db:DB){return new ItemRepository(db).list();}
