"use client";
import {QuotationWorkTasks,type QuotationTask} from "@/components/tasks/quotation-work-tasks";
import {useCallback,useEffect,useState} from "react";
import Link from "next/link";
import {useRouter} from "next/navigation";
import {ArrowPathIcon,FolderIcon,ClipboardDocumentCheckIcon,ClockIcon,ChartBarIcon,ArrowUpRightIcon} from "@heroicons/react/24/outline";
import {useAuthenticatedUser} from "@/components/auth-user-provider";
import {usePersonalProfile} from "@/components/personal-profile";
import {DataGrid,GridAction,GridStatus,type GridQuery} from "@/components/ui/data-grid";
import {purchaseProjectHref,type Dashboard,type PurchaseProject} from "@/components/purchase-fms/data";
import type {PersonalTask} from "@/components/tasks/personal-tasks";
import {ProcurementCharts} from "./procurement-charts";
import {ProcurementTaskPanel} from "./procurement-task-panel";
import {useDashboardSource} from "./use-dashboard-source";
import {greetingAt,projectStatus,taskAttention} from "./dashboard-model";
import "./procurement-dashboard.css";
type TaskPage={quotation_tasks?:QuotationTask[];rows:PersonalTask[];total:number};
function RecordedProgress({value}:{value:number|null}){
 if(value===null||!Number.isFinite(value)||value<0||value>100)return <span className="go-progress-unavailable">Not recorded</span>;
 return <div className="go-project-progress"><div role="progressbar" aria-label="Recorded purchase progress" aria-valuemin={0} aria-valuemax={100} aria-valuenow={value}><span style={{width:`${value}%`}}/></div><strong>{value}%</strong></div>;
}
export function ProcurementDashboard({purchaseEntry=false}:{purchaseEntry?:boolean}){
 const user=useAuthenticatedUser(),profile=usePersonalProfile(user.id),router=useRouter();
 const can=(code:string)=>user.permissions.includes("system:admin")||user.permissions.includes(code);
 const taskAccess=can("work.tasks.view")&&can("work.tasks.read");
 const [revision,setRevision]=useState(0),[now,setNow]=useState<Date|null>(null);
 const [query,setQuery]=useState({page:1,size:20,search:"",sort:"number",direction:"asc"});
 useEffect(()=>{const timer=setTimeout(()=>setNow(new Date()),0);return()=>clearTimeout(timer)},[revision]);
 const projectPath=`/api/purchase-fms?page=${query.page}&page_size=${query.size}&search=${encodeURIComponent(query.search)}&sort=${query.sort}&direction=${query.direction}`;
 const projects=useDashboardSource<Dashboard>(projectPath,revision);
 const scope=useDashboardSource<Dashboard>("/api/purchase-fms?page=1&page_size=1&sort=number&direction=asc",revision);
 const tasks=useDashboardSource<TaskPage>(taskAccess?"/api/my-tasks?page=1&page_size=20&role=all&state=all":null,revision);
 const changed=useCallback((q:GridQuery)=>setQuery(old=>old.page===q.page&&old.size===q.size&&old.search===q.search&&old.sort===q.sort&&old.direction===q.direction?old:{page:q.page,size:q.size,search:q.search,sort:q.sort,direction:q.direction}),[]);
 const projectRows=projects.data?.items??[],taskRows=tasks.data?.rows??[];
 const active=projectRows.filter(r=>projectStatus(r)==="active").length;
 const attention=taskAttention(taskRows);
 const refreshing=projects.loading||scope.loading||tasks.loading;
 const stats=[
  {label:scope.data?.portfolio?"Purchasing projects":"Assigned projects",value:scope.data?.total,note:scope.error?"Project count unavailable":scope.data?.portfolio?"Authorised portfolio scope":"Purchase ownership & PM oversight",icon:FolderIcon,tone:"blue"},
  {label:"Active projects in view",value:projects.data?active:undefined,note:projects.error?"Project status unavailable":`${projectRows.length} ${projectRows.length===1?"project":"projects"} on this page`,icon:ChartBarIcon,tone:"teal"},
  {label:"Assigned tasks in view",value:tasks.data?taskRows.length+(tasks.data.quotation_tasks?.length??0):undefined,note:tasks.error?"Task count unavailable":!taskAccess?"Personal task access not enabled":"Workflow assignments & quotation work",icon:ClipboardDocumentCheckIcon,tone:"blue"},
  {label:"Overdue tasks in view",value:tasks.data?attention.overdue:undefined,note:tasks.error?"Due dates unavailable":!taskAccess?"Personal task access not enabled":taskRows.length?`Within ${taskRows.length} displayed assignments`:"No overdue assignments in this view",icon:ClockIcon,tone:attention.overdue?"amber":"muted"},
 ];
 useEffect(()=>{const refresh=()=>setRevision(v=>v+1);window.addEventListener("purchase-progress-saved",refresh);window.addEventListener("focus",refresh);return()=>{window.removeEventListener("purchase-progress-saved",refresh);window.removeEventListener("focus",refresh)}},[]);
 function refresh(){setRevision(v=>v+1)}
 return <div className="go-procurement-dashboard"><header className="go-dashboard-heading"><div><h1>{purchaseEntry?"Purchase FMS":`${now?greetingAt(now):"Welcome"}, ${profile?.display_name||user.username}`}</h1><p>{purchaseEntry?`${now?greetingAt(now):"Welcome"}, ${profile?.display_name||user.username}. `:""}Your purchasing work, at a glance.</p></div><div className="go-dashboard-heading-actions">{now&&<time dateTime={now.toISOString()}>{new Intl.DateTimeFormat("en-IN",{timeZone:"Asia/Kolkata",weekday:"short",day:"numeric",month:"short",year:"numeric"}).format(now)}</time>}<button type="button" className="go-dashboard-refresh" onClick={refresh} disabled={refreshing}><ArrowPathIcon aria-hidden="true" className={refreshing?"go-refreshing":""}/>{refreshing?"Refreshing…":"Refresh"}</button></div></header>
 <div className="go-dashboard-metrics" aria-label="Purchasing summary">{stats.map(s=><section className={`go-dashboard-metric go-metric-${s.tone}`} key={s.label}><s.icon aria-hidden="true"/><div><h2>{s.label}</h2><strong>{s.value===undefined?"—":s.value.toLocaleString("en-IN")}</strong><p>{s.note}</p></div></section>)}</div>
 {scope.error&&<p role="alert" className="go-dashboard-inline-error">{scope.error} Use Refresh to retry.</p>}
 <ProcurementCharts rows={projectRows} total={projects.data?.total??null} loading={projects.loading} error={projects.error}/>
 <section className="go-dashboard-panel go-dashboard-projects" aria-labelledby="assigned-projects-title"><div className="go-dashboard-section-heading"><div><h2 id="assigned-projects-title">{scope.data?.portfolio?"Purchasing projects":"Assigned projects"}</h2><p>Open a project to continue its purchasing workflow.</p></div><Link className="go-dashboard-text-link" href={purchaseEntry?"/dashboard?screen=purchase-fms&view=map":"/dashboard?screen=purchase-fms"}>{purchaseEntry?"Reference process map":"Purchase FMS"} <ArrowUpRightIcon aria-hidden="true"/></Link></div>
 {projects.error&&<p role="alert" className="go-dashboard-inline-error">{projects.error} <button type="button" onClick={refresh}>Retry</button></p>}
 <DataGrid<PurchaseProject> label={scope.data?.portfolio?"Purchasing projects":"Assigned purchasing projects"} rows={projectRows} loading={projects.loading} remote={{total:projects.data?.total??0,onChange:changed}} initialSort="number" columns={[
 {key:"number",label:"Project",value:r=>r.project_number,render:r=><Link className="go-dashboard-text-link" href={purchaseProjectHref(r.id,r.next_step)}>{r.project_number}</Link>},
 {key:"name",label:"Project name",value:r=>r.name},
 {key:"state",label:"Status",value:projectStatus,render:r=><GridStatus value={projectStatus(r).replaceAll("_"," ")}/>},
 {key:"approval",label:"Quotation approvals",sortable:false,value:r=>r.pending_approval_groups||r.returned_groups||r.approved_groups||0,render:r=><span>{r.returned_groups?`${r.returned_groups} returned · `:""}{r.pending_approval_groups?`${r.pending_approval_groups} waiting for approval · `:""}{r.approved_groups?`${r.approved_groups} approved`:!r.pending_approval_groups&&!r.returned_groups?"Not submitted":""}</span>},
 {key:"progress",label:"Purchase progress",sortable:false,value:r=>r.progress_percent??"Not recorded",render:r=><RecordedProgress value={r.progress_percent}/>},
 ]} actions={r=><GridAction label={`Open purchasing for ${r.name}`} onClick={()=>router.push(purchaseProjectHref(r.id,r.next_step))}/>}/>
 {!projects.loading&&!projects.error&&!projectRows.length&&<p className="go-chart-note">{query.search?"No projects match your search.":"No purchasing projects are assigned to your account yet."}</p>}
 </section>
 <QuotationWorkTasks rows={tasks.data?.quotation_tasks??[]}/>
 <ProcurementTaskPanel rows={taskRows} total={tasks.data?.total??null} loading={tasks.loading} error={tasks.error} enabled={taskAccess} onSaved={refresh}/>
 <p className="go-dashboard-updated">{projects.loadedAt?`Projects loaded ${new Intl.DateTimeFormat("en-IN",{timeZone:"Asia/Kolkata",hour:"numeric",minute:"2-digit"}).format(projects.loadedAt)} IST`:"Project records unavailable"} · Charts follow the project view above. Task state tracks coordination, not specialist completion.</p>
 </div>;
}
