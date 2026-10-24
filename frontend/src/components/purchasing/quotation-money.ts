import {decimalUnits,moneyText} from "@/components/projects/purchase-budget-calculations";
export function quotationMoney(value:string|null|undefined,currency:string):string{
 if(value==null)return "Not set";
 const negative=value.startsWith("-");const unsigned=negative?value.slice(1):value;
 const cents=decimalUnits(unsigned,2);
 if(cents!==null)return moneyText(negative?-cents:cents,currency);
 const units=decimalUnits(unsigned,4);if(units===null)return "Invalid amount";
 return `${negative?"−":""}${currency} ${(units/BigInt(10000)).toLocaleString("en-IN")}.${(units%BigInt(10000)).toString().padStart(4,"0")}`;
}
export function positiveDifference(value:string|null|undefined):boolean{
 if(value==null||value.startsWith("-"))return false;
 const cents=decimalUnits(value,2);return cents!==null&&cents>BigInt(0);
}

export function quotationPremium(value:string,lowest:string|undefined,currency:string):string{
 if(lowest===undefined)return "";
 const amount=decimalUnits(value,2),base=decimalUnits(lowest,2);
 if(amount===null||base===null||amount<=base)return "";
 const difference=amount-base;
 if(base===BigInt(0))return `${moneyText(difference,currency)} above lowest`;
 const points=(difference*BigInt(10000)+base/BigInt(2))/base;
 return `${moneyText(difference,currency)} (${points/BigInt(100)}.${(points%BigInt(100)).toString().padStart(2,"0")}%) above lowest`;
}
