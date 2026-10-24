"use client";
import { useSearchParams } from "next/navigation";
import { useAuthenticatedUser } from "@/components/auth-user-provider";
import { PurchaseDashboard } from "./purchase-dashboard";
import {PurchasePackageDetail} from "./work-packages";
import { PurchaseWorkspace } from "./purchase-workspace";
import { ProcessMap } from "@/components/processes/process-map";
import { purchaseProcess } from "@/components/processes/process-directory";
export function PurchaseFMS({allowedScreens}:{allowedScreens:readonly string[]}) {
 const params=useSearchParams(),user=useAuthenticatedUser();
 const can= (code:string)=>user.permissions.includes("system:admin")||user.permissions.includes(code);
 if (!can("projects.purchase_workflow.view")||!can("projects.purchase_workflow.read")) return <p role="alert">Purchase FMS view and read access are required.</p>;
 const project=params.get("project");
 const taskPackage=params.get("package");
 if(project&&taskPackage&&params.get("work")==="task")return <PurchasePackageDetail project={project} id={taskPackage}/>;
 return project ? <PurchaseWorkspace key={project} project={project} map={params.get("view")==="map"} allowedScreens={allowedScreens}/> : params.get("view")==="map" ? <ProcessMap process={purchaseProcess} allowedScreens={allowedScreens}/> : <PurchaseDashboard/>;
}
