import { proxyAuthenticatedRequest } from "@/lib/auth/backend-proxy";
import { clearAuthCookies } from "@/lib/auth/cookies";
export async function POST(request: Request) {
 const response = await proxyAuthenticatedRequest(request, { params: Promise.resolve({}) }, "auth/me/password", new Set(["POST"]), 8192);
 if (response.ok) clearAuthCookies(response);
 return response;
}
