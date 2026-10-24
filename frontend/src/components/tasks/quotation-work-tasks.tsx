"use client";
import Link from "next/link";
export type QuotationTask={id:string;project_id:string;project_name:string;title:string;link:string;state:string};
export function QuotationWorkTasks({rows}:{rows:QuotationTask[]}){
 if(!rows.length)return null;
 return <section className="space-y-3 rounded-xl border border-amber-200 bg-amber-50 p-5" aria-label="Quotation tasks needing attention"><h2 className="font-bold">Quotation work needing attention</h2>{rows.map(r=><Link className="flex flex-wrap items-center justify-between gap-3 rounded-lg border bg-white p-4" href={r.link} key={r.id}><span><strong>{r.title}</strong><span className="block text-sm">{r.project_name}</span></span><span className="text-sm capitalize">{r.state.replaceAll("_"," ")} →</span></Link>)}</section>;
}
