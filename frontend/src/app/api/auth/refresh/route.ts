import { NextResponse } from "next/server";
import { cookies } from "next/headers";

import { clearAuthCookies, setAuthCookies } from "@/lib/auth/cookies";
import { AUTH_COOKIE_NAMES } from "@/lib/auth/session";

import { backendApiUrl } from "@/lib/auth/backend-config";

export async function POST() {
  const refreshToken = (await cookies()).get(AUTH_COOKIE_NAMES.refreshToken)?.value;
  if (!refreshToken) {
    const response = NextResponse.json({ message: "No active session" }, { status: 401 });
    clearAuthCookies(response);
    return response;
  }

  try {
    const backendResponse = await fetch(`${backendApiUrl}/auth/refresh`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refresh_token: refreshToken }),
      cache: "no-store",
    });
    if (!backendResponse.ok && backendResponse.status !== 401 && backendResponse.status !== 403) {
      return NextResponse.json({ message: "Session refresh unavailable" }, { status: 503 });
    }
    if (!backendResponse.ok) {
      const response = NextResponse.json({ message: "Session expired" }, { status: 401 });
      clearAuthCookies(response);
      return response;
    }

    const response = NextResponse.json({ ok: true });
    setAuthCookies(response, await backendResponse.json());
    return response;
  } catch {
    return NextResponse.json({ message: "Session refresh unavailable" }, { status: 503 });
  }
}
