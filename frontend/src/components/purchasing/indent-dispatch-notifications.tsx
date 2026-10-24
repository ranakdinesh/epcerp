"use client";
import { useEffect, useState } from "react";
import Link from "next/link";
import { useAuthenticatedUser } from "@/components/auth-user-provider";
import { RecordDrawer } from "@/components/ui/record-drawer";
type Notice={id:string;project_id:string;bo_id:string;number:string;group:string;vendor:string;created_at:string};
export function IndentDispatchNotifications(){
 const user=useAuthenticatedUser();
 const allowed=user.permissions.some(p=>["system:admin","projects.master.read","projects.purchase_indent_documents.read"].includes(p));
 const [notices,setNotices]=useState<Notice[]>([]);
 const [error,setError]=useState("");
 const [open,setOpen]=useState(false);
 useEffect(()=>{
  if(!allowed)return;
  const controller=new AbortController();let busy=false;
  async function load(){if(busy||document.visibilityState==="hidden")return;busy=true;try{const response=await fetch("/api/indent-dispatch-notifications",{cache:"no-store",signal:controller.signal});if(!response.ok)throw Error();const data=await response.json();if(!controller.signal.aborted){setNotices(data);setError("");}}catch{if(!controller.signal.aborted)setError("Unable to refresh indent notifications.");}finally{busy=false;}}
  void load();const timer=window.setInterval(()=>void load(),60000);
  const refresh=()=>void load();window.addEventListener("focus",refresh);window.addEventListener("indent-dispatch-changed",refresh);document.addEventListener("visibilitychange",refresh);
  return()=>{controller.abort();clearInterval(timer);window.removeEventListener("focus",refresh);window.removeEventListener("indent-dispatch-changed",refresh);document.removeEventListener("visibilitychange",refresh);};
 },[allowed,open]);
 if(!allowed)return null;
 return <><button type="button" aria-label={`Indent dispatch notifications: ${notices.length} pending`} onClick={()=>setOpen(true)} className="min-h-11 rounded-lg border border-slate-300 bg-white px-3 text-sm font-semibold text-slate-800">Indents <span className="rounded-full bg-amber-100 px-2 py-1 text-amber-900">{error?"!":notices.length}</span></button>{open&&<RecordDrawer centered title="Indent dispatch notifications" onClose={()=>setOpen(false)}>
 {error&&<p role="alert" className="text-sm text-red-700">{error}</p>}
 <p className="text-sm text-slate-600">Print was requested, but the actual sent date is missing. Recording dispatch resolves the notice for all recipients.</p>
 {!notices.length&&!error&&<p>No pending indent dispatch notifications.</p>}
 <ul className="space-y-3">{notices.map(notice=><li key={notice.id}><Link className="block rounded-lg border border-slate-200 bg-white p-3" onClick={()=>setOpen(false)} href={`/dashboard?screen=purchase-fms&project=${encodeURIComponent(notice.project_id)}&step=PUR-02&bo=${encodeURIComponent(notice.bo_id)}`}><p className="font-semibold">{notice.vendor}</p><p className="mt-1 text-sm">{notice.group} · Sent date missing</p><p className="mt-1 break-all text-xs text-slate-500">{notice.number}</p><p className="mt-1 text-xs">Notified {new Date(notice.created_at).toLocaleString()}</p></Link></li>)}</ul>
 </RecordDrawer>}</>;
}
