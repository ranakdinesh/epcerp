import { NextResponse } from "next/server";

import { setAuthCookies } from "@/lib/auth/cookies";

import { backendApiUrl } from "@/lib/auth/backend-config";

export async function POST(request: Request) {
  let payload: { identifier?: unknown; password?: unknown };
  try {
    payload = await request.json();
  } catch {
    return NextResponse.json({ message: "Invalid login request" }, { status: 400 });
  }

  if (
    typeof payload.identifier !== "string" ||
    typeof payload.password !== "string" ||
    payload.identifier.trim().length < 3 ||
    payload.password.length < 8
  ) {
    return NextResponse.json({ message: "Invalid login request" }, { status: 400 });
  }

  try {
    const backendResponse = await fetch(`${backendApiUrl}/auth/login`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        identifier: payload.identifier.trim(),
        password: payload.password,
      }),
      cache: "no-store",
    });

    if (!backendResponse.ok) {
      return NextResponse.json(
        { message: "Invalid email or username, or password" },
        { status: backendResponse.status === 429 ? 429 : 401 },
      );
    }

    const tokens = await backendResponse.json();
    const response = NextResponse.json({ ok: true, must_change_password: tokens.must_change_password === true });
    setAuthCookies(response, tokens);
    return response;
  } catch {
    return NextResponse.json(
      { message: "Sign-in is temporarily unavailable. Please try again." },
      { status: 503 },
    );
  }
}
