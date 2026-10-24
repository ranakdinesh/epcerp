import { cookies } from "next/headers";

export const AUTH_COOKIE_NAMES = {
  accessToken: "epcerp_access_token",
  refreshToken: "epcerp_refresh_token",
} as const;

import { backendApiUrl } from "@/lib/auth/backend-config";

export type AuthenticatedUser = {
  id: string;
  username: string;
  first_name?: string;
  last_name?: string;
  mobile?: string;
  is_active?: boolean;
  is_superuser?: boolean;
  email: string;
  permissions: string[];
  must_change_password?: boolean;
};

export async function getAuthenticatedUser(): Promise<AuthenticatedUser | null> {
  const accessToken = (await cookies()).get(AUTH_COOKIE_NAMES.accessToken)?.value;
  if (!accessToken) {
    return null;
  }

  try {
    const response = await fetch(`${backendApiUrl}/auth/me`, {
      headers: { Authorization: `Bearer ${accessToken}` },
      cache: "no-store",
    });
    if (!response.ok) {
      return null;
    }
    return (await response.json()) as AuthenticatedUser;
  } catch {
    // Do not expose internal API/network details to an unauthenticated visitor.
    return null;
  }
}
