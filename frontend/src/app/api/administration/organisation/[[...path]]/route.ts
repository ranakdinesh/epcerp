import { proxyAuthenticatedRequest } from "@/lib/auth/backend-proxy";
const methods = new Set(["GET", "POST", "PATCH"]);
const id = "[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}";
const get = new RegExp(`^(departments|employees|team|team/${id}/work|offboarding|offboarding/${id}(/candidates|/history)?)$`);
const post = new RegExp(`^(departments|offboarding|offboarding/${id}/(plan|approve|refresh|cancel|deactivate))$`);
const patch = new RegExp(`^(departments|employees)/${id}$`);
async function proxy(request: Request, context: { params: Promise<{ path?: string[] }> }) {
 const params = await context.params;
 const path = params.path ?? [];
 // Reject decoded path separators and traversal before constructing an upstream URL.
 if (path.some(segment => !segment || segment.includes("/") || segment.includes("\\") || segment === "." || segment === "..")) return Response.json({detail:"Not found"},{status:404});
 const route = path.join("/");
 const allowed = request.method === "GET" ? get.test(route) : request.method === "POST" ? post.test(route) : request.method === "PATCH" && patch.test(route);
 if (!allowed) return Response.json({detail:"Not found"},{status:404});
 return proxyAuthenticatedRequest(request, {params:Promise.resolve(params)}, "auth/organisation", methods, 16384);
}
export const GET = proxy;
export const POST = proxy;
export const PATCH = proxy;
