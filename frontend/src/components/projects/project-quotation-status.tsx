"use client";
import {useEffect,useState} from "react";
import Link from "next/link";
type Group={bo_id:string;group_id:string;name:string;state:string;message:string;return_action:string};
export function ProjectQuotationStatus({project,finance}:{project:string;finance:boolean}){
 const [rows,setRows]=useState<Group[]|null>(null),[error,setError]=useState("");
 useEffect(()=>{const c=new AbortController();fetch(`/api/projects/${project}/quotation-status`,{cache:"no-store",signal:c.signal}).then(async r=>{if(r.status===403)return [];const d=await r.json();if(!r.ok)throw Error(d.detail||"Unable to load quotation status");return d;}).then(setRows).catch(e=>{if(e.name!=="AbortError")setError(e.message)});return()=>c.abort()},[project]);
 if(!rows?.length&&!error)return null;
 return <section className="space-y-3 rounded-xl border bg-white p-5"><h2 className="font-bold">Purchase quotation approvals</h2>{error&&<p role="alert">{error}</p>}{rows?.map(r=><div key={`${r.bo_id}:${r.group_id}`} className="rounded-lg border p-3"><p className="font-semibold">{r.name} · {r.state.replaceAll("_"," ")}</p><p className="text-sm">{r.message}</p>{finance&&<Link className="text-sm text-blue-700 underline" href={`/dashboard?screen=quotation-approvals&project=${project}&bo=${r.bo_id}`}>Open Finance request</Link>}</div>)}</section>;
}
