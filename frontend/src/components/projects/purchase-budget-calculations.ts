/** Exact decimal calculations for live unit-rate entry; server remains authoritative. */
export function decimalUnits(value: string, places: number): bigint | null {
  if (!/^\d+(\.\d+)?$/.test(value)) return null;
  const [whole, fraction = ""] = value.split(".");
  if (fraction.length > places) return null;
  return BigInt(whole + fraction.padEnd(places, "0"));
}
export function unitBudget(quantity: string, rate: string): bigint | null {
  const qty = decimalUnits(quantity, 4), cents = decimalUnits(rate, 2);
  return qty === null || qty <= BigInt(0) || cents === null ? null : (qty * cents + BigInt(5000)) / BigInt(10000);
}
export function moneyText(cents: bigint, currency: string): string {
  const negative = cents < BigInt(0), absolute = negative ? -cents : cents;
  return `${negative ? "−" : ""}${currency} ${(absolute / BigInt(100)).toLocaleString("en-IN")}.${(absolute % BigInt(100)).toString().padStart(2, "0")}`;
}
export function itemKey(item: { bo_id: string; line_id: string }): string {
  return `${item.bo_id}:${item.line_id}`;
}
