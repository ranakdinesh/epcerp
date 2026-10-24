import type {Item,Line} from "@/components/purchasing/purchase-bo-workspace";
export type ItemPreparation={id:string;name?:string;preparer_id:string;version:number;state:string;lines:Line[]};
export type ItemListing={lists:ItemPreparation[];can_write:boolean;can_complete:boolean};
export type PurchaseItemRow={id:string;preparation:ItemPreparation;line:Line};
export function itemRows(records:readonly ItemPreparation[]):PurchaseItemRow[]{return records.flatMap(preparation=>preparation.lines.map(line=>({id:`${preparation.id}:${line.id}`,preparation,line})));}
export function canEditPreparation(record:ItemPreparation,userID:string,superuser:boolean,canWrite:boolean){return canWrite&&record.state==="draft"&&(record.preparer_id===userID||superuser);}
export function editablePreparation(records:readonly ItemPreparation[],userID:string,superuser:boolean){return records.find(r=>canEditPreparation(r,userID,superuser,true)&&r.lines.length<100);}
export function usedItems(records:readonly ItemPreparation[],current?:PurchaseItemRow){return new Set(itemRows(records).filter(r=>r.id!==current?.id).flatMap(r=>r.line.item_id?[r.line.item_id]:[]));}
export function itemInputErrors(line:Line,item:Partial<Item>,used:ReadonlySet<string>){
 const errors:Record<string,string>={};
 if(!line.item_id)errors.item_id="Select an eligible item.";
 else if(used.has(line.item_id))errors.item_id="This item is already added. Edit its existing row.";
 const q=line.quantity??"";
 if(!/^[0-9]{1,13}(\.[0-9]{1,3})?$/.test(q))errors.quantity="Enter a positive quantity with at most 3 decimal places.";
 else{const[whole,part=""]=q.split(".");const exact=BigInt(whole)*BigInt(1000)+BigInt(part.padEnd(3,"0"));if(exact===BigInt(0))errors.quantity="Enter a positive quantity.";else if(["nos","each","pcs","piece","pieces","number","numbers"].includes((item.uom??"").trim().toLowerCase())&&exact%BigInt(1000)!==BigInt(0))errors.quantity="This unit requires a whole-number quantity.";}
 const date=line.required_date??"";
 if(!/^\d{4}-\d{2}-\d{2}$/.test(date)||Number.isNaN(Date.parse(date+"T00:00:00Z"))||new Date(date+"T00:00:00Z").toISOString().slice(0,10)!==date)errors.required_date="Set a valid required-at-site date.";
 return errors;
}
export function itemSavePayload(record:ItemPreparation,line:Line,adding:boolean,reason:string){
 if(!adding&&!record.lines.some(r=>r.id===line.id))throw Error("This item is no longer in the saved preparation. Refresh items.");
 const lines=adding?[...record.lines,line]:record.lines.map(r=>r.id===line.id?line:r);
 return {version:record.version,...(record.version===0?{name:"Purchase requirements"}:{}),reason,
 lines:lines.map(({id,item_id,quantity,required_date,reference,notes})=>({id,item_id,quantity,required_date,reference,notes}))};
}

export function nextItemEntry(saved:ItemPreparation,actor:string,recordID:string,lineID:string){
 const record=saved.lines.length<100?saved:{id:recordID,preparer_id:actor,version:0,state:"draft",lines:[]};
 const initial:Line={id:lineID,item_id:null,quantity:null,required_date:null,reference:"",notes:"",item_snapshot:{},demand_id:null};
 return {record,initial,adding:true};
}
