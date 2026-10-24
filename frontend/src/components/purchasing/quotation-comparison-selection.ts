import type { Comparison } from "./group-quotation-workspace";

export function quoteKey(quote: Comparison) { return `${quote.document_id}:${quote.revision ?? 1}`; }
function cents(value: string) {
  const [whole, fraction = ""] = value.split(".");
  return BigInt(whole) * BigInt(100) + BigInt(fraction.padEnd(2, "0"));
}
export function bestVendorQuote(quotes: Comparison[]): Comparison | undefined {
  return quotes.filter(quote=>quote.eligible).sort((a,b)=>{
    const difference=cents(a.total)-cents(b.total);
    return difference<BigInt(0)?-1:difference>BigInt(0)?1:(b.revision??1)-(a.revision??1);
  })[0];
}
export function draftComparison(quotes: Comparison[], choices: Record<string,string>): Comparison[] {
  const documents=[...new Set(quotes.map(quote=>quote.document_id))];
  const selected=documents.flatMap(document=>{
    const options=quotes.filter(quote=>quote.document_id===document);
    const explicit=options.find(quote=>quoteKey(quote)===choices[document]&&quote.eligible);
    const chosen=explicit??bestVendorQuote(options);
    return chosen?[{...chosen,lines:chosen.lines.map(line=>({...line}))}]:[];
  });
  for(const quote of selected){
    const group=selected.filter(other=>other.group_id===quote.group_id);
    const totals=[...new Set(group.map(other=>cents(other.total)))].sort((a,b)=>a<b?-1:a>b?1:0);
    quote.rank=totals.findIndex(total=>total===cents(quote.total))+1;
    quote.suggested=quote.rank===1;
    for(const line of quote.lines){
      const candidates=group.flatMap(other=>other.lines.filter(item=>item.line_id===line.line_id&&item.landed!==null).map(item=>cents(item.landed!)));
      line.item_lowest=Boolean(line.landed!==null&&candidates.length&&cents(line.landed!)===candidates.reduce((a,b)=>a<b?a:b));
    }
  }
  return selected;
}
