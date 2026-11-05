import "server-only";
import { backendApiUrl } from "./backend-config";

// Module ownership is explicit. Native services run in-process; unmigrated
// resources use Go. Native failures never fall back to another writer.
export async function backendRequest(input: string | URL, init?: RequestInit): Promise<Response> {
  const url = new URL(input);
  if (url.origin === new URL(backendApiUrl).origin) {
    const path=url.pathname.replace(/^\/api\/v1/,"");
    if(process.env.EPCERP_PROJECT_BUDGETS_OWNER === "nextjs") {
      const {handleBudget,ownsBudget}=await import("@/server/projects/budgets/http");
      if(ownsBudget(path))return handleBudget(new Request(url,init));
    }
    if(process.env.EPCERP_PURCHASING_OWNER === "nextjs") {
      const {handlePurchasing,ownsPurchasing}=await import("@/server/purchasing/http");
      if(ownsPurchasing(path))return handlePurchasing(new Request(url,init));
    }
    if(process.env.EPCERP_PROJECT_TEAM_OWNER === "nextjs") {
      const {handleTeam,ownsTeam}=await import("@/server/projects/team/http");
      if(ownsTeam(path))return handleTeam(new Request(url,init));
    }
    if(process.env.EPCERP_PROJECTS_OWNER === "nextjs") {
      const {handleProjects,ownsProjects}=await import("@/server/projects/http");
      if(ownsProjects(path))return handleProjects(new Request(url,init));
    }
    if(process.env.EPCERP_PERSONAL_TASKS_OWNER === "nextjs" && /^\/my-tasks(?:\/|$)/.test(path)) {
      const {handleTasks}=await import("@/server/personal-tasks/http");
      return handleTasks(new Request(url,init));
    }
    if(process.env.EPCERP_INBOX_OWNER === "nextjs" && /^\/inbox(?:\/|$)/.test(path)) {
      const {handleInbox}=await import("@/server/inbox/http");
      return handleInbox(new Request(url,init));
    }
    if(process.env.EPCERP_TSC_OWNER === "nextjs" && /^\/tender-fms\/drafts\/[^/]+\/tsc(?:\/|$)/.test(path)) {
      const {handleTSC}=await import("@/server/tenders/tsc/http");
      return handleTSC(new Request(url,init));
    }
    if(process.env.EPCERP_TENDER_REGISTER_OWNER === "nextjs") {
      const {handleRegister,ownsRegister}=await import("@/server/tenders/register/http");
      if(ownsRegister(path))return handleRegister(new Request(url,init));
    }
    if(process.env.EPCERP_TENDER_INTAKE_OWNER === "nextjs") {
      const {handleIntake,ownsIntake}=await import("@/server/tenders/intake/http");
      if(ownsIntake(path))return handleIntake(new Request(url,init));
    }
    if(process.env.EPCERP_AUTH_OWNER === "nextjs") {
      const { handleAuth, ownsAuthPath } = await import("@/server/auth/http");
      if (ownsAuthPath(path)) return handleAuth(new Request(url,init));
    }
    if((process.env.EPCERP_CLIENTS_OWNER === "nextjs" && /^\/clients(?:\/|$)/.test(path)) ||
       (process.env.EPCERP_REFERENCES_OWNER === "nextjs" && /^\/client-references(?:\/|$)/.test(path)) ||
       (process.env.EPCERP_CATALOGUE_OWNER === "nextjs" && /^\/(?:categories|manufacturers|configuration-templates|catalogue-items)(?:\/|$)/.test(path)) ||
       (process.env.EPCERP_VENDOR_VERIFICATION_OWNER === "nextjs" && /^\/suppliers(?:\/|$)/.test(path)) ||
       (process.env.EPCERP_VENDOR_DOCUMENTS_OWNER === "nextjs" && /^\/suppliers(?:\/|$)/.test(path)) ||
       (process.env.EPCERP_VENDOR_PROFILES_OWNER === "nextjs" && /^\/suppliers(?:\/|$)/.test(path))) {
      const {handleMaster,ownsMasterPath,ownsNativeSupplierPath}=await import("@/server/master/http");
      if(ownsMasterPath(path) && (!/^\/suppliers(?:\/|$)/.test(path) || ownsNativeSupplierPath(path))) return handleMaster(new Request(url,init));
    }
  }
  return fetch(input,init);
}
