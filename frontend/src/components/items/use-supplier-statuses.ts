"use client";
import {useEffect,useState} from "react";
import {supplierRequest,type Verification} from "./supplier-types";

export function useSupplierStatuses(ids:string[]){
 const key=ids.join(","),[statuses,setStatuses]=useState<Record<string,Verification|null>>({}),[revision,setRevision]=useState(0);
 useEffect(()=>{
  const controller=new AbortController(),queue=key?key.split(","):[],next:Record<string,Verification|null>={};
  // Bound concurrent evidence/MinIO reads on supplier directories.
  async function worker(){while(queue.length&&!controller.signal.aborted){const id=queue.shift()!;try{next[id]=await supplierRequest<Verification>(`/api/settings/suppliers/${id}/verification`,{signal:controller.signal});}catch{next[id]=null;}}}
  Promise.all(Array.from({length:Math.min(4,queue.length)},worker)).then(()=>{if(!controller.signal.aborted)setStatuses(next);});
  return()=>controller.abort();
 },[key,revision]);
 useEffect(()=>{function refresh(){if(document.visibilityState==="visible")setRevision(v=>v+1);}const timer=window.setInterval(refresh,60000);window.addEventListener("focus",refresh);document.addEventListener("visibilitychange",refresh);return()=>{clearInterval(timer);window.removeEventListener("focus",refresh);document.removeEventListener("visibilitychange",refresh);};},[]);
 return {statuses,refresh:()=>setRevision(v=>v+1)};
}
