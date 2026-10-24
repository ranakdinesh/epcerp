import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

import { AUTH_COOKIE_NAMES } from "@/lib/auth/session";

export function proxy(request: NextRequest) {
  const hasAccessToken = Boolean(request.cookies.get(AUTH_COOKIE_NAMES.accessToken)?.value);
  const { pathname, search } = request.nextUrl;

  if ((pathname.startsWith("/dashboard") || pathname.startsWith("/tenders")) && !hasAccessToken) {
    const loginUrl = new URL("/auth/renew", request.url);
    loginUrl.searchParams.set("next", `${pathname}${search}`);
    return NextResponse.redirect(loginUrl);
  }

  if (pathname === "/login" && !hasAccessToken && !request.nextUrl.searchParams.has("session")) {
    const renewUrl = new URL("/auth/renew", request.url);
    renewUrl.searchParams.set("next", request.nextUrl.searchParams.get("next") ?? "/dashboard");
    return NextResponse.redirect(renewUrl);
  }

  if (pathname === "/login" && hasAccessToken) {
    return NextResponse.redirect(new URL("/dashboard", request.url));
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/dashboard/:path*", "/tenders/:path*", "/login"],
};
