import {decimalUnits} from "@/components/projects/purchase-budget-calculations";
type InputLine={line_id:string;budget?:string|null;quoted?:string|null};
type Offer={best_price:boolean;preferred:boolean;snapshot:{total:string;lines:InputLine[]}};
const cents=(value:string|null|undefined)=>value==null?null:decimalUnits(value,2);
function amount(value:bigint|null):string|null{if(value===null)return null;const n=value<BigInt(0)?-value:value;return `${value<BigInt(0)?"-":""}${n/BigInt(100)}.${(n%BigInt(100)).toString().padStart(2,"0")}`;}
function difference(left:bigint|null,right:bigint|null){return left===null||right===null?null:left-right;}
function sum(values:(bigint|null)[]){return values.length===0||values.some(v=>v===null)?null:values.reduce<bigint>((total,v)=>total+v!,BigInt(0));}

export function financeComparison<L extends InputLine>(lines:L[],offers:Offer[]){
 const lowest=offers.find(p=>p.best_price)?.snapshot??offers[0]?.snapshot,preferred=offers.find(p=>p.preferred)?.snapshot;
 const lowLines=new Map(lowest?.lines.map(l=>[l.line_id,l])),preferredLines=new Map(preferred?.lines.map(l=>[l.line_id,l]));
 const values=lines.map(line=>{const budget=cents(line.budget),low=cents(lowLines.get(line.line_id)?.quoted),pref=cents(preferredLines.get(line.line_id)?.quoted);return {line,budget,low,pref};});
 const rows=values.map(({line,budget,low,pref})=>({...line,id:line.line_id,budgetAmount:amount(budget),lowestAmount:amount(low),preferredAmount:amount(pref),lowestVariance:amount(difference(low,budget)),preferredVariance:amount(difference(pref,budget)),premium:amount(difference(pref,low))}));
 const budget=sum(values.map(v=>v.budget)),lowSum=sum(values.map(v=>v.low)),prefSum=sum(values.map(v=>v.pref)),lowTotal=cents(lowest?.total),prefTotal=cents(preferred?.total);
 const issues:string[]=[];
 if(lowTotal!==null&&lowSum!==null&&lowTotal!==lowSum)issues.push("Lowest-price total differs from its item amounts. Review the source quotation.");
 if(prefTotal!==null&&prefSum!==null&&prefTotal!==prefSum)issues.push("Preferred total differs from its item amounts. Review the source quotation.");
 return {rows,lowest,preferred,issues,totals:{budgetAmount:amount(budget),lowestAmount:amount(lowTotal),preferredAmount:amount(prefTotal),lowestVariance:amount(difference(lowSum===null?null:lowTotal,budget)),preferredVariance:amount(difference(prefSum===null?null:prefTotal,budget)),premium:amount(difference(prefSum===null?null:prefTotal,lowSum===null?null:lowTotal))}};
}
