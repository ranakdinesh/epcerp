/** Live estimate uses decimal integers; saved server calculations remain authoritative. */
export type PriceInput = { pricing_mode: "unit_price" | "line_total"; unit_price: string; line_total: string; discount_pct: string; tax_pct: string };
function scaled(value: string, places: number): bigint {
  if (!/^\d+(\.\d+)?$/.test(value)) throw Error("Enter a nonnegative amount");
  const [whole, fraction = ""] = value.split(".");
  if (fraction.length > places) throw Error("Too many decimal places");
  return BigInt(whole + fraction.padEnd(places, "0"));
}
const rounded = (numerator: bigint, denominator: bigint) => (numerator + denominator / BigInt(2)) / denominator;
export const decimalMoney = (cents: bigint) => `${cents / BigInt(100)}.${(cents % BigInt(100)).toString().padStart(2, "0")}`;
export function lineEstimate(entry: PriceInput, quantity: string) {
  const value = entry.pricing_mode === "line_total" ? entry.line_total : entry.unit_price;
  if (!value) return null;
  try {
    const qty = scaled(quantity, 4);
    if (qty <= BigInt(0)) return null;
    const base = entry.pricing_mode === "line_total" ? scaled(value, 2) : rounded(qty * scaled(value, 2), BigInt(10000));
    const discountPercent = scaled(entry.discount_pct, 2), taxPercent = scaled(entry.tax_pct, 2);
    if (discountPercent > BigInt(10000) || taxPercent > BigInt(10000)) return null;
    const discount = rounded(base * discountPercent, BigInt(10000));
    const net = base - discount, tax = rounded(net * taxPercent, BigInt(10000));
    return { base, discount, net, tax, total: net + tax };
  } catch { return null; }
}
export function chargeEstimate(value: string) { try { return scaled(value, 2); } catch { return null; } }

/** Preserve the user's last edited basis, without round-tripping a lump sum. */
export function editPrice(entry:PriceInput,key:"unit_price"|"line_total",value:string,quantity:string):PriceInput {
 const next={...entry,pricing_mode:key==="unit_price"?"unit_price" as const:"line_total" as const,[key]:value};
 const other=key==="unit_price"?"line_total":"unit_price";
 if(!value)return {...next,[other]:""};
 try {const qty=scaled(quantity,4),amount=scaled(value,2);if(qty<=BigInt(0))return {...next,[other]:""};return {...next,[other]:decimalMoney(key==="unit_price"?rounded(qty*amount,BigInt(10000)):rounded(amount*BigInt(10000),qty))};}catch{return {...next,[other]:""};}
}
export function fixedPrice(value:string){try{return decimalMoney(scaled(value,2))}catch{return value}}

export function quotationAmountFields(totalOnly:boolean,totalAmount:FormDataEntryValue|null,freight:FormDataEntryValue|null,otherCharges:FormDataEntryValue|null){
 return totalOnly?{amount_mode:"total_only" as const,total_amount:String(totalAmount),freight:"0",other_charges:"0"}:{amount_mode:"itemized" as const,total_amount:null,freight:String(freight),other_charges:String(otherCharges)};
}
