"use client";
import {useEffect,useState} from "react";
export function useDashboardSource<T>(path:string|null,revision:number){
 const [result,setResult]=useState<{key:string;data:T|null;error:string;loadedAt:Date|null}>({key:"",data:null,error:"",loadedAt:null});
 const key=`${path}:${revision}`;
 useEffect(()=>{
  if(!path)return;
  const controller=new AbortController();
  fetch(path,{cache:"no-store",signal:controller.signal}).then(async response=>{
   const value=await response.json();
   if(!response.ok)throw Error(typeof value.detail==="string"?value.detail:"This information could not be loaded.");
   return value as T;
  }).then(data=>{if(!controller.signal.aborted)setResult({key,data,error:"",loadedAt:new Date()});}).catch(error=>{if(!controller.signal.aborted)setResult({key,data:null,error:error instanceof Error?error.message:"Connection unavailable.",loadedAt:null});});
  return()=>controller.abort();
 },[path,revision,key]);
 const current=path&&result.key===key;
 return {loadedAt:current?result.loadedAt:null,data:current?result.data:null,error:current?result.error:"",loading:!!path&&!current,enabled:!!path};
}
