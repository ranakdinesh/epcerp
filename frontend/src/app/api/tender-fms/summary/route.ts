import { cookies } from "next/headers";
import { AUTH_COOKIE_NAMES } from "@/lib/auth/session";
import { AppError } from "@/server/errors";
import { tenderSummary } from "@/server/tenders/summary";

const headers = { "Cache-Control": "private, no-store" };

export async function GET() {
  const token = (await cookies()).get(AUTH_COOKIE_NAMES.accessToken)?.value;
  if (!token) return Response.json({ detail: "Sign in to view tender performance." }, { status: 401, headers });
  try {
    return Response.json(await tenderSummary(token), { headers });
  } catch (error) {
    const status = error instanceof AppError ? error.status : 503;
    return Response.json({ detail: "Tender performance could not be loaded." }, { status, headers });
  }
}
