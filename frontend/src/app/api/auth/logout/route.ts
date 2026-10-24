import { NextResponse } from "next/server";
import { cookies } from "next/headers";

import { clearAuthCookies } from "@/lib/auth/cookies";
import { AUTH_COOKIE_NAMES } from "@/lib/auth/session";

import { backendApiUrl } from "@/lib/auth/backend-config";

export async function POST() {
  const refreshToken = (await cookies()).get(AUTH_COOKIE_NAMES.refreshToken)?.value;

  if (refreshToken) {
    try {
      await fetch(`${backendApiUrl}/auth/logout`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ refresh_token: refreshToken }),
        cache: "no-store",
      });
    } catch {
      // Always clear local cookies even if the API is momentarily unavailable.
    }
  }

  const response = NextResponse.json({ ok: true });
  clearAuthCookies(response);
  return response;
}
