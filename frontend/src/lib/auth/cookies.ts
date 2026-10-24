import { NextResponse } from "next/server";

import { AUTH_COOKIE_NAMES } from "@/lib/auth/session";
import { authCookieOptions } from "@/lib/auth/cookie-policy";

type TokenPair = {
  access_token: string;
  refresh_token: string;
  expires_in: number;
  refresh_expires_in: number;
};

const cookieOptions = authCookieOptions({
  NODE_ENV: process.env.NODE_ENV,
  EPCERP_ENVIRONMENT: process.env.EPCERP_ENVIRONMENT,
  EPCERP_DEVELOPMENT_HTTP: process.env.EPCERP_DEVELOPMENT_HTTP,
});

export function setAuthCookies(response: NextResponse, tokens: TokenPair): void {
  response.cookies.set(AUTH_COOKIE_NAMES.accessToken, tokens.access_token, {
    ...cookieOptions,
    maxAge: tokens.expires_in,
    path: "/",
  });
  response.cookies.set(AUTH_COOKIE_NAMES.refreshToken, tokens.refresh_token, {
    ...cookieOptions,
    maxAge: tokens.refresh_expires_in,
    path: "/api/auth",
  });
}

export function clearAuthCookies(response: NextResponse): void {
  response.cookies.set(AUTH_COOKIE_NAMES.accessToken, "", {
    ...cookieOptions,
    maxAge: 0,
    path: "/",
  });
  response.cookies.set(AUTH_COOKIE_NAMES.refreshToken, "", {
    ...cookieOptions,
    maxAge: 0,
    path: "/api/auth",
  });
}
