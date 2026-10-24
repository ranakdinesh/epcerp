"use client";
import {useState} from "react";
import Link from "next/link";
import dynamic from "next/dynamic";
import {ClipboardDocumentCheckIcon,ArrowUpRightIcon,ExclamationTriangleIcon} from "@heroicons/react/24/outline";
import {RecordDrawer} from "@/components/ui/record-drawer";
import type {PersonalTask} from "@/components/tasks/personal-tasks";
import {taskAttention} from "./dashboard-model";
const TaskWork=dynamic(()=>import("@/components/tasks/task-work").then(m=>m.TaskWork),{loading:()=> <p role="status">Loading assigned work…</p>});
export function ProcurementTaskPanel({rows,total,loading,error,enabled,onSaved}:{rows:PersonalTask[];total:number|null;loading:boolean;error:string;enabled:boolean;onSaved:()=>void}){
 const [selected,setSelected]=useState<PersonalTask|null>(null);
 const attention=taskAttention(rows);
 return <section className="go-dashboard-panel go-dashboard-tasks" aria-labelledby="task-attention-title"><div className="go-dashboard-section-heading"><div><h2 id="task-attention-title">Tasks needing attention</h2>{!!rows.length&&<p>Showing {rows.length} of {total} assignments · {attention.overdue} overdue in this view</p>}</div>{enabled&&<Link className="go-dashboard-text-link" href="/dashboard?screen=my-tasks">View all tasks <ArrowUpRightIcon aria-hidden="true"/></Link>}</div>
 {loading?<p className="go-dashboard-placeholder" role="status">Loading your assignments…</p>:error?<p role="alert" className="go-dashboard-inline-error">{error}</p>:!enabled?<div className="go-dashboard-empty"><ClipboardDocumentCheckIcon aria-hidden="true"/><h3>Personal task access is not enabled</h3><p>Your purchasing project assignment is available. Personal task access requires the existing My tasks permissions.</p></div>:!rows.length?<div className="go-dashboard-empty"><ClipboardDocumentCheckIcon aria-hidden="true"/><h3>No purchasing tasks assigned yet</h3><p>Your doer and issue-resolver assignments will appear here when recorded in Purchase FMS.</p></div>:<div className="go-dashboard-task-list">{attention.rows.map(task=><button type="button" className="go-dashboard-task" key={task.id} onClick={()=>setSelected(task)}><span className={`go-task-mark${task.is_overdue?" is-overdue":""}`}><ExclamationTriangleIcon aria-hidden="true"/></span><span className="go-task-description"><strong>{task.step_number}. {task.title}</strong><span>{task.project_name} · {task.relation==="resolver"?"Issue resolver":"Doer"} · {task.coordination_state.replaceAll("_"," ")}</span>{(task.block_reason||!task.available)&&<span className="go-task-warning">{task.block_reason||"Source blocked — open for details"}</span>}</span><span className={`go-task-due${task.is_overdue?" is-overdue":""}`}>{task.is_overdue&&<strong>Overdue</strong>}<span>{(task.relation==="resolver"?task.resolution_due:task.planned_date)||"No due date"}</span></span><ArrowUpRightIcon className="go-task-open" aria-hidden="true"/></button>)}</div>}
 {selected&&<RecordDrawer title={`${selected.project_name} · ${selected.title}`} wide extraWide onClose={()=>{setSelected(null);onSaved()}}><TaskWork task={selected}/></RecordDrawer>}
 </section>;
}
