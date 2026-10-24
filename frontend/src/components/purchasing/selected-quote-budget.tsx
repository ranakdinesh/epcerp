"use client";
import { useEffect, useState } from "react";
import { quotationMoney } from "./quotation-money";
import { DataGrid } from "@/components/ui/data-grid";

type Preview = {
  quote_key: string;
  budget_revision: number | null;
  currency: string;
  message: string;
  lines: { line_id: string; item: { description?: string; sku?: string }; quantity: string; budget: string | null; quoted: string | null; difference: string | null }[];
};

export function SelectedQuoteBudget({ url, quoteKey }: { url: string; quoteKey: string }) {
  const [result, setResult] = useState<Preview | null>(null);
  const [error, setError] = useState("");
  useEffect(() => {
    const controller = new AbortController();
    fetch(`${url}/budget-preview?quote_key=${encodeURIComponent(quoteKey)}`, { cache: "no-store", signal: controller.signal })
      .then(async response => {
        const data = await response.json();
        if (!response.ok) throw Error(typeof data.detail === "string" ? data.detail : "Cannot load budget comparison");
        setResult(data);
      }).catch(error => { if (error.name !== "AbortError") setError(error.message); });
    return () => controller.abort();
  }, [url, quoteKey]);
  if (error) return <p role="alert" className="text-sm text-red-800">{error}</p>;
  if (!result || result.quote_key !== quoteKey) return <p role="status">Loading selected quotation budget comparison…</p>;
  const money = (value: string | null) => value === null ? "Not allocated / quoted" : quotationMoney(value,result.currency);
  return <section className="space-y-3">
    <h5 className="font-bold">Selected quotation vs approved item budgets</h5>
    <p className="text-sm text-slate-600">{result.message}{result.budget_revision !== null ? ` Budget revision ${result.budget_revision}.` : ""}</p>
    <DataGrid label="Selected quotation budget comparison" rows={result.lines.map(line => ({ ...line, id: line.line_id }))} searchable={false} paginate={false} columns={[
      { key: "item", label: "Item", value: line => line.item.description ?? line.item.sku ?? line.line_id },
      { key: "quantity", label: "Quantity", value: line => line.quantity },
      { key: "budget", label: "Approved item budget", value: line => money(line.budget) },
      { key: "quoted", label: "Selected quotation", value: line => money(line.quoted) },
      { key: "difference", label: "Difference (+ / −)", value: line => line.difference === null ? "Budget / price missing" : `${line.difference} ${result.currency}` },
    ]} />
  </section>;
}
