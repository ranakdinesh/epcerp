"use client";
import {useEffect,useState} from "react";
import dynamic from "next/dynamic";
import {BellIcon} from "@heroicons/react/24/outline";
import {useAuthenticatedUser} from "@/components/auth-user-provider";
import {RecordDrawer} from "@/components/ui/record-drawer";
import {useDashboardSource} from "./use-dashboard-source";
const Inbox=dynamic(()=>import("@/components/communication/personal-inbox").then(m=>m.PersonalInbox),{loading:()=> <p role="status">Loading notifications…</p>});
export function NotificationBell(){
 const user=useAuthenticatedUser();
 const allowed=user.permissions.includes("system:admin")||user.permissions.includes("communication.inbox.read");
 const [open,setOpen]=useState(false),[revision,setRevision]=useState(0);
 const source=useDashboardSource<{total:number}>(allowed?"/api/inbox?page=1&size=20&status=unread":null,revision);
 useEffect(()=>{const refresh=()=>setRevision(v=>v+1);const timer=window.setInterval(()=>{if(document.visibilityState==="visible")refresh()},30000);window.addEventListener("focus",refresh);return()=>{window.clearInterval(timer);window.removeEventListener("focus",refresh)}},[]);
 const unread=source.data?.total;
 return <><button type="button" className="go-icon-button go-notification-bell" aria-label={unread===undefined?"Open notifications":`Open notifications, ${unread} unread`} title="Notifications" aria-haspopup="dialog" onClick={()=>setOpen(true)}><BellIcon aria-hidden="true"/>{unread!==undefined&&unread>0&&<span className="go-notification-count" aria-hidden="true">{unread>99?"99+":unread}</span>}</button>
 {open&&<RecordDrawer title="Notifications" wide onClose={()=>{setOpen(false);setRevision(v=>v+1)}}>{allowed?<Inbox/>:<div className="go-dashboard-empty"><BellIcon aria-hidden="true"/><h3>Notification access is not enabled</h3><p>Your administrator can enable your personal inbox. This does not grant Finance approval access.</p></div>}</RecordDrawer>}
 </>;
}
