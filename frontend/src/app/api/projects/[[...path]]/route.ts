import { proxyAuthenticatedRequest } from "@/lib/auth/backend-proxy";

import { hasSameOrigin } from "@/lib/auth/request-origin";
const methods = new Set(["GET", "POST", "PATCH"]);
const largeDrafts = new Set(["purchase-bo", "purchase-indents", "budget"]);
const proxy = async (request: Request, context: { params: Promise<{ path?: string[] }> }) => {
  const params = await context.params;
  const projectMaster = (params.path?.length ?? 0)<=1 || params.path?.length===2&&['manager','schedule-preview','metadata-history','metadata-options','manager-history'].includes(params.path[1]);
  const projectBudget = params.path?.length === 2 && params.path[1] === 'budget';
  const projectTeam = params.path?.[1] === 'responsibilities';
  if ((projectMaster || projectTeam || projectBudget) && ['POST','PATCH'].includes(request.method) && !hasSameOrigin(request)) return Response.json({detail:"Same-origin request required"},{status:403});
  const quotationUpload = request.method === "POST" && params.path?.length === 6 && params.path[1] === "purchase-bo" && params.path[3] === "group-quotations" && params.path[5] === "attachments";
  const technicalUpload = request.method === "POST" && params.path?.length === 7 && params.path[1] === "purchase-workflow" && params.path[3] === "technical" && params.path[4] === "uploads";
  const orderUpload=request.method==="POST"&&params.path?.length===6&&params.path[1]==="purchase-workflow"&&params.path[3]==="purchase-orders"&&params.path[4]==="uploads";
  const quotationReview = request.method === "POST" && params.path?.length === 5 && params.path[1] === "purchase-bo" && params.path[3] === "quotation-approvals" && (params.path[4] === "review" || params.path[4] === "print");
  const limit = quotationReview ? 1024 * 1024 : (quotationUpload || technicalUpload || orderUpload) ? 11 * 1024 * 1024 : params.path?.[3] === "indent-groups" ? 1024 * 1024 : largeDrafts.has(params.path?.[1] ?? "") ? 512 * 1024 : 64 * 1024;
  return proxyAuthenticatedRequest(request, { params: Promise.resolve(params) }, "projects", methods, limit, technicalUpload || orderUpload);
};
export const GET = proxy;
export const POST = proxy;
export const PATCH = proxy;
