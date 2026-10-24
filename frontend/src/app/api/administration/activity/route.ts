import { proxyAuthenticatedRequest } from "@/lib/auth/backend-proxy";

export function POST(request: Request) {
  return proxyAuthenticatedRequest(request, { params: Promise.resolve({}) }, "auth/activity", new Set(["POST"]));
}
