import "server-only";
import "@/server/runtime";
import { backendApiUrl } from "./backend-config";

// All application services run in process. Unmapped routes never use a network fallback.
export async function backendRequest(input: string | URL, init?: RequestInit): Promise<Response> {
  const url = new URL(input);
  if (url.origin === new URL(backendApiUrl).origin) {
    const path=url.pathname.replace(/^\/api\/v1/,"");
    if(process.env.EPCERP_PLATFORM_OWNER === "nextjs") {
      if(/^\/health\/(?:live|ready)$/.test(path))return (await import('@/server/platform/health')).health(path.endsWith('/ready'));
      if(/^\/auth\/organisation(?:\/|$)/.test(path))return (await import('@/server/platform/organisation')).handleOrganisation(new Request(url,init));
      if(/^\/auth\/user_profile(?:\/|$)/.test(path))return (await import('@/server/platform/profile')).handleProfile(new Request(url,init));
      if(/^\/administration\/director(?:\/options)?$/.test(path))return (await import('@/server/platform/governance')).handleGovernance(new Request(url,init));
      if(/^\/work-calendar\/holidays(?:\/[^/]+)?$/.test(path))return (await import('@/server/platform/calendar')).handleCalendar(new Request(url,init));
      if(/^\/documents(?:\/|$)/.test(path))return (await import('@/server/platform/storage')).handleStorage(new Request(url,init));
      if(/^\/settings\/company-profile(?:\/|$)/.test(path))return (await import('@/server/company/http')).handleCompany(new Request(url,init));
      const historical=await import('@/server/tenders/intake/historical');if(historical.ownsHistorical(path))return historical.handleHistorical(new Request(url,init));
    }
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
    if(process.env.EPCERP_TSC_OWNER === "nextjs" && /^\/tender-fms\/drafts\/[^/]+\/(?:tsc|flow-status|bidding-approval|prebid-queries|bid-items|vendor-quotations|bid-costing|award-project)(?:\/|$)/.test(path)) {
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
  return Response.json({detail:"Not found"},{status:404,headers:{"Cache-Control":"no-store"}});
}
