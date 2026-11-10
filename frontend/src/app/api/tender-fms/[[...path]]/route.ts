import { proxyAuthenticatedRequest } from "@/lib/auth/backend-proxy";
import { hasSameOrigin } from "@/lib/auth/request-origin";
type Context = { params: Promise<{ path?: string[] }> };
const legacy = new Set(["GET", "POST"]);
async function route(request: Request, context: Context) {
  const { path = [] } = await context.params;
  const joined = path.join("/");
  const upload = /^receipts\/[0-9a-f-]{36}\/files\/[0-9a-f-]{36}$/.test(joined);
  const tsc = /^drafts\/[0-9a-f-]{36}\/(?:flow-status|bidding-approval|prebid-queries(?:\/[0-9a-f-]{36})?|bid-items(?:\/[0-9a-f-]{36})?|vendor-quotations(?:\/(?:select-l1|[0-9a-f-]{36}))?|bid-costing|award-project|tsc(?:\/options|\/import(?:\/commit)?|\/[0-9a-f-]{36}(?:\/(?:document|uploaded-document|resume|evidence|history|eligibility(?:\/verify)?)|\/responses\/[0-9a-f-]{36}|\/answers\/[a-zA-Z0-9_.-]{1,80}(?:\/(?:research|originals))?)?)?)$/.test(joined);
  const draft = /^drafts(?:\/[0-9a-f-]{36}(?:\/batches|\/documents\/[0-9a-f-]{36}(?:\/(?:text|result|original|history|retry))?)?)?$/.test(joined);
  const named = /^records(?:\/[0-9a-f-]{36}(?:\/manager-assignments)?)?$/.test(joined);
  const receipt = /^receipts(?:\/[0-9a-f-]{36}(?:\/seal)?)?$/.test(joined);
  if (["POST", "PUT", "PATCH", "DELETE"].includes(request.method) && (named || receipt || upload || draft || tsc || joined.endsWith("/ownership"))) {
    if (!hasSameOrigin(request)) return Response.json({ detail: "Same-origin request required" }, { status: 403 });
  }
  const methods = upload ? new Set(["GET", "PUT", "HEAD"]) : named ? new Set(["GET", "POST", "PATCH"]) : tsc ? new Set(["GET", "POST", "PATCH", "DELETE"]) : receipt ? new Set(["GET", "POST"]) : draft ? new Set(["GET", "POST", "PATCH", "DELETE"]) : legacy;
  const wordImport = /^drafts\/[0-9a-f-]{36}\/tsc\/import(?:\/commit)?$/.test(joined);
  const limit = wordImport ? 6 * 1024 * 1024 : upload ? 64 * 1024 * 1024 : receipt || named || draft || tsc ? 32 * 1024 : joined === "upload" ? 32 * 1024 * 1024 + 64 * 1024 : joined.endsWith("/ownership") ? 8192 : 1024;
  return proxyAuthenticatedRequest(request, context, "tender-fms", methods, limit, upload && request.method === "PUT" ? "HEAD" : false);
}
export const GET = route;
export const POST = route;
export const PATCH = route;
export const PUT = route;
export const HEAD = route;

export const DELETE = route;
